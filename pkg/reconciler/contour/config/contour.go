/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"knative.dev/serving/pkg/apis/networking/v1alpha1"
)

const (
	// ContourConfigName is the name of the configmap containing all
	// customizations for contour related features.
	ContourConfigName = "config-contour"

	visibilityConfigKey = "visibility"
)

// Contour contains contour related configuration defined in the
// contour config map.
type Contour struct {
	VisibilityKeys    map[v1alpha1.IngressVisibility]sets.String
	VisibilityClasses map[v1alpha1.IngressVisibility]string
}

type visibilityValue struct {
	Class   string `json:"class"`
	Service string `json:"service"`
}

// NewContourFromConfigMap creates an Contour config from the supplied ConfigMap
func NewContourFromConfigMap(configMap *corev1.ConfigMap) (*Contour, error) {
	contour := &Contour{
		VisibilityKeys:    map[v1alpha1.IngressVisibility]sets.String{},
		VisibilityClasses: map[v1alpha1.IngressVisibility]string{},
	}

	v, ok := configMap.Data[visibilityConfigKey]
	if !ok {
		// These are the defaults.
		contour.VisibilityKeys = map[v1alpha1.IngressVisibility]sets.String{
			v1alpha1.IngressVisibilityClusterLocal: sets.NewString("projectcontour/envoy-internal"),
			v1alpha1.IngressVisibilityExternalIP:   sets.NewString("projectcontour/envoy-external"),
		}
		contour.VisibilityClasses = map[v1alpha1.IngressVisibility]string{
			v1alpha1.IngressVisibilityClusterLocal: "contour-internal",
			v1alpha1.IngressVisibilityExternalIP:   "contour",
		}
	} else {
		value := make(map[v1alpha1.IngressVisibility]visibilityValue)
		if err := yaml.Unmarshal([]byte(v), value); err != nil {
			return nil, err
		}

		if _, ok := value[v1alpha1.IngressVisibilityClusterLocal]; !ok {
			return nil, fmt.Errorf("visibility must contain %q with class and service",
				v1alpha1.IngressVisibilityClusterLocal)
		}
		if _, ok := value[v1alpha1.IngressVisibilityExternalIP]; !ok {
			return nil, fmt.Errorf("visibility must contain %q with class and service",
				v1alpha1.IngressVisibilityExternalIP)
		}

		for key, value := range value {
			// Check that the visibility makes sense.
			switch key {
			case v1alpha1.IngressVisibilityClusterLocal, v1alpha1.IngressVisibilityExternalIP:
			default:
				return nil, fmt.Errorf("Unrecognized visibility: %q", key)
			}

			// See if the Service is a valid namespace/name token.
			_, _, err := cache.SplitMetaNamespaceKey(value.Service)
			if err != nil {
				return nil, err
			}
			contour.VisibilityKeys[key] = sets.NewString(value.Service)
			contour.VisibilityClasses[key] = value.Class
		}
	}
	return contour, nil
}
