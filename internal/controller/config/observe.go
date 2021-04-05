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

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/crossplane-contrib/provider-cloudinit/apis/config/v1alpha1"
)

const (
	errConfigInfoNilInObservedRelease = "config info is nil in observed cloudinit release"
	errChartNilInObservedConfig       = "chart field is nil in observed cloudinit config"
	errChartMetaNilInObservedConfig   = "chart metadata field is nil in observed cloudinit config"
	errObjectNotPartOfConfig          = "object is not part of config: %v"
)

// generateObservation generates config observation for the input release object
func generateObservation(in *config.Config) v1alpha1.ReleaseObservation {
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
	return s == config.StatusPendingInstall || s == release.StatusPendingUpgrade || s == release.StatusPendingRollback
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
