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

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1alpha1 "github.com/crossplane-contrib/provider-cloudinit/apis/config/v1alpha1"
	clients "github.com/crossplane-contrib/provider-cloudinit/internal/clients"
	"github.com/crossplane-contrib/provider-cloudinit/internal/cloudinit"
)

// Error strings.
const (
	errNotConfig           = "managed resource is not a Config"
	errGetPart             = "cannot get ConfigMap referenced as part"
	errGetConfigMap        = "cannot get ConfigMap"
	errCreateConfigMap     = "cannot create ConfigMap"
	errDeleteConfigMap     = "cannot delete ConfigMap"
	errManagedConfigUpdate = "cannot update managed Config resource"
	errNotRender           = "cannot render cloud-init data"
	errUpdateConfigMap     = "cannot update ConfigMap"

	configMapKey = "cloud-init"
)

// Setup adds a controller that reconciles
// Config managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.ConfigGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Config{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.ConfigGroupVersionKind),
			managed.WithExternalConnecter(&ctrlConnector{kube: mgr.GetClient()}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithConnectionPublishers(),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type ctrlConnector struct {
	kube client.Client
}

func (c *ctrlConnector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	// TODO(displague) construct some client wrapper?
	return &ctrlClients{kube: c.kube}, nil
}

type ctrlClients struct {
	kube client.Client
}

func (e *ctrlClients) renderCloudInit(ctx context.Context, cr *v1alpha1.Config) (string, error) {
	cl := clients.NewCloudInitClient(cr.Spec.ForProvider.Gzip, cr.Spec.ForProvider.Gzip, cr.Spec.ForProvider.Boundary)
	for _, p := range cr.Spec.ForProvider.Parts {
		content := p.Content

		if p.ConfigMapKeyRef != nil {
			partCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      p.ConfigMapKeyRef.Name,
					Namespace: p.ConfigMapKeyRef.Namespace,
				},
			}
			partNsn := types.NamespacedName{
				Name:      partCM.GetName(),
				Namespace: partCM.GetNamespace(),
			}
			if err := e.kube.Get(ctx, partNsn, partCM); err != nil {
				if p.ConfigMapKeyRef.Optional {
					// TODO(displague) log that this optional configmap was not available
					continue
				}
				return "", errors.Wrap(resource.Ignore(clients.IsErrorNotFound, err), errGetPart)
			}
			key := p.ConfigMapKeyRef.Key
			if key == "" {
				key = configMapKey
			}
			content = partCM.Data[key]
		}

		cl.AppendPart(content, p.Filename, p.ContentType, p.MergeType)
	}

	return cloudinit.RenderCloudinitConfig(cl)
}

func generateConfigMap(cr *v1alpha1.Config, want string) *corev1.ConfigMap {
	key := cr.Spec.WriteCloudInitToRef.Key
	if key == "" {
		key = configMapKey
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.WriteCloudInitToRef.Name,
			Namespace: cr.Spec.WriteCloudInitToRef.Namespace,
		},
		Data: map[string]string{
			key: want,
		},
	}
}

func (e *ctrlClients) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotConfig)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.WriteCloudInitToRef.Name,
			Namespace: cr.Spec.WriteCloudInitToRef.Namespace,
		},
	}
	nsn := types.NamespacedName{
		Name:      cm.GetName(),
		Namespace: cm.GetNamespace(),
	}
	err := e.kube.Get(ctx, nsn, cm)

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(resource.Ignore(clients.IsErrorNotFound, err), errGetConfigMap)
	}

	want, err := e.renderCloudInit(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	key := cr.Spec.WriteCloudInitToRef.Key
	if key == "" {
		key = configMapKey
	}
	got := cm.Data[key]

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(resource.Ignore(clients.IsErrorNotFound, err), errNotRender)
	}

	eo := managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: got == want}

	currentSpec := cr.Spec.ForProvider.DeepCopy()
	// cloudinitClient.LateInitializeSpec(&cr.Spec.ForProvider, *observed)
	if !cmp.Equal(currentSpec, &cr.Spec.ForProvider) {
		if err := e.kube.Update(ctx, cr); err != nil {
			return eo, errors.Wrap(err, errManagedConfigUpdate)
		}
	}

	cr.SetConditions(xpv1.Available())
	return eo, errors.Wrap(err, errManagedConfigUpdate)
}

func (e *ctrlClients) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotConfig)
	}

	cr.Status.SetConditions(xpv1.Creating())

	want, err := e.renderCloudInit(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errNotRender)
	}

	cm := generateConfigMap(cr, want)
	err = e.kube.Create(ctx, cm)
	return managed.ExternalCreation{}, errors.Wrap(err, errCreateConfigMap)
}

func (e *ctrlClients) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotConfig)
	}

	want, err := e.renderCloudInit(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errNotRender)
	}

	cm := generateConfigMap(cr, want)
	err = e.kube.Update(ctx, cm)
	return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateConfigMap)

}

func (e *ctrlClients) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return errors.New(errNotConfig)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	nsn := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.WriteCloudInitToRef.Name,
			Namespace: cr.Spec.WriteCloudInitToRef.Namespace,
		},
	}

	err := e.kube.Delete(ctx, nsn)
	return errors.Wrap(resource.Ignore(clients.IsErrorNotFound, err), errDeleteConfigMap)
}
