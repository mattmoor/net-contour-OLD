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
	"testing"

	_ "github.com/mattmoor/net-contour/pkg/client/injection/informers/projectcontour/v1/httpproxy/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/pod/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/service/fake"
	_ "knative.dev/serving/pkg/client/injection/informers/networking/v1alpha1/ingress/fake"

	"github.com/mattmoor/net-contour/pkg/reconciler/contour/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/system"
	"knative.dev/serving/pkg/network"

	. "knative.dev/pkg/reconciler/testing"
	// . "knative.dev/serving/pkg/reconciler/testing/v1alpha1"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	c := NewController(ctx, configmap.NewStaticWatcher(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      config.ContourConfigName,
		},
	}, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      network.ConfigName,
		},
	}))

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

// TODO(mattmoor): DO NOT SUBMIT
// func TestKeyLookup(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		key     string
// 		objects []runtime.Object
// 		wantErr bool
// 	}{{
// 		name:    "bad key",
// 		key:     "this/is/a/bad/key",
// 		wantErr: true,
// 	}, {
// 		name:    "missing namespace",
// 		key:     "missing-namespace",
// 		wantErr: true,
// 	}, {
// 		name:    "service not found",
// 		key:     "not/found",
// 		wantErr: true,
// 	}, {
// 		name:    "endpoints not found",
// 		key:     "no/endpoints",
// 		wantErr: true,
// 		objects: []runtime.Object{
// 			&corev1.Service{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Namespace: "no",
// 					Name:      "endpoints",
// 				},
// 			},
// 		},
// 	}, {
// 		name:    "everything is fine",
// 		key:     "peachy/keen",
// 		wantErr: false,
// 		objects: []runtime.Object{
// 			&corev1.Service{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Namespace: "peachy",
// 					Name:      "keen",
// 				},
// 			},
// 			&corev1.Endpoints{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Namespace: "peachy",
// 					Name:      "keen",
// 				},
// 			},
// 		},
// 	}}

// 	for _, test := range tests {
// 		t.Run(test.name, func(t *testing.T) {
// 			logger := testlogging.TestLogger(t)
// 			l := NewListers(test.objects)
// 			kl := keyLookup(logger, l.GetK8sServiceLister(), l.GetEndpointsLister())

// 			_, _, _, err := kl(test.key)
// 			if (err != nil) != test.wantErr {
// 				t.Errorf("keyLookup() = %v, wanted error is %v", err, test.wantErr)
// 			}
// 		})
// 	}
// }
