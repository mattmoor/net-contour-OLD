/*
Copyright 2019 The Knative Authors

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

package contour

import (
	"context"
	"fmt"
	"reflect"

	contourclientset "github.com/mattmoor/net-contour/pkg/client/clientset/versioned"
	contourlisters "github.com/mattmoor/net-contour/pkg/client/listers/projectcontour/v1"
	"github.com/mattmoor/net-contour/pkg/reconciler/contour/resources"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/serving/pkg/apis/networking/v1alpha1"
	clientset "knative.dev/serving/pkg/client/clientset/versioned"
	listers "knative.dev/serving/pkg/client/listers/networking/v1alpha1"
)

// Reconciler implements controller.Reconciler for Ingress resources.
type Reconciler struct {
	// Client is used to write back status updates.
	Client        clientset.Interface
	ContourClient contourclientset.Interface

	// Listers index properties about resources
	Lister        listers.IngressLister
	ContourLister contourlisters.HTTPProxyLister

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder record.EventRecorder
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// If our controller has configuration state, we'd "freeze" it and
	// attach the frozen configuration to the context.
	//    ctx = r.configStore.ToContext(ctx)

	// Get the resource with this namespace/name.
	original, err := r.Lister.Ingresses(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("resource %q no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}
	// Don't modify the informers copy.
	resource := original.DeepCopy()

	// Reconcile this copy of the resource and then write back any status
	// updates regardless of whether the reconciliation errored out.
	reconcileErr := r.reconcile(ctx, resource)
	if equality.Semantic.DeepEqual(original.Status, resource.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err = r.updateStatus(resource); err != nil {
		logger.Warnw("Failed to update resource status", zap.Error(err))
		r.Recorder.Eventf(resource, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for %q: %v", resource.Name, err)
		return err
	}
	if reconcileErr != nil {
		r.Recorder.Event(resource, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, ing *v1alpha1.Ingress) error {
	if ing.GetDeletionTimestamp() != nil {
		// Check for a DeletionTimestamp.  If present, elide the normal reconcile logic.
		// When a controller needs finalizer handling, it would go here.
		return nil
	}
	ing.Status.InitializeConditions()

	if err := r.reconcileProxies(ctx, ing); err != nil {
		return err
	}

	ing.Status.ObservedGeneration = ing.Generation
	return nil
}

func (r *Reconciler) reconcileProxies(ctx context.Context, ing *v1alpha1.Ingress) error {
	pl, err := resources.MakeHTTPProxies(ctx, ing)
	if err != nil {
		return err
	}

	for _, proxy := range pl {
		selector := labels.Set(map[string]string{
			"ingress.parent": ing.Name,
			"ingress.fqdn":   proxy.Spec.VirtualHost.Fqdn,
		}).AsSelector()
		elts, err := r.ContourLister.HTTPProxies(ing.Namespace).List(selector)
		if err != nil {
			return err
		}
		if len(elts) == 0 {
			_, err := r.ContourClient.ProjectcontourV1().HTTPProxies(proxy.Namespace).Create(proxy)
			if err != nil {
				return err
			}
			continue
		}
		update := elts[0].DeepCopy()
		update.Annotations = proxy.Annotations
		update.Labels = proxy.Labels
		update.Spec = proxy.Spec
		_, err = r.ContourClient.ProjectcontourV1().HTTPProxies(proxy.Namespace).Update(update)
		if err != nil {
			return err
		}
	}

	err = r.ContourClient.ProjectcontourV1().HTTPProxies(ing.Namespace).DeleteCollection(
		&metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf(
				"ingress.parent=%s,ingress.generation!=%d", ing.Name, ing.Generation),
		})
	if err != nil {
		return err
	}

	// TODO(mattmoor): Do this for real.
	ing.Status.MarkNetworkConfigured()
	ing.Status.MarkLoadBalancerReady(
		[]v1alpha1.LoadBalancerIngressStatus{},
		[]v1alpha1.LoadBalancerIngressStatus{},
		[]v1alpha1.LoadBalancerIngressStatus{{
			DomainInternal: "envoy-internal.projectcontour.svc.cluster.local",
		}})

	return nil
}

// Update the Status of the resource.  Caller is responsible for checking
// for semantic differences before calling.
func (r *Reconciler) updateStatus(desired *v1alpha1.Ingress) (*v1alpha1.Ingress, error) {
	actual, err := r.Lister.Ingresses(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(actual.Status, desired.Status) {
		return actual, nil
	}
	// Don't modify the informers copy
	existing := actual.DeepCopy()
	existing.Status = desired.Status
	return r.Client.NetworkingV1alpha1().Ingresses(desired.Namespace).UpdateStatus(existing)
}
