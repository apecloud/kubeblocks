/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package helm

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/exp/maps"

	"go.uber.org/zap/buffer"
	helm "helm.sh/helm/v3/pkg/release"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	resourceName = "kubeblocks-opsrequest-editor-role, ClusterRole (rbac.authorization.k8s.io)"
	exceptRemove = `--- kubeblocks-opsrequest-editor-role, ClusterRole (rbac.authorization.k8s.io) 0.5.1-fake
+++ 
@@ -1,13 +1 @@
-apiVersion: rbac.authorization.k8s.io/v1
-kind: ClusterRole
-metadata:
-  labels:
-    app.kubernetes.io/instance: kubeblocks
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: kubeblocks
-  name: kubeblocks-opsrequest-editor-role
-rules:
-  change: 1
-  slice:
-  - {}
 

`
	exceptAdd = `--- kubeblocks-opsrequest-editor-role, ClusterRole (rbac.authorization.k8s.io) 0.5.1-fake
+++ kubeblocks-opsrequest-editor-role, ClusterRole (rbac.authorization.k8s.io) 0.5.2-fake
@@ -9,2 +9,3 @@
 rules:
+  apiGroups: apps
   change: 1

`
	exceptModify = `--- kubeblocks-opsrequest-editor-role, ClusterRole (rbac.authorization.k8s.io) 0.5.1-fake
+++ kubeblocks-opsrequest-editor-role, ClusterRole (rbac.authorization.k8s.io) 0.5.2-fake
@@ -9,3 +9,3 @@
 rules:
-  apiGroups: appsv2
+  apiGroups: apps
   change: 1

`

	crdContent = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: componentclassdefinitions.apps.kubeblocks.io
spec:
  group: apps.kubeblocks.io
  names:
    categories:
    - kubeblocks
    kind: ComponentClassDefinition
  scope: Cluster
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        description: ComponentClassDefinition is the Schema for the componentclassdefinitions
          API
        properties:
          apiVersion:
            type: string
    served: true
    storage: true
    subresources:
      status: {}`
)

var _ = Describe("helm diff", func() {
	var obj map[any]any
	var crdObj map[any]any
	var content string
	var release *helm.Release
	var out buffer.Buffer

	buildRelease := func(obj map[any]any) *helm.Release {
		var res helm.Release
		marshal, _ := yaml.Marshal(obj)
		manifest := `---
# Source: kubeblocks/templates/rbac/apps_backuppolicytemplate_editor_role.yaml
# permissions for end users to edit backuppolicytemplates.` + "\n" + string(marshal)
		res.Manifest = manifest
		return &res
	}

	exceptToMap := func(obj map[any]any, key string) map[any]any {
		Expect(obj).ShouldNot(BeNil())
		m, ok := obj[key].(map[any]any)
		Expect(ok).Should(BeTrue())
		return m
	}
	exceptToSlice := func(obj map[any]any, key string) []any {
		Expect(obj).ShouldNot(BeNil())
		m, ok := obj[key].([]any)
		Expect(ok).Should(BeTrue())
		return m
	}

	BeforeEach(func() {
		obj = map[any]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[any]any{
				"name": "kubeblocks-opsrequest-editor-role",
				"labels": map[any]any{
					"helm.sh/chart":                "kubeblocks-0.5.1",
					"app.kubernetes.io/name":       "kubeblocks",
					"app.kubernetes.io/instance":   "kubeblocks",
					"app.kubernetes.io/version":    "0.5.1",
					"app.kubernetes.io/managed-by": "Helm",
				},
			},
			"rules": map[any]any{
				"description": "should be delete",
				"change":      1,
				"slice": []any{
					map[any]any{
						"description": "should be delete too",
					},
				},
			},
		}
		release = buildRelease(obj)
		content = release.Manifest
	})

	It("test metadata String", func() {
		m := []metadata{
			{APIVersion: "rbac.authorization.k8s.io/v1",
				Kind: "ClusterRole",
				Metadata: struct {
					Name   string            `yaml:"name"`
					Labels map[string]string `yaml:"labels"`
				}{
					"kubeblocks-opsrequest-editor-role",
					map[string]string{},
				},
			}, {APIVersion: "v1",
				Kind: "Service",
				Metadata: struct {
					Name   string            `yaml:"name"`
					Labels map[string]string `yaml:"labels"`
				}{
					"kubeblocks",
					map[string]string{},
				},
			},
		}
		e := []string{
			resourceName,
			"kubeblocks, Service (v1)",
		}
		for i := range m {
			Expect(m[i].String()).Should(Equal(e[i]))
		}
	})

	It("test sortedKey", func() {
		manifest := map[string]*MappingResult{
			"b": nil,
			"c": nil,
			"a": nil,
		}
		Expect(sortedKeys(manifest)).Should(Equal([]string{"a", "b", "c"}))
	})

	It("test delete obj label", func() {
		testObj := obj
		metadata := exceptToMap(testObj, "metadata")
		labels := exceptToMap(metadata, "labels")
		Expect(labels["helm.sh/chart"]).ShouldNot(BeNil())
		deleteLabel(&testObj, "helm.sh/chart")
		Expect(labels["helm.sh/chart"]).Should(BeNil())
	})

	It("test delete obj field", func() {
		testObj := obj
		rules := exceptToMap(testObj, "rules")
		slice := exceptToSlice(rules, "slice")
		Expect(rules["description"]).ShouldNot(BeNil())
		Expect(slice[0]).ShouldNot(BeEmpty())
		deleteObjField(&testObj, "description")
		Expect(rules["description"]).Should(BeNil())
		Expect(slice[0]).Should(BeEmpty())
	})

	Context("test ParseContent", func() {

		It("test ParseContent", func() {
			parseContent, err := ParseContent(content)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(parseContent.Name).Should(Equal(resourceName))
			Expect(parseContent.Kind).Should(Equal("ClusterRole"))
			unusefulContent := `---
# Source: kubeblocks/templates/rbac/apps_backuppolicytemplate_editor_role.yaml
# permissions for end users to edit backuppolicytemplates.
`
			parseContent, err = ParseContent(unusefulContent)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(parseContent).Should(BeNil())
			errorContent := `---
			# Source: kubeblocks/tJ!@JDASD!bASD!@D!@
			# permissions for end users to edit backuppolicytemplates.
			ASDasdodh1*(!@#D!`
			parseContent, err = ParseContent(errorContent)
			Expect(err).Should(HaveOccurred())
			Expect(parseContent).Should(BeNil())
		})

		It("test black list", func() {
			blackListContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-chart-kubeblocks-values
data:
  values-kubeblocks-override.yaml: |-
    adminPassword: kubeblocks
    adminUser: admin`
			parseContent, err := ParseContent(blackListContent)
			Expect(err).Should(Succeed())
			Expect(parseContent).Should(BeNil())
		})

		It("test Parse CRD", func() {
			parseContent, err := ParseContent(crdContent)
			Expect(err).Should(Succeed())
			Expect(parseContent).ShouldNot(BeNil())
		})
	})

	It("test buildManifestMapByRelease", func() {
		releaseMap, err := buildManifestMapByRelease(release)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(releaseMap).Should(HaveKey(resourceName))
	})

	Context("test OutputDiff", func() {
		BeforeEach(func() {
			crdObj = map[any]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[any]any{
					"name": "kubeblocks-opsrequest-editor-role",
					"labels": map[any]any{
						"helm.sh/chart": "kubeblocks-0.5.1",
					},
				},
				"spec": map[any]any{
					"versions": []any{
						map[any]any{
							"schema": map[any]any{
								"openAPIV3Schema": map[any]any{
									"type": object,
									"properties": map[any]any{
										"apiVersion": map[any]any{"type": "string"},
										"kind":       map[any]any{"type": "string"},
										"metadata":   map[any]any{"type": object},
										"spec": map[any]any{
											"properties": map[any]any{
												"fake": map[any]any{"type": "string"},
											},
											"type": object,
										},
									},
								},
							},
						},
					},
				},
			}
		})
		It("test OutputDiff CRD", func() {
			crdRelease := buildRelease(crdObj)
			Expect(OutputDiff(crdRelease, nil, "0.5.1-fake", "", &out, false)).ShouldNot(HaveOccurred())
			Expect(out.String()).Should(Equal(`CUSTOMRESOURCEDEFINITION            MODE      
kubeblocks-opsrequest-editor-role   Removed   

`))
			out.Reset()
			Expect(OutputDiff(nil, crdRelease, "", "0.5.1-fake", &out, false)).Should(Succeed())
			Expect(out.String()).Should(Equal(`CUSTOMRESOURCEDEFINITION            MODE    
kubeblocks-opsrequest-editor-role   Added   

`))
		})

		It("test OutputDiff detail", func() {
			out.Reset()
			releaseA := release
			Expect(OutputDiff(releaseA, nil, "0.5.1-fake", "", &out, true)).Should(Succeed())
			Expect(out.String()).Should(Equal(exceptRemove))
			out.Reset()
			// add
			otherObj := obj
			rules := exceptToMap(otherObj, "rules")
			rules["apiGroups"] = "apps"
			releaseB := buildRelease(otherObj)
			Expect(OutputDiff(releaseA, releaseB, "0.5.1-fake", "0.5.2-fake", &out, true)).Should(Succeed())
			Expect(out.String()).Should(Equal(exceptAdd))
			// modify
			out.Reset()
			rules["apiGroups"] = "appsv2"
			releaseA = buildRelease(otherObj)
			Expect(OutputDiff(releaseA, releaseB, "0.5.1-fake", "0.5.2-fake", &out, true)).Should(Succeed())
			Expect(out.String()).Should(Equal(exceptModify))
		})
	})

	Context("test outputCRDDiff", func() {
		var apiA map[string]any
		var properties = map[any]any{}

		removeAPIAndAddAPI := func() map[string]any {
			var newProperties = map[any]any{}
			maps.Copy(newProperties, properties)
			delete(newProperties, "namespace")
			newProperties["newApi"] = map[any]any{TYPE: "boolean"}
			temp := map[string]any{
				"spec": map[any]any{
					REQUIRED:   []any{"name"},
					TYPE:       object,
					PROPERTIES: newProperties,
				},
			}
			return temp
		}

		modifyTheRequired := func() map[string]any {
			temp := map[string]any{
				"spec": map[any]any{
					TYPE:       object,
					PROPERTIES: properties,
				},
			}
			return temp
		}

		modifyTheField := func() map[string]any {
			var newProperties = map[any]any{}
			maps.Copy(newProperties, properties)
			newProperties["name"] = map[any]any{TYPE: "string", "maxLength": 63}
			temp := map[string]any{
				"spec": map[any]any{
					REQUIRED:   []any{"name"},
					TYPE:       object,
					PROPERTIES: newProperties,
				},
			}
			return temp
		}
		BeforeEach(func() {
			properties = map[any]any{
				"name":      map[any]any{TYPE: "string"},
				"namespace": map[any]any{TYPE: "string"},
				"defaultInstallValues": map[any]any{
					TYPE: array,
					"items": map[any]any{
						TYPE: object,
						PROPERTIES: map[any]any{
							"container1": map[any]any{TYPE: "string"},
							"container2": map[any]any{TYPE: "string"},
						},
					},
				},
			}
			apiA = map[string]any{
				"spec": map[any]any{
					REQUIRED:   []any{"name"},
					TYPE:       object,
					PROPERTIES: properties,
				},
			}
		})
		It("test Added and Removed", func() {
			out.Reset()
			apiB := removeAPIAndAddAPI()
			outputCRDDiff(apiA, apiB, "Fake CRD", &out)
			Expect(out.String()).Should(Equal(`Fake CRD
API              IS-REQUIRED   MODE      DETAILS             
spec.newApi      false         Added                         
spec.namespace   false         Removed   {"type":"string"}   

`))
		})
		It("test Modified", func() {
			out.Reset()
			apiB := modifyTheField()
			outputCRDDiff(apiA, apiB, "Fake CRD", &out)
			Expect(out.String()).Should(Equal(`Fake CRD
API         IS-REQUIRED   MODE       DETAILS                                                 
spec.name   true          Modified   {"type":"string"} -> {"maxLength":63,"type":"string"}   

`))
			out.Reset()
			apiB = modifyTheRequired()
			outputCRDDiff(apiA, apiB, "Fake CRD", &out)
			Expect(out.String()).Should(Equal(`Fake CRD
API         IS-REQUIRED     MODE       DETAILS   
spec.name   true -> false   Modified             

`))
		})

	})
})
