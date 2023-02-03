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
type filterFilter = func(string) bool

type FakeKubeObjectHelper struct {
	// resource base path
	basePath string

	filter filterFilter

	// filter
	filterResources map[schema.GroupVersionResource]string
	// custom options
	resourceOptions map[string][]testdata.ResourceOptions
	// directory create cr
	customCreateResources map[schema.GroupVersionResource][]createResourceObject
}

type ResourceNamer struct {
	ns          string
	cdName      string
	cvName      string
	tplName     string
	ccName      string
	clusterName string
}

type helperOption func(helper *FakeKubeObjectHelper)

func CreateRandomResourceNamer(ns string) ResourceNamer {
	return ResourceNamer{
		ns:          ns,
		cdName:      "clusterdef-test-" + testdata.GenRandomString(),
		cvName:      "clusterversion-test-" + testdata.GenRandomString(),
		tplName:     "tpl-test-" + testdata.GenRandomString(),
		ccName:      "configconstraint-test-" + testdata.GenRandomString(),
		clusterName: "cluster-test-" + testdata.GenRandomString(),
	}
}

func GenerateConfigTemplate(namer ResourceNamer, volumeName string) []dbaasv1alpha1.ConfigTemplate {
	return []dbaasv1alpha1.ConfigTemplate{
		{
			Namespace:           namer.ns,
			Name:                namer.tplName,
			ConfigTplRef:        namer.tplName,
			ConfigConstraintRef: namer.ccName,
			VolumeName:          volumeName,
		},
	}
}

func WithResourceKind(resource schema.GroupVersionResource, kind string, options ...testdata.ResourceOptions) helperOption {
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

func WithResourceFilter(filter filterFilter) helperOption {
	return func(helper *FakeKubeObjectHelper) {
		helper.filter = filter
	}
}

func WithCustomResource(resource schema.GroupVersionResource, creator createResourceObject) helperOption {
	return func(helper *FakeKubeObjectHelper) {
		if helper.customCreateResources == nil {
			helper.customCreateResources = make(map[schema.GroupVersionResource][]createResourceObject)
		}
		creatorList := helper.customCreateResources[resource]
		creatorList = append(creatorList, creator)
		helper.customCreateResources[resource] = creatorList
	}
}

func newFakeConfigCMResource(namer ResourceNamer, componentName, volumeName string, dataMap map[string]string) createResourceObject {
	return func() runtime.Object {
		return &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: types.VersionV1,
				Kind:       types.KindCM,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cfgcore.GetComponentCfgName(namer.clusterName, componentName, volumeName),
				Namespace: namer.ns,
			},
			Data: dataMap,
		}
	}
}

func newFakeClusterResource(namer ResourceNamer, componentName, componentType string) createResourceObject {
	return func() runtime.Object {
		return &dbaasv1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: dbaasv1alpha1.APIVersion,
				Kind:       dbaasv1alpha1.ClusterKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      namer.clusterName,
				Namespace: namer.ns,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef:     namer.cdName,
				ClusterVersionRef: namer.cvName,
				Components: []dbaasv1alpha1.ClusterComponent{{
					Name: componentName,
					Type: componentType,
				}},
			},
		}
	}
}

func NewFakeResourceObjectHelper(basePath string, options ...helperOption) FakeKubeObjectHelper {
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
	resourceList, err := testdata.ScanDirectoryPath(helper.basePath)
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
