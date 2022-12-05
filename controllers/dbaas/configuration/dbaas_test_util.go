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

package configuration

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

const (
	ISVClusterScope = "default"

	ISVTestCdPrefix   = "test-clusterdefinition-"
	ISVTestAvPrefix   = "test-appversion-"
	ISVTestTplPrefix  = "test-cfgtpl-"
	TestClusterPrefix = "test-cluster-"
)

type FakeTest struct {
	CdName     string
	AvName     string
	CfgTplName string
	Namespace  string
	MockSts    bool

	// for yaml file
	CdYaml          string
	AvYaml          string
	CfgCMYaml       string
	CfgTemplateYaml string
	StsYaml         string
}

type ISVResource interface {
	corev1.ConfigMap | appsv1.StatefulSet | dbaasv1alpha1.ClusterDefinition | dbaasv1alpha1.AppVersion | dbaasv1alpha1.ConfigurationTemplate
}

type K8sResource interface {
	client.Object
}

type TestWrapper struct {
	testEnv      FakeTest
	testRootPath string
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

func (w *TestWrapper) TplName() string {
	return w.testEnv.CfgTplName
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
			ClusterDefRef:     w.testEnv.CdName,
			AppVersionRef:     w.testEnv.AvName,
			TerminationPolicy: dbaasv1alpha1.Delete,
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
	ok, _ := HandleConfigTemplate(cd,
		func(templates []dbaasv1alpha1.ConfigTemplate) (bool, error) {
			return true, nil
		},
		func(component *dbaasv1alpha1.ClusterDefinitionComponent) error {
			configSpec := component.ConfigSpec
			if configSpec == nil {
				return nil
			}
			for i := 0; i < len(configSpec.ConfigTemplateRefs); i++ {
				configSpec.ConfigTemplateRefs[i].Name = w.testEnv.CfgTplName
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
		client.InNamespace(clusterNS),
		client.HasLabels{testCtx.TestObjLabelKey}); err != nil {
		return err
	}

	// step5: delete config cm cr
	if err := k8sClient.DeleteAllOf(ctx,
		&corev1.ConfigMap{},
		client.InNamespace(ISVClusterScope),
		client.HasLabels{
			testCtx.TestObjLabelKey,
			CMConfigurationTplNameLabelKey,
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

func (w *TestWrapper) DeleteCD() error {
	return w.cli.Delete(w.ctx, w.cd)
}

func (w *TestWrapper) DeleteAV() error {
	return w.cli.Delete(w.ctx, w.av)
}

func (w *TestWrapper) DeleteTpl() error {
	var (
		ctx       = w.ctx
		k8sClient = w.cli
	)
	if err := k8sClient.Delete(ctx, w.tpl); err != nil {
		return err
	}
	return k8sClient.Delete(ctx, w.cm)
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

func (w *TestWrapper) CreateCfgOnCluster(cfgFile string, cluster *dbaasv1alpha1.Cluster, componentName string) (*corev1.ConfigMap, error) {
	insCfgCMName := cfgcore.GetComponentCfgName(cluster.Name, componentName, w.testEnv.CfgTplName)
	if w.testEnv.MockSts {
		if err := w.createStsFromFile(cluster, componentName, insCfgCMName); err != nil {
			return nil, err
		}
	}

	cmObj := &corev1.ConfigMap{}
	cmObj, err := createISVCrFromFile(path.Join(w.testRootPath, cfgFile), cmObj, func(cm *corev1.ConfigMap) {
		if cm.Labels == nil {
			cm.Labels = make(map[string]string)
		}
		cm.Labels[intctrlutil.AppNameLabelKey] = cluster.Name
		cm.Labels[intctrlutil.AppInstanceLabelKey] = cluster.Name
		cm.Labels[intctrlutil.AppComponentLabelKey] = componentName
		cm.Labels[CMConfigurationTplNameLabelKey] = w.testEnv.CfgTplName
		cm.Labels[CMInsConfigurationLabelKey] = "true"
	})
	if err != nil {
		return nil, err
	}

	cmObj.Name = insCfgCMName
	cmObj.Namespace = w.testEnv.Namespace

	w.createCrObject(cmObj)
	return cmObj, nil
}

func (w *TestWrapper) WithCRName(name string) client.ObjectKey {
	return client.ObjectKey{
		Namespace: w.testEnv.Namespace,
		Name:      name,
	}
}

func (w *TestWrapper) updateCrObject(obj client.Object, patch client.Patch) error {
	var (
		ctx       = w.ctx
		k8sClient = w.cli
	)

	return k8sClient.Patch(ctx, obj, patch)
}

func (w *TestWrapper) createStsFromFile(cluster *dbaasv1alpha1.Cluster, componentName string, cmName string) error {
	if w.testEnv.StsYaml == "" {
		return cfgcore.MakeError("require statefuleset cr yaml.")
	}

	sts := &appsv1.StatefulSet{}
	cmObj, err := createISVCrFromFile(path.Join(w.testRootPath, w.testEnv.StsYaml), sts, func(cm *appsv1.StatefulSet) {
		if cm.Labels == nil {
			cm.Labels = make(map[string]string)
		}
		cm.Labels[intctrlutil.AppNameLabelKey] = cluster.Name
		cm.Labels[intctrlutil.AppInstanceLabelKey] = cluster.Name
		cm.Labels[intctrlutil.AppComponentLabelKey] = componentName
		cm.Labels[cfgcore.GenerateUniqLabelKeyWithConfig(w.testEnv.CfgTplName)] = w.testEnv.CfgTplName

		sts.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "for_test",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
					},
				},
			},
		}

	})
	if err != nil {
		return err
	}

	cmObj.Name = fmt.Sprintf("%s-%s", cluster.Name, componentName)
	cmObj.Namespace = w.testEnv.Namespace

	w.createCrObject(cmObj)
	return nil
}

func GenRandomCDName() string {
	return ISVTestCdPrefix + BuildRandomString()
}

func GenRandomAVName() string {
	return ISVTestAvPrefix + BuildRandomString()
}

func GenRandomClusterName() string {
	return TestClusterPrefix + BuildRandomString()
}

func GenRandomTplName() string {
	return ISVTestTplPrefix + BuildRandomString()
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
		testInfo.Namespace = testCtx.DefaultNamespace
	}

	testWrapper := &TestWrapper{
		testEnv:      testInfo,
		ctx:          ctx,
		testCtx:      testCtx,
		cli:          k8sClient,
		testRootPath: dataPath,
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
		crObj, err = createISVCrFromFile(fileName, obj, t.updateAvComTplMeta)
		t.av = crObj.(*dbaasv1alpha1.AppVersion)
	case *dbaasv1alpha1.ClusterDefinition:
		crObj, err = createISVCrFromFile(fileName, obj, t.updateComTplMeta)
		t.cd = crObj.(*dbaasv1alpha1.ClusterDefinition)
	case *dbaasv1alpha1.ConfigurationTemplate:
		crObj, err = createISVCrFromFile(fileName, obj, func(tpl *dbaasv1alpha1.ConfigurationTemplate) {
			tpl.Spec.TplRef = t.testEnv.CfgTplName
		})
		t.tpl = crObj.(*dbaasv1alpha1.ConfigurationTemplate)
	// case *dbaasv1alpha1.Cluster:
	//	crObj, err = createISVCrFromFile(fileName, obj)
	case *corev1.ConfigMap:
		crObj, err = createISVCrFromFile(fileName, obj)
		t.cm = crObj.(*corev1.ConfigMap)
	}

	if err != nil {
		t.err = err
		return
	}

	crObj.SetName(tplName)
	crObj.SetNamespace(ISVClusterScope)
	t.createCrObject(crObj)
}

func createISVCrFromFile[T ISVResource](fileName string, param *T, ops ...func(obj *T)) (*T, error) {
	cdYaml, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	obj := param
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

func ValidateISVCR[T any, OBJECT K8sResource](test *TestWrapper, obj client.Object, handle func(obj OBJECT) T) (T, error) {
	var (
		ctx          = test.ctx
		k8sClient    = test.cli
		objKey       client.ObjectKey
		defaultValue T
	)

	switch obj.(type) {
	case *dbaasv1alpha1.AppVersion:
		objKey = client.ObjectKey{
			Namespace: ISVClusterScope,
			Name:      test.testEnv.AvName,
		}
	case *dbaasv1alpha1.ClusterDefinition:
		objKey = client.ObjectKey{
			Namespace: ISVClusterScope,
			Name:      test.testEnv.CdName,
		}
	case *dbaasv1alpha1.ConfigurationTemplate:
		objKey = client.ObjectKey{
			Namespace: ISVClusterScope,
			Name:      test.testEnv.CfgTplName,
		}
	case *corev1.ConfigMap:
		objKey = client.ObjectKey{
			Namespace: ISVClusterScope,
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

func ValidateCR[T any, T2 K8sResource](test *TestWrapper, obj T2, objKey client.ObjectKey, handle func(obj T2) T) (T, error) {
	var (
		ctx          = test.ctx
		k8sClient    = test.cli
		defaultValue T
	)

	// obj := *new(T2)
	if err := k8sClient.Get(ctx, objKey, obj); err != nil {
		return defaultValue, err
	}

	return handle(obj), nil
}

func createCrFromFile[T K8sResource](fileName string) (T, error) {
	obj := new(T)

	cdYaml, err := os.ReadFile(fileName)
	if err != nil {
		return *obj, err
	}

	if err := yaml.Unmarshal(cdYaml, obj); err != nil {
		return *obj, err
	}

	return *obj, nil
}

func UpdateCR[T any, T2 K8sResource](test *TestWrapper, obj T2, objKey client.ObjectKey, fileName string, op func(cm T2, newCm T2) (client.Patch, error)) error {
	var (
		ctx       = test.ctx
		k8sClient = test.cli
	)

	crObj, err := createCrFromFile[T2](path.Join(test.testRootPath, fileName))
	if err != nil {
		return err
	}

	if err := k8sClient.Get(ctx, objKey, obj); err != nil {
		return err
	}

	if patch, err := op(obj, crObj); err != nil {
		return err
	} else {
		return test.updateCrObject(obj, patch)
	}

}
