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

package resources

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/serving/pkg/apis/networking/v1alpha1"
)

func MakeHTTPProxies(ctx context.Context, ing *v1alpha1.Ingress) ([]*v1.HTTPProxy, error) {
	proxies := []*v1.HTTPProxy{}
	for _, rule := range ing.Spec.Rules {
		class := "contour"
		if rule.Visibility == v1alpha1.IngressVisibilityClusterLocal {
			class = "contour-internal"
		}

		if len(rule.HTTP.Paths) != 1 {
			return nil, errors.New("multiple paths is not supported")
		}
		path := rule.HTTP.Paths[0]

		var top *v1.TimeoutPolicy
		if path.Timeout != nil {
			top = &v1.TimeoutPolicy{
				Response: path.Timeout.Duration.String(),
			}
		}

		var retry *v1.RetryPolicy
		if path.Retries != nil {
			retry = &v1.RetryPolicy{
				NumRetries:    uint32(path.Retries.Attempts),
				PerTryTimeout: path.Retries.PerTryTimeout.Duration.String(),
			}
		}

		svcs := make([]v1.Service, 0, len(path.Splits))
		for _, split := range path.Splits {
			svcs = append(svcs, v1.Service{
				Name:   split.ServiceName,
				Port:   split.ServicePort.IntValue(),
				Weight: uint32(split.Percent),
				// TODO(mattmoor): AppendHeaders
			})
		}

		base := v1.HTTPProxy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ing.Namespace,
				Labels: map[string]string{
					"ingress.generation": fmt.Sprintf("%d", ing.Generation),
					"ingress.parent":     ing.Name,
				},
				Annotations: map[string]string{
					"kubernetes.io/ingress.class": class,
				},
				OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(ing)},
			},
			Spec: v1.HTTPProxySpec{
				// VirtualHost: filled in below
				Routes: []v1.Route{{
					TimeoutPolicy: top,
					RetryPolicy:   retry,
					Services:      svcs,
				}},
			},
		}

		for _, host := range rule.Hosts {
			hostProxy := base.DeepCopy()
			hostProxy.Name = kmeta.ChildName(ing.Name, host)
			hostProxy.Spec.VirtualHost = &v1.VirtualHost{
				Fqdn: host,
			}
			hostProxy.Labels["ingress.fqdn"] = host

			proxies = append(proxies, hostProxy)
		}
	}

	return proxies, nil
}
