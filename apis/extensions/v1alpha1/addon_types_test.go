/*
Copyright ApeCloud, Inc.

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

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/apecloud/kubeblocks/internal/testutil"
)

func TestSelectorRequirementString(t *testing.T) {
	g := NewGomegaWithT(t)
	r := SelectorRequirement{
		Key:      KubeGitVersion,
		Operator: Contains,
	}
	// just to fullfile test coverage
	g.Expect(r.String()).To(Equal(r.String()))
}

func TestSelectorRequirementNoOperator(t *testing.T) {
	g := NewGomegaWithT(t)
	var r *SelectorRequirement
	g.Expect(r.MatchesFromConfig()).Should(BeFalse())
	r = &SelectorRequirement{
		Key: KubeGitVersion,
	}
	g.Expect(r.MatchesFromConfig()).Should(BeFalse())
}

func TestSelectorRequirementContains(t *testing.T) {
	g := NewGomegaWithT(t)
	const distro = "k3s"
	testutil.SetKubeServerVersionWithDistro("1", "24", "0", distro)
	r := SelectorRequirement{
		Key:      KubeGitVersion,
		Operator: Contains,
	}

	// empty value with no matching
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())

	// single value matching
	r.Values = []string{
		distro,
	}
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())

	// multiple values matching
	r.Values = []string{
		"eks",
		distro,
	}
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())

	// multiple values with no matching
	r.Values = []string{
		"eks",
		"ack",
	}
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())
}

func TestSelectorRequirementNotContains(t *testing.T) {
	g := NewGomegaWithT(t)
	const distro = "k3s"
	testutil.SetKubeServerVersionWithDistro("1", "24", "0", distro)
	r := SelectorRequirement{
		Key:      KubeGitVersion,
		Operator: DoesNotContain,
	}
	// empty value with no matching
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())

	// single value matching
	r.Values = []string{
		distro,
	}
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())

	// multiple values matching
	r.Values = []string{
		"eks",
		distro,
	}
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())

	// multiple values with no matching
	r.Values = []string{
		"eks",
		"ack",
	}
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())
}

func TestSelectorRequirementMatchRegex(t *testing.T) {
	g := NewGomegaWithT(t)
	const distro = "k3s"
	testutil.SetKubeServerVersionWithDistro("1", "24", "0", distro)
	r := SelectorRequirement{
		Key:      KubeGitVersion,
		Operator: MatchRegex,
	}

	// empty value with no matching
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())

	// single value matching
	r.Values = []string{
		distro,
	}
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())

	// multiple values matching
	r.Values = []string{
		"eks",
		distro,
	}
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())

	// multiple values with no matching
	r.Values = []string{
		"eks",
		"ack",
	}
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())

	// semver regex
	r.Operator = MatchRegex
	r.Values = []string{
		"^v?(0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(?:-((?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\\+([0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?$",
	}
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())
	testutil.SetKubeServerVersion("1", "24", "0")
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())

	// pass KubeVersion
	r = SelectorRequirement{
		Key:      KubeVersion,
		Operator: MatchRegex,
	}
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())

	// major.minor regex
	r.Values = []string{
		"^(0|[1-9]\\d*)\\.(0|[1-9]\\d*)$",
	}
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())

}

func TestSelectorRequirementNotMatchRegex(t *testing.T) {
	g := NewGomegaWithT(t)
	const distro = "k3s"
	testutil.SetKubeServerVersionWithDistro("1", "24", "0", distro)
	r := SelectorRequirement{
		Key:      KubeGitVersion,
		Operator: DoesNotMatchRegex,
	}
	// empty value with no matching
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())

	// single value matching
	r.Values = []string{
		distro,
	}
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())

	// multiple values matching
	r.Values = []string{
		"eks",
		distro,
	}
	g.Expect(r.MatchesFromConfig()).ShouldNot(BeTrue())

	// multiple values with no matching
	r.Values = []string{
		"eks",
		"ack",
	}
	g.Expect(r.MatchesFromConfig()).Should(BeTrue())
}

func TestHelmInstallSpecBuildMergedValues(t *testing.T) {
	g := NewGomegaWithT(t)
	newInt32 := func(i int32) *int32 {
		return &i
	}
	newBool := func(b bool) *bool {
		return &b
	}

	mappingName := func(name, jsonSubKey string) string {
		return fmt.Sprintf("%s.%s", name, jsonSubKey)
	}

	const (
		// mapping sub-json-path values
		replicas    = "replicaCount"
		pvEnabled   = "persistentVolume.storageClass"
		sc          = "persistentVolume.storageClass"
		tolerations = "tolerations"
		pvSize      = "persistentVolume.size"
		cpuReq      = "resources.requests.cpu"
		cpuLim      = "resources.limits.cpu"
		memReq      = "resources.requests.memory"
		memLim      = "resources.limits.memory"
	)

	buildHelmValuesMappingItem := func(name string) HelmValuesMappingItem {
		return HelmValuesMappingItem{
			HelmValueMap: HelmValueMapType{
				ReplicaCount: mappingName(name, replicas),
				StorageClass: mappingName(name, sc),
				PVEnabled:    mappingName(name, pvEnabled),
			},
			HelmJSONMap: HelmJSONValueMapType{
				Tolerations: mappingName(name, tolerations),
			},
			ResourcesMapping: &ResourceMappingItem{
				Storage: mappingName(name, pvSize),
				CPU: &ResourceReqLimItem{
					Requests: mappingName(name, cpuReq),
					Limits:   mappingName(name, cpuLim),
				},
				Memory: &ResourceReqLimItem{
					Requests: mappingName(name, memReq),
					Limits:   mappingName(name, memLim),
				},
			},
		}
	}

	helmValues := &HelmTypeInstallSpec{
		InstallValues: HelmInstallValues{
			SetValues:     []string{},
			SetJSONValues: []string{},
		},
		ValuesMapping: HelmValuesMapping{
			HelmValuesMappingItem: buildHelmValuesMappingItem("primary"),
			ExtraItems: []HelmValuesMappingExtraItem{
				{
					Name:                  "extra",
					HelmValuesMappingItem: buildHelmValuesMappingItem("extra"),
				},
			},
		},
	}

	bulidInstallSpecItem := func() AddonInstallSpecItem {
		toleration := []map[string]string{
			{
				"key":      "taint-key",
				"effect":   "NoSchedule",
				"operator": "Exists",
				"value":    "taint-value",
			},
		}
		tolerationJSON, _ := json.Marshal(toleration)
		return AddonInstallSpecItem{
			Replicas:     newInt32(1),
			StorageClass: "local-path",
			PVEnabled:    newBool(true),
			Tolerations:  string(tolerationJSON),
			Resources: ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:     resource.MustParse("1000m"),
					corev1.ResourceMemory:  resource.MustParse("256Mi"),
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2000m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
		}
	}

	installSpec := AddonInstallSpec{
		AddonInstallSpecItem: bulidInstallSpecItem(),
		ExtraItems: []AddonInstallExtraItem{
			{
				Name:                 "extra",
				AddonInstallSpecItem: bulidInstallSpecItem(),
			},
		},
	}

	mergedValues := helmValues.BuildMergedValues(&installSpec)

	m := map[string]*AddonInstallSpecItem{
		"primary": &installSpec.AddonInstallSpecItem,
		"extra":   &installSpec.ExtraItems[0].AddonInstallSpecItem,
	}

	for k, v := range m {
		g.Expect(fmt.Sprintf("%s=%d",
			mappingName(k, replicas), *(v.Replicas))).Should(BeElementOf(mergedValues.SetValues))
		g.Expect(fmt.Sprintf("%s=%s",
			mappingName(k, sc), v.StorageClass)).Should(BeElementOf(mergedValues.SetValues))
		g.Expect(fmt.Sprintf("%s=%v",
			mappingName(k, pvEnabled), *(v.PVEnabled))).Should(BeElementOf(mergedValues.SetValues))
		g.Expect(fmt.Sprintf("%s=%v",
			mappingName(k, pvSize), v.Resources.Requests[corev1.ResourceStorage].ToUnstructured())).Should(BeElementOf(mergedValues.SetValues))
		g.Expect(fmt.Sprintf("%s=%s",
			mappingName(k, cpuReq), v.Resources.Requests[corev1.ResourceCPU].ToUnstructured())).Should(BeElementOf(mergedValues.SetValues))
		g.Expect(fmt.Sprintf("%s=%s",
			mappingName(k, memReq), v.Resources.Requests[corev1.ResourceMemory].ToUnstructured())).Should(BeElementOf(mergedValues.SetValues))
		g.Expect(fmt.Sprintf("%s=%s",
			mappingName(k, cpuLim), v.Resources.Limits[corev1.ResourceCPU].ToUnstructured())).Should(BeElementOf(mergedValues.SetValues))
		g.Expect(fmt.Sprintf("%s=%s",
			mappingName(k, memLim), v.Resources.Limits[corev1.ResourceMemory].ToUnstructured())).Should(BeElementOf(mergedValues.SetValues))
		g.Expect(fmt.Sprintf("%s=%s",
			mappingName(k, tolerations), v.Tolerations)).Should(BeElementOf(mergedValues.SetJSONValues))
		g.Expect(fmt.Sprintf("%s=%s",
			mappingName(k, tolerations), v.Tolerations)).ShouldNot(BeElementOf(mergedValues.SetValues))
	}

	// test unset storageClass
	installSpec.StorageClass = "-"
	mergedValues = helmValues.BuildMergedValues(&installSpec)
	g.Expect(fmt.Sprintf("%s=null",
		mappingName("primary", sc))).Should(BeElementOf(mergedValues.SetValues))
}

func TestAddonSpecMisc(t *testing.T) {
	g := NewGomegaWithT(t)
	var addonSpec *AddonSpec
	g.Expect(addonSpec.GetSortedDefaultInstallValues()).Should(BeNil())
	addonSpec = &AddonSpec{}
	g.Expect(addonSpec.InstallSpec.GetEnabled()).Should(BeFalse())
	g.Expect(addonSpec.Helm.BuildMergedValues(nil)).Should(BeEquivalentTo(HelmInstallValues{}))
	addonSpec.InstallSpec = &AddonInstallSpec{
		Enabled:              true,
		AddonInstallSpecItem: NewAddonInstallSpecItem(),
	}
	g.Expect(addonSpec.InstallSpec.GetEnabled()).Should(BeTrue())

	addonSpec.DefaultInstallValues = []AddonDefaultInstallSpecItem{
		{
			AddonInstallSpec: AddonInstallSpec{
				Enabled: true,
			},
			Selectors: []SelectorRequirement{
				{
					Key:      KubeVersion,
					Operator: Contains,
					Values:   []string{"1.0.0"},
				},
			},
		},
		{
			AddonInstallSpec: AddonInstallSpec{
				Enabled: true,
			},
		},
	}

	di := addonSpec.GetSortedDefaultInstallValues()
	g.Expect(di).Should(HaveLen(2))
}
