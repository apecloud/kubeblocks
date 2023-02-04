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

package cluster

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/test/testdata"
)

type createResourceObject = func() runtime.Object

type FakeKubeObjectHelper struct {
	// resource base path
	basePath string

	filter testdata.FilterOptions

	// filter
	filterResources map[schema.GroupVersionResource]string
	// custom options
	resourceOptions map[string][]testdata.ResourceOptions
	// directory create cr
	customCreateResources map[schema.GroupVersionResource][]createResourceObject
}

type ResourceNamer struct {
	NS          string
	CDName      string
	CVName      string
	TPLName     string
	CCName      string
	ClusterName string
	VolumeName  string
}

type ObjectMockHelperOption func(helper *FakeKubeObjectHelper)

func CreateRandomResourceNamer(ns string) ResourceNamer {
	return ResourceNamer{
		NS:          ns,
		VolumeName:  "config",
		CDName:      "clusterdef-test-" + testdata.GenRandomString(),
		CVName:      "clusterversion-test-" + testdata.GenRandomString(),
		TPLName:     "tpl-test-" + testdata.GenRandomString(),
		CCName:      "configconstraint-test-" + testdata.GenRandomString(),
		ClusterName: "cluster-test-" + testdata.GenRandomString(),
	}
}

func GenerateConfigTemplate(namer ResourceNamer) []dbaasv1alpha1.ConfigTemplate {
	return []dbaasv1alpha1.ConfigTemplate{
		{
			Namespace:           namer.NS,
			Name:                namer.TPLName,
			ConfigTplRef:        namer.TPLName,
			ConfigConstraintRef: namer.CCName,
			VolumeName:          namer.VolumeName,
		},
	}
}

func WithResourceKind(resource schema.GroupVersionResource, kind string, options ...testdata.ResourceOptions) ObjectMockHelperOption {
	return func(helper *FakeKubeObjectHelper) {
		if helper.filterResources == nil {
			helper.filterResources = make(map[schema.GroupVersionResource]string, 0)
		}
		if len(options) != 0 && helper.resourceOptions == nil {
			helper.resourceOptions = map[string][]testdata.ResourceOptions{}
		}
		if len(options) != 0 {
			helper.resourceOptions[kind] = options
		}
		helper.filterResources[resource] = kind
	}
}

func WithResourceFilter(filter testdata.FilterOptions) ObjectMockHelperOption {
	return func(helper *FakeKubeObjectHelper) {
		helper.filter = filter
	}
}

func WithCustomResource(resource schema.GroupVersionResource, creator createResourceObject) ObjectMockHelperOption {
	return func(helper *FakeKubeObjectHelper) {
		if helper.customCreateResources == nil {
			helper.customCreateResources = make(map[schema.GroupVersionResource][]createResourceObject)
		}
		creatorList := helper.customCreateResources[resource]
		creatorList = append(creatorList, creator)
		helper.customCreateResources[resource] = creatorList
	}
}

func NewFakeConfigCMResource(namer ResourceNamer, componentName, volumeName string, options ...testdata.ResourceOptions) createResourceObject {
	return func() runtime.Object {
		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: types.VersionV1,
				Kind:       types.KindCM,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cfgcore.GetComponentCfgName(namer.ClusterName, componentName, volumeName),
				Namespace: namer.NS,
			},
		}
		for _, option := range options {
			option(cm)
		}
		return cm
	}
}

func NewFakeClusterResource(namer ResourceNamer, componentName, componentType string, options ...testdata.ResourceOptions) createResourceObject {
	return func() runtime.Object {
		cluster := &dbaasv1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: dbaasv1alpha1.APIVersion,
				Kind:       dbaasv1alpha1.ClusterKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      namer.ClusterName,
				Namespace: namer.NS,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef:     namer.CDName,
				ClusterVersionRef: namer.CVName,
				Components: []dbaasv1alpha1.ClusterComponent{{
					Name: componentName,
					Type: componentType,
				}},
			},
		}
		for _, option := range options {
			option(cluster)
		}
		return cluster
	}
}

func NewFakeResourceObjectHelper(basePath string, options ...ObjectMockHelperOption) FakeKubeObjectHelper {
	fakeHelper := FakeKubeObjectHelper{
		basePath: basePath,
	}
	for _, option := range options {
		option(&fakeHelper)
	}
	return fakeHelper
}

func (helper *FakeKubeObjectHelper) pass(meta metav1.TypeMeta) bool {
	if helper.filterResources == nil {
		return false
	}
	for _, kind := range helper.filterResources {
		if meta.Kind == kind {
			return false
		}
	}
	return true
}

func (helper *FakeKubeObjectHelper) options(meta metav1.TypeMeta) []testdata.ResourceOptions {
	if helper.resourceOptions == nil {
		return nil
	}
	return helper.resourceOptions[meta.Kind]
}

func (helper *FakeKubeObjectHelper) CreateObjects() []runtime.Object {
	resourceList, err := testdata.ScanDirectoryPath(helper.basePath, helper.filter)
	requireSucceed(err)

	var (
		k8sObj    runtime.Object
		yamlBytes []byte
		meta      metav1.TypeMeta
		allObjs   = make([]runtime.Object, 0)
	)

	// direct create cr
	for _, creatorList := range helper.customCreateResources {
		for _, creator := range creatorList {
			allObjs = append(allObjs, creator())
		}
	}

	// create cr from yaml
	for _, resourceFile := range resourceList {
		yamlBytes, err = testdata.GetTestDataFileContent(resourceFile)
		requireSucceed(err)
		meta, err = testdata.GetResourceMeta(yamlBytes)
		requireSucceed(err)
		if helper.pass(meta) {
			continue
		}

		options := helper.options(meta)
		switch meta.Kind {
		case types.KindClusterDef:
			k8sObj, err = testdata.GetResourceFromContext[dbaasv1alpha1.ClusterDefinition](yamlBytes, options...)
		case types.KindClusterVersion:
			k8sObj, err = testdata.GetResourceFromContext[dbaasv1alpha1.ClusterVersion](yamlBytes, options...)
		case types.KindCM:
			k8sObj, err = testdata.GetResourceFromContext[corev1.ConfigMap](yamlBytes, options...)
		case types.KindConfigConstraint:
			k8sObj, err = testdata.GetResourceFromContext[dbaasv1alpha1.ConfigConstraint](yamlBytes, options...)
		case types.KindOps:
			k8sObj, err = testdata.GetResourceFromContext[dbaasv1alpha1.OpsRequest](yamlBytes, options...)
		case types.KindCluster:
			k8sObj, err = testdata.GetResourceFromContext[dbaasv1alpha1.Cluster](yamlBytes, options...)
		case types.KindSTS:
			k8sObj, err = testdata.GetResourceFromContext[appsv1.StatefulSet](yamlBytes, options...)
		default:
			continue
		}
		requireSucceed(err)
		allObjs = append(allObjs, k8sObj)
	}
	return allObjs
}

func requireSucceed(err error) {
	if err != nil {
		panic(err)
	}
}

func NewFakeOperationsOptions(ns, cName string, opsType dbaasv1alpha1.OpsType, objs ...runtime.Object) (*cmdtesting.TestFactory, *OperationsOptions) {
	streams, _, _, _ := genericclioptions.NewTestIOStreams()
	tf := cmdtesting.NewTestFactory().WithNamespace(ns)
	o := &OperationsOptions{
		BaseOptions: create.BaseOptions{
			IOStreams: streams,
			Name:      cName,
			Namespace: ns,
		},
		TTLSecondsAfterSucceed: 30,
		OpsType:                opsType,
	}

	err := dbaasv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	// TODO using GroupVersionResource of FakeKubeObjectHelper
	listMapping := map[schema.GroupVersionResource]string{
		types.ClusterDefGVR():       types.KindClusterDef + "List",
		types.ClusterVersionGVR():   types.KindClusterVersion + "List",
		types.ClusterGVR():          types.KindCluster + "List",
		types.ConfigConstraintGVR(): types.KindConfigConstraint + "List",
		types.BackupGVR():           types.KindBackup + "List",
		types.RestoreJobGVR():       types.KindRestoreJob + "List",
		types.OpsGVR():              types.KindOps + "List",
	}
	o.Client = dynamicfakeclient.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, listMapping, objs...)
	return tf, o
}
