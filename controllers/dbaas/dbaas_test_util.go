/*
Copyright 2022.

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

package dbaas

import (
	"context"
	"os"
	"path"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

const (
	ISV_CLUSTER_SCOPE = "default"

	ISV_TEST_CD_PREFIX  = "test-clusterdefinition-"
	ISV_TEST_AV_PREFIX  = "test-appversion-"
	ISV_TEST_TPL_PREFIX = "test-cfgtpl-"
	TEST_CLUSTER_PREFIX = "test-cluster-"
)

type FakeTest struct {
	CdName     string
	AvName     string
	CfgTplName string
	Namespace  string

	// for yaml file
	CdYaml          string
	AvYaml          string
	CfgCMYaml       string
	CfgTemplateYaml string
}

type TestWrapper struct {
	testEnv FakeTest
	// clusterName string

	// test error
	err     error
	cli     client.Client
	ctx     context.Context
	testCtx testutil.TestContext

	// cr object
	cd  *dbaasv1alpha1.ClusterDefinition
	av  *dbaasv1alpha1.AppVersion
	tpl *dbaasv1alpha1.ConfigurationTemplate
	cm  *corev1.ConfigMap
}

func (w *TestWrapper) HasError() error {
	return w.err
}

func (w *TestWrapper) CreateCluster(name string) *dbaasv1alpha1.Cluster {
	clusterObj := &dbaasv1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: dbaasv1alpha1.APIVersion,
			Kind:       dbaasv1alpha1.ClusterKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: w.testEnv.Namespace,
		},
		Spec: dbaasv1alpha1.ClusterSpec{
			ClusterDefRef: w.testEnv.CdName,
			AppVersionRef: w.testEnv.AvName,
		},
	}

	w.createCrObject(clusterObj)
	return clusterObj
}

func (w *TestWrapper) createCrObject(obj client.Object) {
	if err := w.testCtx.CheckedCreateObj(w.ctx, obj); err != nil {
		w.err = err
	}
}

func (w *TestWrapper) updateAvComTplMeta(appVer *dbaasv1alpha1.AppVersion) {
	appVer.Spec.ClusterDefinitionRef = w.testEnv.CdName
	for _, component := range appVer.Spec.Components {
		if len(component.ConfigTemplateRefs) == 0 {
			continue
		}
		for i := 0; i < len(component.ConfigTemplateRefs); i++ {
			component.ConfigTemplateRefs[i].Name = w.testEnv.CfgTplName
		}
	}
}

func (w *TestWrapper) updateComTplMeta(cd *dbaasv1alpha1.ClusterDefinition) {
	// fix return value of xxx func is not checked (errcheck)
	ok, _ := configuration.HandleConfigTemplate(cd,
		func(templates []dbaasv1alpha1.ConfigTemplate) (bool, error) {
			return true, nil
		},
		func(component *dbaasv1alpha1.ClusterDefinitionComponent) error {
			for i := 0; i < len(component.ConfigTemplateRefs); i++ {
				component.ConfigTemplateRefs[i].Name = w.testEnv.CfgTplName
			}
			return nil
		})
	_ = ok
}

func (w *TestWrapper) DeleteAllCR() error {
	var (
		ctx       = w.ctx
		testCtx   = w.testCtx
		k8sClient = w.cli
		clusterNS = w.testEnv.Namespace
	)

	// step1: delete cluster cr
	if err := k8sClient.DeleteAllOf(ctx,
		&dbaasv1alpha1.Cluster{},
		client.InNamespace(clusterNS),
		client.HasLabels{testCtx.TestObjLabelKey}); err != nil {
		return err
	}

	// step2: delete appversion cr
	if err := k8sClient.DeleteAllOf(ctx,
		&dbaasv1alpha1.AppVersion{},
		client.HasLabels{testCtx.TestObjLabelKey}); err != nil {
		return err
	}

	// step3: delete clusterdefinition cr
	if err := k8sClient.DeleteAllOf(ctx,
		&dbaasv1alpha1.ClusterDefinition{},
		client.HasLabels{testCtx.TestObjLabelKey}); err != nil {
		return err
	}

	// step4: delete config templateion cr
	if err := k8sClient.DeleteAllOf(ctx,
		&dbaasv1alpha1.ConfigurationTemplate{},
		client.HasLabels{testCtx.TestObjLabelKey}); err != nil {
		return err
	}

	// step5: delete config cm cr
	if err := k8sClient.DeleteAllOf(ctx,
		&corev1.ConfigMap{},
		client.InNamespace(ISV_CLUSTER_SCOPE),
		client.HasLabels{
			testCtx.TestObjLabelKey,
			configuration.CMConfigurationTplNameLabelKey,
		}); err != nil {
		return err
	}

	// step6: delete pvc cr
	if err := k8sClient.DeleteAllOf(ctx,
		&corev1.PersistentVolumeClaim{},
		client.InNamespace(testCtx.DefaultNamespace),
		client.MatchingLabels{
			"app.kubernetes.io/name": "state.mysql-8-cluster-definition",
		}); err != nil {
		return err
	}

	return nil
}

func (w *TestWrapper) DeleteCluster(objKey client.ObjectKey) error {
	var (
		ctx       = w.ctx
		k8sClient = w.cli
	)

	f := &dbaasv1alpha1.Cluster{}
	if err := k8sClient.Get(ctx, objKey, f); err != nil {
		return client.IgnoreNotFound(err)
	}
	return k8sClient.Delete(ctx, f)
}

func (w *TestWrapper) WithCRName(name string) client.ObjectKey {
	return client.ObjectKey{
		Namespace: w.testEnv.Namespace,
		Name:      name,
	}
}

func GenRandomCDName() string {
	return ISV_TEST_CD_PREFIX + BuildRandomString()
}

func GenRandomAVName() string {
	return ISV_TEST_AV_PREFIX + BuildRandomString()
}

func GenRandomClusterName() string {
	return TEST_CLUSTER_PREFIX + BuildRandomString()
}

func GenRandomTplName() string {
	return ISV_TEST_TPL_PREFIX + BuildRandomString()
}

func BuildRandomString() string {
	const (
		RandomLength = 12
		NumDigits    = 2
		NumSymbols   = 0
	)

	randomStr, _ := password.Generate(RandomLength, NumDigits, NumSymbols, true, false)
	return randomStr
}

func CreateDBaasFromISV(testCtx testutil.TestContext, ctx context.Context, k8sClient client.Client, dataPath string, testInfo FakeTest, autoGenerate bool) *TestWrapper {
	if autoGenerate {
		testInfo.CdName = GenRandomCDName()
		testInfo.AvName = GenRandomAVName()
		testInfo.CfgTplName = GenRandomTplName()
		testInfo.Namespace = "default"
	}

	testWrapper := &TestWrapper{
		testEnv: testInfo,
		ctx:     ctx,
		testCtx: testCtx,
		cli:     k8sClient,
	}

	// create dbaas
	// createCdFromISV(&testWrapper, testInfo.CdName, testInfo.CdYaml)
	// createAvFromISV(&testWrapper, testInfo.AvName, testInfo.AvYaml)
	// createCmFromISV(&testWrapper, testInfo.CfgTplName, testInfo.CfgTplName)
	// createCfgTplFromISV(&testWrapper, testInfo.CfgTplName, testInfo.CfgTemplateYaml)

	createCRFromISVWithT(testWrapper, testInfo.CdName, path.Join(dataPath, testInfo.CdYaml), &dbaasv1alpha1.ClusterDefinition{})
	createCRFromISVWithT(testWrapper, testInfo.AvName, path.Join(dataPath, testInfo.AvYaml), &dbaasv1alpha1.AppVersion{})
	createCRFromISVWithT(testWrapper, testInfo.CfgTplName, path.Join(dataPath, testInfo.CfgCMYaml), &corev1.ConfigMap{})
	createCRFromISVWithT(testWrapper, testInfo.CfgTplName, path.Join(dataPath, testInfo.CfgTemplateYaml), &dbaasv1alpha1.ConfigurationTemplate{})

	return testWrapper
}

func createCRFromISVWithT(t *TestWrapper, tplName string, fileName string, crType client.Object) {
	if t.HasError() != nil {
		return
	}

	var (
		err   error
		crObj client.Object
	)

	switch obj := crType.(type) {
	case *dbaasv1alpha1.AppVersion:
		crObj, err = createCrFromFile(fileName, obj, t.updateAvComTplMeta)
		t.av = crObj.(*dbaasv1alpha1.AppVersion)
	case *dbaasv1alpha1.ClusterDefinition:
		crObj, err = createCrFromFile(fileName, obj, t.updateComTplMeta)
		t.cd = crObj.(*dbaasv1alpha1.ClusterDefinition)
	case *dbaasv1alpha1.ConfigurationTemplate:
		crObj, err = createCrFromFile(fileName, obj, func(tpl *dbaasv1alpha1.ConfigurationTemplate) {
			tpl.Spec.TplRef = t.testEnv.CfgTplName
		})
		t.tpl = crObj.(*dbaasv1alpha1.ConfigurationTemplate)
	// case *dbaasv1alpha1.Cluster:
	//	crObj, err = createCrFromFile(fileName, obj)
	case *corev1.ConfigMap:
		crObj, err = createCrFromFile(fileName, obj)
		t.cm = crObj.(*corev1.ConfigMap)
	}

	if err != nil {
		t.err = err
		return
	}

	crObj.SetName(tplName)
	crObj.SetNamespace(ISV_CLUSTER_SCOPE)
	t.createCrObject(crObj)
}

func createCrFromFile[T corev1.ConfigMap | dbaasv1alpha1.ClusterDefinition | dbaasv1alpha1.Cluster | dbaasv1alpha1.AppVersion | dbaasv1alpha1.ConfigurationTemplate](fileName string, param *T, ops ...func(obj *T)) (*T, error) {
	_ = param

	cdYaml, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	obj := new(T)
	if err := yaml.Unmarshal(cdYaml, obj); err != nil {
		return nil, err
	}

	for _, op := range ops {
		op(obj)
	}

	return obj, nil
}

func CreateCluster(test *TestWrapper, clusterName string) *dbaasv1alpha1.Cluster {
	return test.CreateCluster(clusterName)
}

func DeleteCluster(test *TestWrapper, cluster *dbaasv1alpha1.Cluster) error {
	return test.DeleteCluster(client.ObjectKeyFromObject(cluster))
}

func ValidateISVCR[T any, OBJECT interface{ client.Object }](test *TestWrapper, obj client.Object, defaultValue T, handle func(obj OBJECT) T) (T, error) {
	var (
		ctx       = test.ctx
		k8sClient = test.cli
		objKey    client.ObjectKey
	)

	switch obj.(type) {
	case *dbaasv1alpha1.AppVersion:
		objKey = client.ObjectKey{
			Namespace: ISV_CLUSTER_SCOPE,
			Name:      test.testEnv.AvName,
		}
	case *dbaasv1alpha1.ClusterDefinition:
		objKey = client.ObjectKey{
			Namespace: ISV_CLUSTER_SCOPE,
			Name:      test.testEnv.CdName,
		}
	case *dbaasv1alpha1.ConfigurationTemplate:
		objKey = client.ObjectKey{
			Namespace: ISV_CLUSTER_SCOPE,
			Name:      test.testEnv.CfgTplName,
		}
	case *corev1.ConfigMap:
		objKey = client.ObjectKey{
			Namespace: ISV_CLUSTER_SCOPE,
			Name:      test.testEnv.CfgTplName,
		}
	default:
		return defaultValue, cfgcore.MakeError("not support cr type.")
	}

	if err := k8sClient.Get(ctx, objKey, obj); err != nil {
		return defaultValue, err
	}

	return handle(obj.(OBJECT)), nil
}

func ValidateCR[T any, T2 interface{ client.Object }](test *TestWrapper, obj T2, objKey client.ObjectKey, defaultValue T, handle func(obj T2) T) (T, error) {
	var (
		ctx       = test.ctx
		k8sClient = test.cli
	)

	// obj := *new(T2)
	if err := k8sClient.Get(ctx, objKey, obj); err != nil {
		return defaultValue, err
	}

	return handle(obj), nil
}
