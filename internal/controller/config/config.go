/*
Copyright 2020 The Crossplane Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ktype "sigs.k8s.io/kustomize/api/types"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/crossplane-contrib/provider-cloudinit/apis/config/v1alpha1"
	"github.com/crossplane-contrib/provider-cloudinit/internal/cloudinit"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane-contrib/provider-cloudinit/apis/config/v1alpha1"
	cloudinitv1alpha1 "github.com/crossplane-contrib/provider-cloudinit/apis/v1alpha1"
	cloudinitClient "github.com/crossplane-contrib/provider-cloudinit/internal/clients/cloudinit"
)

const (
	errFailedToGetSecret    = "failed to get secret from namespace \"%s\""
	errSecretDataIsNil      = "secret data is nil"
	errFailedToGetConfigMap = "failed to get configmap from namespace \"%s\""
	errConfigMapDataIsNil   = "configmap data is nil"

	errSourceNotSetForValueFrom        = "source not set for value from"
	errFailedToGetDataFromSecretRef    = "failed to get data from secret ref"
	errFailedToGetDataFromConfigMapRef = "failed to get data from configmap ref"
	errMissingKeyForValuesFrom         = "missing key \"%s\" in values from source"

	errConfigInfoNilInObservedRelease = "config info is nil in observed cloudinit release"
	errChartNilInObservedConfig       = "chart field is nil in observed cloudinit config"
	errChartMetaNilInObservedConfig   = "chart metadata field is nil in observed cloudinit config"
	errObjectNotPartOfConfig          = "object is not part of config: %v"

	maxConcurrency = 10

	resyncPeriod     = 10 * time.Minute
	reconcileTimeout = 10 * time.Minute

	cloudinitConfigNameAnnotation      = "meta.helm.sh/config-name"
	cloudinitConfigNamespaceAnnotation = "meta.helm.sh/config-namespace"

	errNotConfig                         = "managed resource is not a Release custom resource"
	errProviderConfigNotSet              = "provider config is not set"
	errProviderNotRetrieved              = "provider could not be retrieved"
	errCredSecretNotSet                  = "provider credentials secret is not set"
	errNewKubernetesClient               = "cannot create new Kubernetes client"
	errProviderSecretNotRetrieved        = "secret referred in provider could not be retrieved"
	errProviderSecretValueForKeyNotFound = "value for key \"%s\" not found in provider credentials secret"
	errFailedToGetLastConfig             = "failed to get last cloudinit config"
	errLastConfigIsNil                   = "last cloudinit config is nil"
	errFailedToCheckIfUpToDate           = "failed to check if config is up to date"
	errFailedToInstall                   = "failed to install config"
	errFailedToUpgrade                   = "failed to upgrade config"
	errFailedToUninstall                 = "failed to uninstall config"
	errFailedToGetRepoCreds              = "failed to get user name and password from secret reference"
	errFailedToComposeValues             = "failed to compose values"
	errFailedToCreateRestConfig          = "cannot create new rest config using provider secret"
	errFailedToTrackUsage                = "cannot track provider config usage"
	errFailedToLoadPatches               = "failed to load patches"
	errFailedToUpdatePatchSha            = "failed to update patch sha"
	errFailedToSetName                   = "failed to update chart spec with the name from URL"
	errFailedToSetVersion                = "failed to update chart spec with the latest version"

	errFmtUnsupportedCredSource = "unsupported credentials source %q"
)

// Setup adds a controller that reconciles Config managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.ConfigGroupKind)
	logger := l.WithValues("controller", name)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ConfigGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			logger: logger,
			client: mgr.GetClient(),
			// usage
			newCloudInitClientFn: cloudinitClient.NewClient,
		}),
		managed.WithLogger(logger),
		managed.WithTimeout(reconcileTimeout),
		managed.WithLongWait(resyncPeriod),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Config{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrency}).
		Complete(r)
}

type connector struct {
	logger               logging.Logger
	client               client.Client
	usage                resource.Tracker
	newCloudInitClientFn func(useGzipCompression bool, useBase64Encoding bool, base64Boundary string) *cloudinitClient.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return nil, errors.New(errNotConfig)
	}
	l := c.logger.WithValues("request", cr.Name)

	l.Debug("Connecting")

	p := &cloudinitv1alpha1.ProviderConfig{}

	if cr.GetProviderConfigReference() == nil {
		return nil, errors.New(errProviderConfigNotSet)
	}

	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errFailedToTrackUsage)
	}

	n := types.NamespacedName{Name: cr.GetProviderConfigReference().Name}
	if err := c.client.Get(ctx, n, p); err != nil {
		return nil, errors.Wrap(err, errProviderNotRetrieved)
	}

	var err error

	useGzipCompression := true
	useBase64Encoding := true
	base64Boundary := "MIMEBOUNDARY"

	h := c.newCloudInitClientFn(useGzipCompression, useBase64Encoding, base64Boundary)
	if err != nil {
		return nil, errors.Wrap(err, errNewKubernetesClient)
	}

	return &cloudinitExternal{
		logger:    l,
		localKube: c.client,
		//kube:      k,
		cloudinit: h,
		// patch:     newPatcher(),
	}, nil
}

type cloudinitExternal struct {
	logger    logging.Logger
	localKube client.Client
	kube      client.Client
	cloudinit *cloudinitClient.Client
	//patch     Patcher
}

func (e *cloudinitExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotConfig)
	}

	e.logger.Debug("Observing")

	rel, err := e.cloudinit.GetLastConfig(meta.GetExternalName(cr))
	if err == driver.ErrConfigNotFound {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errFailedToGetLastConfig)
	}

	if rel == nil {
		return managed.ExternalObservation{}, errors.New(errLastConfigIsNil)
	}

	cr.Status.AtProvider = generateObservation(rel)

	// Determining whether the config is up to date may involve reading values
	// from secrets, configmaps, etc. This will fail if said dependencies have
	// been deleted. We don't need to determine whether we're up to date in
	// order to delete the config, so if we know we're about to be deleted we
	// return early to avoid blocking unnecessarily on missing dependencies.
	if meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: true}, nil
	}

	s, err := isUpToDate(ctx, e.localKube, &cr.Spec.ForProvider, rel, cr.Status)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errFailedToCheckIfUpToDate)
	}
	cr.Status.Synced = s
	cd := managed.ConnectionDetails{}
	if cr.Status.AtProvider.State == config.StatusDeployed && s {
		cr.Status.Failed = 0

		cd, err = connectionDetails(ctx, e.kube, cr.Spec.ConnectionDetails, rel.Name, rel.Namespace)
		if err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, "cannot get connection details")
		}
		cr.Status.SetConditions(xpv1.Available())
	} else {
		cr.Status.SetConditions(xpv1.Unavailable())
	}

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  cr.Status.Synced,
		ConnectionDetails: cd,
	}, nil
}

type deployAction func(config string, chart *chart.Chart, vals map[string]interface{}, patches []ktype.Patch) (*v1alpha1.Config, error)

func (e *cloudinitExternal) deploy(ctx context.Context, cr *v1alpha1.Config, action deployAction) error {
	cv, err := composeValuesFromSpec(ctx, e.localKube, cr.Spec.ForProvider.ValuesSpec)
	if err != nil {
		return errors.Wrap(err, errFailedToComposeValues)
	}

	creds, err := repoCredsFromSecret(ctx, e.localKube, cr.Spec.ForProvider.Chart.PullSecretRef)
	if err != nil {
		return errors.Wrap(err, errFailedToGetRepoCreds)
	}

	p, err := e.patch.getFromSpec(ctx, e.localKube, cr.Spec.ForProvider.PatchesFrom)
	if err != nil {
		return errors.Wrap(err, errFailedToLoadPatches)
	}

	chart, err := e.cloudinit.PullAndLoadChart(&cr.Spec.ForProvider.Chart, creds)
	if err != nil {
		return err
	}
	if cr.Spec.ForProvider.Chart.Name == "" {
		cr.Spec.ForProvider.Chart.Name = chart.Metadata.Name
		if err := e.localKube.Update(ctx, cr); err != nil {
			return errors.Wrap(err, errFailedToSetName)
		}
	}
	if cr.Spec.ForProvider.Chart.Version == "" {
		cr.Spec.ForProvider.Chart.Version = chart.Metadata.Version
		if err := e.localKube.Update(ctx, cr); err != nil {
			return errors.Wrap(err, errFailedToSetVersion)
		}
	}

	rel, err := action(meta.GetExternalName(cr), chart, cv, p)

	if err != nil {
		return err
	}

	if rel == nil {
		return errors.New(errLastConfigIsNil)
	}

	sha, err := e.patch.shaOf(p)
	if err != nil {
		return errors.Wrap(err, errFailedToUpdatePatchSha)
	}
	cr.Status.PatchesSha = sha
	cr.Status.AtProvider = generateObservation(rel)

	return nil
}

func (e *cloudinitExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotConfig)
	}

	e.logger.Debug("Creating")
	return managed.ExternalCreation{}, errors.Wrap(e.deploy(ctx, cr, e.cloudinit.Install), errFailedToInstall)
}

func (e *cloudinitExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotConfig)
	}

	e.logger.Debug("Updating")
	return managed.ExternalUpdate{}, errors.Wrap(e.deploy(ctx, cr, e.cloudinit.Upgrade), errFailedToUpgrade)
}

func (e *cloudinitExternal) Delete(_ context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return errors.New(errNotConfig)
	}

	e.logger.Debug("Deleting")

	return errors.Wrap(e.cloudinit.Uninstall(meta.GetExternalName(cr)), errFailedToUninstall)
}

// generateObservation generates config observation for the input release object
func generateObservation(in *config.Config) v1alpha1.ConfigObservation {
	o := v1alpha1.ConfigObservation{}

	relInfo := in.Info
	if relInfo != nil {
		o.State = relInfo.Status
		o.ConfigDescription = relInfo.Description
		o.Revision = in.Version
	}
	return o
}

// isUpToDate checks whether desired spec up to date with the observed state for a given config
func isUpToDate(ctx context.Context, kube client.Client, in *v1alpha1.ConfigParameters, observed *config.ConfigMap, s v1alpha1.ConfigStatus) (bool, error) {
	if observed.Info == nil {
		return false, errors.New(errConfigInfoNilInObservedRelease)
	}

	if isPending(observed.Info.Status) {
		return false, nil
	}

	oc := observed.Chart
	if oc == nil {
		return false, errors.New(errChartNilInObservedConfig)
	}

	ocm := oc.Metadata
	if ocm == nil {
		return false, errors.New(errChartMetaNilInObservedConfig)
	}
	if in.Chart.Name != ocm.Name {
		return false, nil
	}
	if in.Chart.Version != ocm.Version {
		return false, nil
	}
	desiredConfig, err := composeValuesFromSpec(ctx, kube, in.ValuesSpec)
	if err != nil {
		return false, errors.Wrap(err, errFailedToComposeValues)
	}

	d, err := yaml.Marshal(desiredConfig)
	if err != nil {
		return false, err
	}

	observedConfig := observed.Config
	if observedConfig == nil {
		// If no config provider, desiredConfig returns as empty map. However, observed would be nil in this case.
		// We know both empty and nil are same.
		observedConfig = make(map[string]interface{})
	}

	o, err := yaml.Marshal(observedConfig)
	if err != nil {
		return false, err
	}

	if string(d) != string(o) {
		return false, nil
	}

	changed, err := newPatcher().hasUpdates(ctx, kube, in.PatchesFrom, s)
	if err != nil {
		return false, errors.Wrap(err, errFailedToLoadPatches)
	}

	if changed {
		return false, nil
	}

	return true, nil
}

func isPending(s config.Status) bool {
	return s == config.StatusPendingInstall || s == cloudinit.StatusPendingUpgrade || s == cloudinit.StatusPendingRollback
}

func connectionDetails(ctx context.Context, kube client.Client, connDetails []v1alpha1.ConnectionDetail, relName, relNamespace string) (managed.ConnectionDetails, error) {
	mcd := managed.ConnectionDetails{}

	for _, cd := range connDetails {
		ro := unstructuredFromObjectRef(cd.ObjectReference)
		if err := kube.Get(ctx, types.NamespacedName{Name: ro.GetName(), Namespace: ro.GetNamespace()}, &ro); err != nil {
			return mcd, errors.Wrap(err, "cannot get object")
		}

		// TODO(hasan): consider making this check configurable, i.e. possible to skip via a field in spec
		if !partOfConfig(ro, relName, relNamespace) {
			return mcd, errors.Errorf(errObjectNotPartOfConfig, cd.ObjectReference)
		}

		paved := fieldpath.Pave(ro.Object)
		v, err := paved.GetValue(cd.FieldPath)
		if err != nil {
			return mcd, errors.Wrapf(err, "failed to get value at fieldPath: %s", cd.FieldPath)
		}
		s := fmt.Sprintf("%v", v)
		fv := []byte(s)
		// prevent secret data being encoded twice
		if cd.Kind == "Secret" && cd.APIVersion == "v1" && strings.HasPrefix(cd.FieldPath, "data") {
			fv, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				return mcd, errors.Wrap(err, "failed to decode secret data")
			}
		}

		mcd[cd.ToConnectionSecretKey] = fv
	}

	return mcd, nil
}

func unstructuredFromObjectRef(r corev1.ObjectReference) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetAPIVersion(r.APIVersion)
	u.SetKind(r.Kind)
	u.SetName(r.Name)
	u.SetNamespace(r.Namespace)

	return u
}

func partOfConfig(u unstructured.Unstructured, relName, relNamespace string) bool {
	a := u.GetAnnotations()
	return a[cloudinitConfigNameAnnotation] == relName && a[helmReleaseNamespaceAnnotation] == relNamespace
}

func getSecretData(ctx context.Context, kube client.Client, nn types.NamespacedName) (map[string][]byte, error) {
	s := &corev1.Secret{}
	if err := kube.Get(ctx, nn, s); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFailedToGetSecret, nn.Namespace))
	}
	if s.Data == nil {
		return nil, errors.New(errSecretDataIsNil)
	}
	return s.Data, nil
}

func getConfigMapData(ctx context.Context, kube client.Client, nn types.NamespacedName) (map[string]string, error) {
	cm := &corev1.ConfigMap{}
	if err := kube.Get(ctx, nn, cm); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFailedToGetConfigMap, nn.Namespace))
	}
	if cm.Data == nil {
		return nil, errors.New(errConfigMapDataIsNil)
	}
	return cm.Data, nil
}

func getDataValueFromSource(ctx context.Context, kube client.Client, source v1alpha1.ValueFromSource, defaultKey string) (string, error) { // nolint:gocyclo
	if source.SecretKeyRef != nil {
		r := source.SecretKeyRef
		d, err := getSecretData(ctx, kube, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
		if kerrors.IsNotFound(errors.Cause(err)) && !r.Optional {
			return "", errors.Wrap(err, errFailedToGetDataFromSecretRef)
		}
		if err != nil && !kerrors.IsNotFound(errors.Cause(err)) {
			return "", errors.Wrap(err, errFailedToGetDataFromSecretRef)
		}
		k := defaultKey
		if r.Key != "" {
			k = r.Key
		}
		valBytes, ok := d[k]
		if !ok && !r.Optional {
			return "", errors.New(fmt.Sprintf(errMissingKeyForValuesFrom, k))
		}
		return string(valBytes), nil
	}
	if source.ConfigMapKeyRef != nil {
		r := source.ConfigMapKeyRef
		d, err := getConfigMapData(ctx, kube, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
		if kerrors.IsNotFound(errors.Cause(err)) && !r.Optional {
			return "", errors.Wrap(err, errFailedToGetDataFromConfigMapRef)
		}
		if err != nil && !kerrors.IsNotFound(errors.Cause(err)) {
			return "", errors.Wrap(err, errFailedToGetDataFromConfigMapRef)
		}
		k := defaultKey
		if r.Key != "" {
			k = r.Key
		}
		valString, ok := d[k]
		if !ok && !r.Optional {
			return "", errors.New(fmt.Sprintf(errMissingKeyForValuesFrom, k))
		}
		return valString, nil
	}
	return "", errors.New(errSourceNotSetForValueFrom)
}
