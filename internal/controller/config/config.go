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
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1alpha1 "github.com/crossplane-contrib/provider-cloudinit/apis/config/v1alpha1"
	clients "github.com/crossplane-contrib/provider-cloudinit/internal/clients"
	cloudinitClient "github.com/crossplane-contrib/provider-cloudinit/internal/clients/cloudinit"
	"github.com/crossplane-contrib/provider-cloudinit/internal/cloudinit"
)

// Error strings.
const (
	errNotConfigMap        = "managed resource is not a ConfigMap"
	errGetConfigMap        = "cannot get ConfigMap"
	errCreateConfigMap     = "cannot create ConfigMap"
	errDeleteConfigMap     = "cannot delete ConfigMap"
	errManagedConfigUpdate = "cannot update managed Config resource"
	errNotRender           = "cannot render cloud-init data"
)

// SetupConfig adds a controller that reconciles
// Config managed resources.
func SetupConfig(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(v1alpha1.ConfigGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
		}).
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
	s := clients.NewCloudInitClient(false, false, "")
	return &ctrlClients{kube: c.kube, client: s}, nil
}

type ctrlClients struct {
	kube   client.Client
	client *cloudinitClient.Client
}

func (e *ctrlClients) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotConfigMap)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.WriteCloudInitToRef.Name,
			Namespace: cr.GetNamespace(),
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
	got := cm.Data["cloud-init"]

	want, err := cloudinit.RenderCloudinitConfig(e.client)

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

	// cr.Status.AtProvider = cloudinitClient.GenerateGlobalAddressObservation(*observed)

	/*
		switch cr.Status.AtProvider.Status {
		case v1alpha1.StatusReserving:
			cr.SetConditions(xpv1.Creating())
		case v1alpha1.StatusInUse, v1alpha1.StatusReserved:
			cr.SetConditions(xpv1.Available())
		}
	*/
	return eo, errors.Wrap(err, errManagedConfigUpdate)
}

func (e *ctrlClients) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotConfigMap)
	}

	cr.Status.SetConditions(xpv1.Creating())

	data, err := cloudinit.RenderCloudinitConfig(e.client)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errNotRender)

	}
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Name:      cr.Spec.WriteCloudInitToRef.Name,
			Namespace: cr.GetNamespace(),
		},
		Data: map[string]string{
			"cloud-init": data,
		},
	}
	err = e.kube.Create(ctx, cm)
	return managed.ExternalCreation{}, errors.Wrap(err, errCreateConfigMap)
}

func (e *ctrlClients) Update(_ context.Context, _ resource.Managed) (managed.ExternalUpdate, error) {
	// Global addresses cannot be updated.
	return managed.ExternalUpdate{}, nil
}

func (e *ctrlClients) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Config)
	if !ok {
		return errors.New(errNotConfigMap)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	nsn := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.WriteCloudInitToRef.Name,
			Namespace: cr.GetNamespace(),
		},
	}

	err := e.kube.Delete(ctx, nsn)
	return errors.Wrap(resource.Ignore(clients.IsErrorNotFound, err), errDeleteConfigMap)
}
