/*
Copyright ApeCloud Inc.

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

package cluster

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type UpdatableFlag struct {
	TerminationPolicy string `json:"terminationPolicy"`
	PodAntiAffinity   string `json:"podAntiAffinity"`
	Monitor           string `json:"monitor"`
	EnableAllLogs     string `json:"enableAllLogs"`

	// TopologyKeys if TopologyKeys is nil, add omitempty json tag.
	// because CueLang can not covert null to list.
	TopologyKeys []string `json:"topologyKeys,omitempty"`

	NodeLabels     map[string]string   `json:"nodeLabels,omitempty"`
	Tolerations    []map[string]string `json:"tolerations,omitempty"`
	TolerationsRaw []string            `json:"-"`
}

// buildPatch build a patch to be patched to a cluster
func buildPatch(u *UpdatableFlag) ([]byte, error) {
	spec := map[string]interface{}{}
	if u.TerminationPolicy != "" {
		if err := unstructured.SetNestedField(spec, u.TerminationPolicy, "terminationPolicy"); err != nil {
			return nil, err
		}
	}

	// affinity
	affinity := &dbaasv1alpha1.Affinity{
		TopologyKeys:    u.TopologyKeys,
		PodAntiAffinity: dbaasv1alpha1.PodAntiAffinity(u.PodAntiAffinity),
		NodeLabels:      u.NodeLabels,
	}
	aff, err := runtime.DefaultUnstructuredConverter.ToUnstructured(affinity)
	if err != nil {
		return nil, err
	}
	if err = unstructured.SetNestedField(spec, aff, "affinity"); err != nil {
		return nil, err
	}

	// tolerations
	tolerations := make([]map[string]string, 0)
	for _, tolerationRaw := range u.TolerationsRaw {
		toleration := map[string]string{}
		for _, entries := range strings.Split(tolerationRaw, ",") {
			parts := strings.SplitN(entries, "=", 2)
			toleration[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
		tolerations = append(tolerations, toleration)
	}
	if len(tolerations) > 0 {
		if err = unstructured.SetNestedField(spec, tolerations, "tolerations"); err != nil {
			return nil, err
		}
	}

	// monitor

	// logs

	obj := unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": spec,
		},
	}
	bytes, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return bytes, err
}
