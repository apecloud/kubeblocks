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

package configuration

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/onsi/gomega"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	mockobject "github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	"github.com/apecloud/kubeblocks/test/testdata"
)

type FakeTest struct {
	// for yaml file
	CDYaml      string
	CVYaml      string
	CfgCMYaml   string
	CfgCCYaml   string
	StsYaml     string
	ClusterYaml string

	TestDataPath    string
	ComponentName   string
	CDComponentType string

	DisableConfigTpl        bool
	DisableConfigConstraint bool
}

type TestWrapper struct {
	namer   mockobject.ResourceNamer
	testEnv FakeTest

	cli     client.Client
	ctx     context.Context
	testCtx testutil.TestContext

	// cr object
	CD    *dbaasv1alpha1.ClusterDefinition
	CV    *dbaasv1alpha1.ClusterVersion
	TplCM *corev1.ConfigMap
	CC    *dbaasv1alpha1.ConfigConstraint
	STS   *appv1.StatefulSet
	CfgCM *corev1.ConfigMap

	stsName    string
	cfgCMName  string
	allObjects []client.Object
}

const (
	TestComponentName       = "wesql"
	TestCDComponentTypeName = "replicasets"
)

func (w *TestWrapper) DeleteAllObjects() {
	for _, obj := range w.allObjects {
		if err := w.testCtx.Cli.Delete(w.testCtx.Ctx, obj); err != nil {
			gomega.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
		}
	}

	var (
		inNS = client.InNamespace(w.namer.NS)
		ml   = client.HasLabels{w.testCtx.TestObjLabelKey}
	)

	if w.CV != nil {
		testdbaas.ClearResources(&w.testCtx, intctrlutil.ClusterVersionSignature, inNS, ml)
	}
	if w.CD != nil {
		testdbaas.ClearResources(&w.testCtx, intctrlutil.ClusterDefinitionSignature, inNS, ml)
	}
	if w.CC != nil {
		testdbaas.ClearResources(&w.testCtx, intctrlutil.ConfigConstraintSignature, inNS, ml)
	}
	if w.TplCM != nil {
		testdbaas.ClearResources(&w.testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
	}
}

func (w *TestWrapper) contains(fileName string) bool {
	return w.testEnv.CDYaml == fileName ||
		w.testEnv.CfgCCYaml == fileName ||
		w.testEnv.CfgCMYaml == fileName ||
		w.testEnv.StsYaml == fileName ||
		w.testEnv.CVYaml == fileName ||
		w.testEnv.ClusterYaml == fileName
}

func (w *TestWrapper) GetNamer() mockobject.ResourceNamer {
	return w.namer
}

func (w *TestWrapper) setMockObject(obj client.Object) {
	switch obj.GetName() {
	case w.namer.CDName:
		w.CD = obj.(*dbaasv1alpha1.ClusterDefinition)
	case w.namer.CCName:
		w.CC = obj.(*dbaasv1alpha1.ConfigConstraint)
	case w.namer.CVName:
		w.CV = obj.(*dbaasv1alpha1.ClusterVersion)
	case w.namer.TPLName:
		w.TplCM = obj.(*corev1.ConfigMap)
	case w.cfgCMName:
		w.CfgCM = obj.(*corev1.ConfigMap)
	case w.stsName:
		w.STS = obj.(*appv1.StatefulSet)
	}
	w.allObjects = append(w.allObjects, obj)
}

func (w *TestWrapper) DeleteTpl() error {
	if err := w.testCtx.Cli.Delete(w.testCtx.Ctx, w.CC); err != nil {
		return err
	}
	return w.testCtx.Cli.Delete(w.testCtx.Ctx, w.TplCM)
}

func (w *TestWrapper) DeleteCV() error {
	return w.testCtx.Cli.Delete(w.testCtx.Ctx, w.CV)
}

func (w *TestWrapper) DeleteCD() error {
	return w.testCtx.Cli.Delete(w.testCtx.Ctx, w.CD)
}

func NewFakeK8sObjectFromFile[T intctrlutil.Object, PT intctrlutil.PObject[T]](w *TestWrapper, yamlFile string, pobj PT, options ...testdata.ResourceOptions) {
	pt := testdbaas.CreateCustomizedObj(&w.testCtx, filepath.Join(w.testEnv.TestDataPath, yamlFile), pobj, func(t PT) {
		for _, option := range options {
			option(pobj)
		}
	})
	w.setMockObject(pt)
}

func NewFakeDBaasCRsFromProvider(testCtx testutil.TestContext, ctx context.Context, mockInfo FakeTest) *TestWrapper {
	var (
		cdComponentTypeName = mockInfo.CDComponentType
		componentName       = mockInfo.ComponentName
	)

	randomNamer := mockobject.CreateRandomResourceNamer(testCtx.DefaultNamespace)
	testWrapper := &TestWrapper{
		namer:      randomNamer,
		testEnv:    mockInfo,
		ctx:        ctx,
		testCtx:    testCtx,
		cli:        testCtx.Cli,
		stsName:    fmt.Sprintf("%s-%s", randomNamer.ClusterName, componentName),
		cfgCMName:  cfgcore.GetComponentCfgName(randomNamer.ClusterName, componentName, randomNamer.VolumeName),
		allObjects: make([]client.Object, 0),
	}

	resourceObjectsHelper := mockobject.NewFakeResourceObjectHelper(mockInfo.TestDataPath,
		mockobject.WithResourceKind(types.ConfigConstraintGVR(), types.KindConfigConstraint, testdata.WithName(randomNamer.CCName)),
		mockobject.WithResourceKind(types.CMGVR(), types.KindCM, testdata.WithNamespacedName(randomNamer.TPLName, randomNamer.NS)),
		mockobject.WithResourceKind(types.ClusterVersionGVR(), types.KindClusterVersion, testdata.WithName(randomNamer.CVName), testdata.WithClusterDef(randomNamer.CDName)),
		mockobject.WithResourceKind(types.ClusterDefGVR(), types.KindClusterDef,
			testdata.WithName(randomNamer.CDName),
			testdata.WithConfigTemplate(mockobject.GenerateConfigTemplate(randomNamer), testdata.ComponentTypeSelector(dbaasv1alpha1.Stateful)),
			testdata.WithUpdateComponent(testdata.ComponentTypeSelector(dbaasv1alpha1.Stateful),
				func(component *dbaasv1alpha1.ClusterDefinitionComponent) {
					component.TypeName = cdComponentTypeName
					if mockInfo.DisableConfigTpl {
						component.ConfigSpec = nil
						return
					}
					if component.ConfigSpec == nil && len(component.ConfigSpec.ConfigTemplateRefs) == 0 {
						return
					}
					tpl := &component.ConfigSpec.ConfigTemplateRefs[0]
					if mockInfo.DisableConfigConstraint {
						tpl.ConfigConstraintRef = ""
					}
				})),
		// mock config cm
		mockobject.WithCustomResource(types.CMGVR(), mockobject.NewFakeConfigCMResource(randomNamer, componentName, randomNamer.VolumeName,
			testdata.WithCMData(func() map[string]string {
				cmYaml := mockInfo.CfgCMYaml
				if len(cmYaml) == 0 {
					return make(map[string]string)
				}
				cm, _ := testdata.GetResourceFromTestData[corev1.ConfigMap](filepath.Join(mockInfo.TestDataPath, cmYaml))
				return cm.Data
			}),
			testdata.WithLabels(
				intctrlutil.AppNameLabelKey, randomNamer.ClusterName,
				intctrlutil.AppInstanceLabelKey, randomNamer.ClusterName,
				intctrlutil.AppComponentLabelKey, componentName,
				cfgcore.CMConfigurationTplNameLabelKey, randomNamer.TPLName,
				cfgcore.CMConfigurationConstraintsNameLabelKey, randomNamer.CCName,
				cfgcore.CMConfigurationISVTplLabelKey, randomNamer.TPLName,
				cfgcore.CMConfigurationTypeLabelKey, cfgcore.ConfigInstanceType,
			))),
		// mock cluster
		mockobject.WithResourceKind(types.ClusterGVR(), types.KindCluster,
			testdata.WithNamespacedName(randomNamer.ClusterName, randomNamer.NS),
			testdata.WithClusterDef(randomNamer.CDName),
			testdata.WithClusterVersion(randomNamer.CVName),
			testdata.WithClusterComponent(testdata.ComponentIndexSelector(0),
				testdata.WithComponentTypeName(mockInfo.ComponentName, mockInfo.CDComponentType)),
			testdata.WithLabels(
				intctrlutil.AppNameLabelKey, randomNamer.ClusterName,
				intctrlutil.AppInstanceLabelKey, randomNamer.ClusterName,
				intctrlutil.AppComponentLabelKey, componentName,
			)),
		// mock sts
		mockobject.WithResourceKind(types.STSGVR(), types.KindSTS,
			testdata.WithNamespacedName(testWrapper.stsName, randomNamer.NS),
			testdata.WithPodTemplate(
				testdata.WithConfigmapVolume(testWrapper.cfgCMName, randomNamer.VolumeName),
				testdata.WithPodVolumeMount(testdata.WithContainerIndexSelector(0), func(container *corev1.Container) {
					container.VolumeMounts = []corev1.VolumeMount{{
						Name:      randomNamer.VolumeName,
						MountPath: "/mnt/config_for_test",
					}}
				}),
			),
			testdata.WithLabels(
				intctrlutil.AppNameLabelKey, randomNamer.ClusterName,
				intctrlutil.AppInstanceLabelKey, randomNamer.ClusterName,
				intctrlutil.AppComponentLabelKey, componentName,
				cfgcore.GenerateTPLUniqLabelKeyWithConfig(randomNamer.TPLName), testWrapper.cfgCMName,
			),
		),
		mockobject.WithResourceFilter(func(fileName string) bool {
			return testWrapper.contains(fileName)
		}),
	)

	for _, obj := range resourceObjectsHelper.CreateObjects() {
		k8sObj, ok := obj.(client.Object)
		testWrapper.setMockObject(k8sObj)
		gomega.Expect(ok).Should(gomega.BeTrue())
		gomega.Expect(testCtx.CreateObj(testWrapper.ctx, k8sObj)).Should(gomega.Succeed())
	}
	return testWrapper
}
