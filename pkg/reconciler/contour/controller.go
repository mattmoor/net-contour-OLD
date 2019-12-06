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

	servingclient "knative.dev/serving/pkg/client/injection/client"
	ingressinformer "knative.dev/serving/pkg/client/injection/informers/networking/v1alpha1/ingress"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/serving/pkg/apis/networking"
	"knative.dev/serving/pkg/reconciler"
)

const (
	controllerAgentName = "contour-controller"

	// ContourIngressClassName value for specifying knative's Contour
	// Ingress reconciler.
	ContourIngressClassName = "contour.ingress.networking.knative.dev"
)

// NewController returns a new Ingress controller for Project Contour.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	ingressInformer := ingressinformer.Get(ctx)

	c := &Reconciler{
		Client: servingclient.Get(ctx),
		Lister: ingressInformer.Lister(),
		Recorder: record.NewBroadcaster().NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}
	impl := controller.NewImpl(c, logger, "ContourIngresses")

	logger.Info("Setting up event handlers")

	myFilterFunc := reconciler.AnnotationFilterFunc(networking.IngressClassAnnotationKey, ContourIngressClassName, false)
	ingressHandler := cache.FilteringResourceEventHandler{
		FilterFunc: myFilterFunc,
		Handler:    controller.HandleAll(impl.Enqueue),
	}
	ingressInformer.Informer().AddEventHandler(ingressHandler)

	// TODO(mattmoor): Child resources.

	return impl
}
