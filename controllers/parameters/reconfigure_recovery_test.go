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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/operations"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func TestReconfigureRestoredContentWaitsForTargetRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, restored string
		recover        bool
		conflict       bool
		newRevision    bool
	}{
		{name: "restored content", restored: "enabled=ON\n", recover: true},
		{name: "restored semantics", restored: "# restored\nenabled=ON\n", recover: true},
		{name: "stable formatting change", restored: "# reformatted\nenabled=ON\n"},
		{name: "cluster conflict", restored: "enabled=ON\n", recover: true, conflict: true},
		{name: "newer revision", restored: "# restored\nenabled=ON\n", recover: true, newRevision: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
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
			targetValue := "ON"
			if tc.recover {
				targetHash = failedHash
				targetValue = "nonsense"
			}
			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
				Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
					Name: componentName, Replicas: 1,
					Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To(configName), ConfigHash: targetHash,
						Reconfigure: ptr.To(true), ReconfigureArgs: [][]string{{"enabled", targetValue}}}},
				}}},
			}
			otherConfig := appsv1.ClusterComponentConfig{Name: ptr.To("other"), ConfigHash: ptr.To("other-hash"),
				Reconfigure: ptr.To(true), ReconfigureArgs: [][]string{{"other", "value"}}}
			cluster.Spec.ComponentSpecs[0].Configs = append(cluster.Spec.ComponentSpecs[0].Configs, otherConfig)
			template := appsv1.ComponentFileTemplate{Name: configName, Template: "template", Namespace: namespace, ExternalManaged: ptr.To(true)}
			cp := &parametersv1alpha1.ComponentParameter{
				ObjectMeta: metav1.ObjectMeta{Name: core.GenerateComponentConfigurationName(clusterName, componentName), Namespace: namespace, Generation: 3, Finalizers: []string{constant.ConfigFinalizerName}},
				Spec: parametersv1alpha1.ComponentParameterSpec{ClusterName: clusterName, ComponentName: componentName,
					ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{Name: configName, ConfigSpec: &template}}},
				Status: parametersv1alpha1.ComponentParameterStatus{ConfigurationItemStatus: []parametersv1alpha1.ConfigTemplateItemDetailStatus{{Name: configName, Phase: parametersv1alpha1.CRunningPhase, UpdateRevision: "3"}}},
			}
			item, err := json.Marshal(cp.Spec.ConfigItemDetails[0])
			if err != nil {
				t.Fatal(err)
			}
			cm.Annotations[constant.ConfigAppliedVersionAnnotationKey] = string(item)
			cm.Annotations[constant.ParametersAppliedComponentGenerationKey] = "1"
			component := &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{Name: constant.GenerateClusterComponentName(clusterName, componentName), Namespace: namespace, Generation: 1,
					Labels: constant.GetCompLabels(clusterName, componentName), Annotations: map[string]string{constant.KBAppClusterUIDKey: "cluster-uid"}},
				Spec: appsv1.ComponentSpec{CompDef: "mysql"},
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
				Spec:       workloads.InstanceSetSpec{Configs: []workloads.ConfigTemplate{{Name: configName, ConfigHash: targetHash}}},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
					Configs: []workloads.InstanceConfigStatus{{Name: configName, ConfigHash: appliedHash}},
				}}},
			}
			ops := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: namespace},
				Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: clusterName, Type: opsv1alpha1.ReconfiguringType},
				Status:     opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase},
			}
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: componentName}}}
			tpl := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: template.Template, Namespace: namespace}, Data: cm.Data}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(cp, ops, its).
				WithObjects(cm, cluster, cp, component, definition, pd, its, ops, tpl).Build()
			reconciler := &ReconfigureReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}
			if tc.conflict {
				conflicted := false
				reconciler.Client = interceptor.NewClient(cli, interceptor.Funcs{
					Update: func(ctx context.Context, underlying client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
						if _, ok := obj.(*appsv1.Cluster); ok && !conflicted {
							conflicted = true
							concurrent := &appsv1.Cluster{}
							if err := underlying.Get(ctx, client.ObjectKeyFromObject(obj), concurrent); err != nil {
								return err
							}
							concurrent.Annotations = map[string]string{"concurrent": "kept"}
							if err := underlying.Update(ctx, concurrent); err != nil {
								return err
							}
						}
						return underlying.Update(ctx, obj, opts...)
					},
				})
				result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
				if err != nil || result.RequeueAfter == 0 || !conflicted {
					t.Fatalf("conflicting Cluster update did not requeue: result=%+v, conflict=%v, err=%v", result, conflicted, err)
				}
			}
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
			if status.Phase == parametersv1alpha1.CFinishedPhase {
				t.Errorf("restored parameters reported Finished before the failed target was recovered: %+v", status.ReconcileDetail)
			}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
				t.Fatal(err)
			}
			cfg := cluster.Spec.ComponentSpecs[0].Configs[0]
			if !reflect.DeepEqual(cluster.Spec.ComponentSpecs[0].Configs[1], otherConfig) {
				t.Fatal("recovery changed another configuration")
			}
			if tc.conflict && cluster.Annotations["concurrent"] != "kept" {
				t.Fatal("retry lost the concurrent Cluster update")
			}
			if tc.recover && ptr.Equal(cfg.ConfigHash, failedHash) {
				t.Errorf("restored parameters left failed Cluster target and arguments unchanged: %+v", cfg)
			}
			if !ptr.Equal(cfg.ConfigHash, computeTargetConfigHash(nil, cm.Data)) {
				t.Errorf("no-op update did not publish the current content hash: %+v", cfg)
			}
			if !tc.recover {
				if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)}); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
					t.Fatal(err)
				}
				if ptr.Deref(cluster.Spec.ComponentSpecs[0].Configs[0].Restart, false) {
					t.Fatal("repeated formatting-only reconciliation requested a restart")
				}
			}
			if ptr.Deref(cfg.Restart, true) || ptr.Deref(cfg.Reconfigure, true) || cfg.ReconfigureArgs != nil || cfg.ReconfigureAction != nil {
				t.Fatalf("recovery did not replace the failed action with the restored config target: %+v", cfg)
			}
			checkCompletion := func(finished bool) {
				t.Helper()
				if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)}); err != nil {
					t.Fatal(err)
				}
				cpReconciler := &ComponentParameterReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}
				if _, err := cpReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cp)}); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(cp), cp); err != nil {
					t.Fatal(err)
				}
				if got := cp.Status.Phase == parametersv1alpha1.CFinishedPhase; got != finished {
					t.Fatalf("ComponentParameter finished=%v, want %v: %+v", got, finished, cp.Status)
				}
				resource := &operations.OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(10)}
				phase, _, err := operations.GetOpsManager().OpsMap[opsv1alpha1.ReconfiguringType].OpsHandler.ReconcileAction(
					intctrlutil.RequestCtx{Ctx: ctx}, cli, resource)
				if err != nil {
					t.Fatal(err)
				}
				if got := phase == opsv1alpha1.OpsSucceedPhase; got != finished {
					t.Fatalf("OpsRequest succeeded=%v, want %v", got, finished)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), ops); err != nil {
					t.Fatal(err)
				}
				if ops.Status.Phase != phase {
					t.Fatalf("persisted OpsRequest phase=%s, returned %s", ops.Status.Phase, phase)
				}
			}
			checkCompletion(false)
			its.Spec.Configs = []workloads.ConfigTemplate{{Name: configName, ConfigHash: cfg.ConfigHash}}
			if err := cli.Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			if tc.restored != "enabled=ON\n" {
				checkCompletion(false)
			}
			its.Status.InstanceStatus[0].Configs[0].ConfigHash = cfg.ConfigHash
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			if tc.newRevision {
				changed := false
				reconciler.Client = interceptor.NewClient(cli, interceptor.Funcs{
					Patch: func(ctx context.Context, underlying client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if _, ok := obj.(*corev1.ConfigMap); ok && !changed {
							changed = true
							latest := &corev1.ConfigMap{}
							if err := underlying.Get(ctx, client.ObjectKeyFromObject(obj), latest); err != nil {
								return err
							}
							latest.Data["my.cnf"] = "enabled=OFF\n"
							latest.Annotations[constant.ConfigurationRevision] = "4"
							if err := underlying.Update(ctx, latest); err != nil {
								return err
							}
						}
						return underlying.Patch(ctx, obj, patch, opts...)
					},
				})
				result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
				if err != nil || result.RequeueAfter == 0 || !changed {
					t.Fatalf("new configuration revision did not requeue: result=%+v, changed=%v, err=%v", result, changed, err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(cm), cm); err != nil {
					t.Fatal(err)
				}
				if cm.Annotations[constant.LastAppliedConfigAnnotationKey] != string(applied) {
					t.Fatal("stale reconciliation advanced the applied configuration after a newer revision arrived")
				}
				if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)}); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
					t.Fatal(err)
				}
				if !ptr.Equal(cluster.Spec.ComponentSpecs[0].Configs[0].ConfigHash, computeTargetConfigHash(nil, cm.Data)) {
					t.Fatal("retry did not publish the newer configuration target")
				}
				return
			}
			checkCompletion(true)
		})
	}
}
