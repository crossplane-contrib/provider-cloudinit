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
	"time"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ktype "sigs.k8s.io/kustomize/api/types"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane-contrib/provider-cloudinit/apis/config/v1alpha1"
	cloudinitv1alpha1 "github.com/crossplane-contrib/provider-cloudinit/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-cloudinit/pkg/clients"
	cloudinitClient "github.com/crossplane-contrib/provider-cloudinit/pkg/clients/helm"
)

const (
	maxConcurrency = 10

	resyncPeriod     = 10 * time.Minute
	reconcileTimeout = 10 * time.Minute

	cloudinitConfigNameAnnotation      = "meta.helm.sh/config-name"
	cloudinitConfigNamespaceAnnotation = "meta.helm.sh/config-namespace"
)

const (
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
			logger:               logger,
			client:               mgr.GetClient(),
			usage:                resource.NewProviderConfigUsageTracker(mgr.GetClient(), &cloudinitv1alpha1.ProviderConfigUsage{}),
			newRestConfigFn:      clients.NewRestConfig,
			newKubeClientFn:      clients.NewKubeClient,
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
	newRestConfigFn      func(kubeconfig []byte) (*rest.Config, error)
	newKubeClientFn      func(config *rest.Config) (client.Client, error)
	newCloudInitClientFn func(log logging.Logger, config *rest.Config, namespace string, wait bool) (cloudinitClient.Client, error)
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

	var rc *rest.Config
	var err error

	s := p.Spec.Credentials.Source
	switch s { //nolint:exhaustive
	case xpv1.CredentialsSourceInjectedIdentity:
		rc, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, errFailedToCreateRestConfig)
		}
	case xpv1.CredentialsSourceSecret:
		ref := p.Spec.Credentials.SecretRef
		if ref == nil {
			return nil, errors.New(errCredSecretNotSet)
		}

		key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		d, err := getSecretData(ctx, c.client, key)
		if err != nil {
			return nil, errors.Wrap(err, errProviderSecretNotRetrieved)
		}
		kc, f := d[ref.Key]
		if !f {
			return nil, errors.Errorf(errProviderSecretValueForKeyNotFound, ref.Key)
		}
		rc, err = c.newRestConfigFn(kc)
		if err != nil {
			return nil, errors.Wrap(err, errFailedToCreateRestConfig)
		}
	default:
		return nil, errors.Errorf(errFmtUnsupportedCredSource, s)
	}

	k, err := c.newKubeClientFn(rc)
	if err != nil {
		return nil, errors.Wrap(err, errNewKubernetesClient)
	}

	h, err := c.newCloudInitClientFn(c.logger, rc, cr.Spec.ForProvider.Namespace, cr.Spec.ForProvider.Wait)
	if err != nil {
		return nil, errors.Wrap(err, errNewKubernetesClient)
	}

	return &cloudinitExternal{
		logger:    l,
		localKube: c.client,
		kube:      k,
		cloudinit: h,
		patch:     newPatcher(),
	}, nil
}

type cloudinitExternal struct {
	logger    logging.Logger
	localKube client.Client
	kube      client.Client
	cloudinit helmClient.Client
	patch     Patcher
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
		ResourceUpToDate:  cr.Status.Synced && !(shouldRollBack(cr) && !rollBackLimitReached(cr)),
		ConnectionDetails: cd,
	}, nil
}

type deployAction func(config string, chart *chart.Chart, vals map[string]interface{}, patches []ktype.Patch) (*release.Config, error)

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

	if shouldRollBack(cr) {
		e.logger.Debug("Last config failed")
		if !rollBackLimitReached(cr) {
			// Rollback
			e.logger.Debug("Will rollback/uninstall to retry")
			cr.Status.Failed++
			// If it's the first revision of a Config, rollback would fail since there is no previous revision.
			// We need to uninstall to retry.
			if cr.Status.AtProvider.Revision == 1 {
				e.logger.Debug("Uninstalling")
				return managed.ExternalUpdate{}, e.cloudinit.Uninstall(meta.GetExternalName(cr))
			}
			e.logger.Debug("Rolling back to previous config version")
			return managed.ExternalUpdate{}, e.cloudinit.Rollback(meta.GetExternalName(cr))
		}
		e.logger.Debug("Reached max rollback retries, will not retry")
		return managed.ExternalUpdate{}, nil
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

func shouldRollBack(cr *v1alpha1.Config) bool {
	return rollBackEnabled(cr) &&
		((cr.Status.Synced && cr.Status.AtProvider.State == config.StatusFailed) ||
			(cr.Status.AtProvider.State == config.StatusPendingInstall) ||
			(cr.Status.AtProvider.State == config.StatusPendingUpgrade))
}

func rollBackEnabled(cr *v1alpha1.Config) bool {
	return cr.Spec.RollbackRetriesLimit != nil
}
func rollBackLimitReached(cr *v1alpha1.Config) bool {
	return cr.Status.Failed >= *cr.Spec.RollbackRetriesLimit
}
