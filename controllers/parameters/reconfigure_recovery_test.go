/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package parameters

import (
	"context"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func TestReconfigureRestoredContentWaitsForTargetRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, restored string
		recover        bool
	}{
		{name: "restored content", restored: "enabled=ON\n", recover: true},
		{name: "restored semantics", restored: "# restored\nenabled=ON\n", recover: true},
		{name: "stable formatting change", restored: "# reformatted\nenabled=ON\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			const namespace, clusterName, componentName, configName = "default", "mysql", "mysql", "mysql-config"
			applied, err := json.Marshal(map[string]string{"my.cnf": "enabled=ON\n"})
			if err != nil {
				t.Fatal(err)
			}
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: core.GetComponentCfgName(clusterName, componentName, configName), Namespace: namespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: clusterName, constant.KBAppComponentLabelKey: componentName,
						constant.CMConfigurationTemplateNameLabelKey: configName,
						constant.CMConfigurationTypeLabelKey:         constant.ConfigInstanceType,
						constant.CMConfigurationSpecProviderLabelKey: configName,
					},
					Annotations: map[string]string{constant.LastAppliedConfigAnnotationKey: string(applied), constant.ConfigurationRevision: "3"},
				},
				Data: map[string]string{"my.cnf": tc.restored},
			}
			failedHash := computeTargetConfigHash(nil, map[string]string{"my.cnf": "enabled=nonsense\n"})
			appliedHash := computeTargetConfigHash(nil, map[string]string{"my.cnf": "enabled=ON\n"})
			targetHash := appliedHash
			if tc.recover {
				targetHash = failedHash
			}
			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
				Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
					Name: componentName, Replicas: 1,
					Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To(configName), ConfigHash: targetHash,
						Reconfigure: ptr.To(true), ReconfigureArgs: [][]string{{"enabled", "nonsense"}}}},
				}}},
			}
			template := appsv1.ComponentFileTemplate{Name: configName, Template: "template"}
			cp := &parametersv1alpha1.ComponentParameter{
				ObjectMeta: metav1.ObjectMeta{Name: core.GenerateComponentConfigurationName(clusterName, componentName), Namespace: namespace},
				Spec: parametersv1alpha1.ComponentParameterSpec{ClusterName: clusterName, ComponentName: componentName,
					ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{Name: configName, ConfigSpec: &template}}},
			}
			component := &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{Name: constant.GenerateClusterComponentName(clusterName, componentName), Namespace: namespace},
				Spec:       appsv1.ComponentSpec{CompDef: "mysql"},
			}
			definition := &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
				Spec:       appsv1.ComponentDefinitionSpec{Configs: []appsv1.ComponentFileTemplate{template}},
				Status:     appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase},
			}
			pd := &parametersv1alpha1.ParametersDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
				Spec: parametersv1alpha1.ParametersDefinitionSpec{ComponentDef: "mysql", TemplateName: configName, FileName: "my.cnf",
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Ini}},
				Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase},
			}
			its := &workloads.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{Name: constant.GenerateWorkloadNamePattern(clusterName, componentName), Namespace: namespace},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
					Configs: []workloads.InstanceConfigStatus{{Name: configName, ConfigHash: appliedHash}},
				}}},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, cluster, cp, component, definition, pd, its).Build()
			reconciler := &ReconfigureReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}
			if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)}); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(cm), cm); err != nil {
				t.Fatal(err)
			}
			status := parametersv1alpha1.ConfigTemplateItemDetailStatus{UpdateRevision: "3"}
			if err := syncStatus(cm, &status); err != nil {
				t.Fatal(err)
			}
			if tc.recover && status.Phase == parametersv1alpha1.CFinishedPhase {
				t.Errorf("restored parameters reported Finished before the failed target was recovered: %+v", status.ReconcileDetail)
			}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
				t.Fatal(err)
			}
			cfg := cluster.Spec.ComponentSpecs[0].Configs[0]
			if tc.recover && ptr.Equal(cfg.ConfigHash, failedHash) {
				t.Errorf("restored parameters left failed Cluster target and arguments unchanged: %+v", cfg)
			}
			if !tc.recover && (!ptr.Equal(cfg.ConfigHash, appliedHash) || ptr.Deref(cfg.Restart, false)) {
				t.Errorf("formatting-only update changed the stable target or requested a restart: %+v", cfg)
			}
		})
	}
}
