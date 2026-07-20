/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package restore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

type postReadyTargetEnvFixture struct {
	manager   *RestoreManager
	client    client.Client
	backupSet BackupActionSet
	target    *dpv1alpha1.BackupStatusTarget
	component types.NamespacedName
}

func postReadyControllerRef(apiVersion, kind, name string, uid types.UID) metav1.OwnerReference {
	controller := true
	return metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        uid,
		Controller: &controller,
	}
}

func newPostReadyTargetEnvFixture(t *testing.T) postReadyTargetEnvFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	require.NoError(t, workloadsv1.AddToScheme(scheme))

	const (
		namespace     = "default"
		clusterName   = "target"
		componentName = "mysql"
	)
	clusterUID := types.UID("cluster-uid")
	componentUID := types.UID("component-uid")
	instanceSetUID := types.UID("instanceset-uid")
	componentObjectName := constant.GenerateClusterComponentName(clusterName, componentName)

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, UID: clusterUID},
		Spec:       appsv1.ClusterSpec{Topology: "shared-nothing"},
	}
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       componentObjectName,
			UID:        componentUID,
			Generation: 1,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: componentName,
			},
			Annotations: map[string]string{constant.KBAppClusterUIDKey: string(clusterUID)},
		},
		Spec: appsv1.ComponentSpec{
			ServiceVersion: "3.3.2",
			Replicas:       1,
			Instances: []appsv1.InstanceTemplate{{
				Name:           "canary",
				ServiceVersion: "3.4.0",
			}},
		},
		Status: appsv1.ComponentStatus{
			ObservedGeneration: 1,
			Phase:              appsv1.RunningComponentPhase,
		},
	}
	instanceSet := &workloadsv1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      componentObjectName,
			UID:       instanceSetUID,
			OwnerReferences: []metav1.OwnerReference{postReadyControllerRef(
				appsv1.APIVersion, appsv1.ComponentKind, component.Name, component.UID)},
		},
	}
	targetLabels := map[string]string{
		"restore-target":                       "true",
		constant.AppInstanceLabelKey:           clusterName,
		constant.KBAppComponentLabelKey:        componentName,
		constant.KBAppInstanceTemplateLabelKey: "canary",
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            componentObjectName + "-0",
			UID:             "pod-uid",
			Labels:          targetLabels,
			OwnerReferences: []metav1.OwnerReference{postReadyControllerRef(workloadsv1.GroupVersion.String(), workloadsv1.InstanceSetKind, instanceSet.Name, instanceSet.UID)},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "database",
			Env: []corev1.EnvVar{
				{Name: dptypes.DPTargetClusterTopology, Value: "spoofed-pod-topology"},
				{Name: dptypes.DPTargetComponentServiceVersion, Value: "spoofed-legacy-version"},
				{Name: dptypes.DPTargetComponentServiceVersionSelector, Value: "spoofed-pod-selector"},
			},
		}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, component, instanceSet, pod).Build()

	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      PostReadyRestoreName(component.UID),
			UID:       "restore-uid-1234",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: appsv1.APIVersion,
				Kind:       appsv1.ComponentKind,
				Name:       component.Name,
				UID:        component.UID,
			}},
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: namespace},
			Env:    []corev1.EnvVar{{Name: "KEEP_RESTORE_ENV", Value: "kept"}},
			ReadyConfig: &dpv1alpha1.ReadyConfig{JobAction: &dpv1alpha1.JobAction{
				Target: dpv1alpha1.JobActionTarget{PodSelector: dpv1alpha1.PodSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"restore-target": "true"}},
					Strategy:      dpv1alpha1.PodSelectionStrategyAny,
				}},
			}},
		},
	}
	backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "backup"}}
	actionSet := &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{
		Restore: &dpv1alpha1.RestoreActionSpec{PostReady: []dpv1alpha1.ActionSpec{{
			Job: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
				Image: "restore:latest", Command: []string{"restore"},
			}},
		}}},
	}}
	target := &dpv1alpha1.BackupStatusTarget{BackupTarget: dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAny},
	}}
	return postReadyTargetEnvFixture{
		manager:   NewRestoreManager(restore, nil, scheme, k8sClient),
		client:    k8sClient,
		backupSet: BackupActionSet{Backup: backup, ActionSet: actionSet},
		target:    target,
		component: client.ObjectKeyFromObject(component),
	}
}

func postReadyJobEnvValues(env []corev1.EnvVar, name string) []string {
	var values []string
	for i := range env {
		if env[i].Name == name {
			values = append(values, env[i].Value)
		}
	}
	return values
}

func TestBuildPostReadyActionJobsResolvesTargetFactsAtDispatch(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	build := func() []corev1.EnvVar {
		jobs, err := fixture.manager.BuildPostReadyActionJobs(
			reqCtx, fixture.client, fixture.client, fixture.backupSet, fixture.target, 0)
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		return jobs[0].Spec.Template.Spec.Containers[0].Env
	}

	env := build()
	require.Equal(t, []string{"shared-nothing"}, postReadyJobEnvValues(env, dptypes.DPTargetClusterTopology))
	require.Equal(t, []string{"3.4.0"}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersionSelector))
	require.Empty(t, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))

	component := &appsv1.Component{}
	require.NoError(t, fixture.client.Get(context.Background(), fixture.component, component))
	component.Spec.Instances[0].ServiceVersion = "3.5.0"
	component.Generation++
	component.Status.ObservedGeneration = component.Generation
	require.NoError(t, fixture.client.Update(context.Background(), component))

	env = build()
	require.Equal(t, []string{"shared-nothing"}, postReadyJobEnvValues(env, dptypes.DPTargetClusterTopology))
	require.Equal(t, []string{"3.5.0"}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersionSelector))
	require.Empty(t, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))
}

func TestBuildPostReadyActionJobsIgnoresMutatedRestoreTargetEnv(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	fixture.manager.Restore.Spec.Env = []corev1.EnvVar{
		{Name: "KEEP_RESTORE_ENV", Value: "kept"},
		{Name: dptypes.DPTargetClusterTopology, Value: "mutated-restore-topology"},
		{Name: dptypes.DPTargetComponentServiceVersion, Value: "mutated-restore-version"},
		{Name: dptypes.DPTargetComponentServiceVersionSelector, Value: "mutated-restore-selector"},
	}

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		fixture.client,
		fixture.client,
		fixture.backupSet,
		fixture.target,
		0,
	)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	env := jobs[0].Spec.Template.Spec.Containers[0].Env
	require.Equal(t, []string{"shared-nothing"}, postReadyJobEnvValues(env, dptypes.DPTargetClusterTopology))
	require.Equal(t, []string{"3.4.0"}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersionSelector))
	require.Empty(t, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))
	require.Equal(t, []string{"kept"}, postReadyJobEnvValues(env, "KEEP_RESTORE_ENV"))
}

func TestBuildPostReadyActionJobsPublishesEmptyDeclaredSelectorWhenAppsHasNone(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	component := &appsv1.Component{}
	require.NoError(t, fixture.client.Get(context.Background(), fixture.component, component))
	component.Spec.ServiceVersion = ""
	component.Spec.Instances[0].ServiceVersion = ""
	component.Generation++
	component.Status.ObservedGeneration = component.Generation
	require.NoError(t, fixture.client.Update(context.Background(), component))

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		fixture.client,
		fixture.client,
		fixture.backupSet,
		fixture.target,
		0,
	)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	env := jobs[0].Spec.Template.Spec.Containers[0].Env
	require.Equal(t, []string{""}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersionSelector))
}

func TestBuildPostReadyActionJobsWaitsForObservedTargetComponentGeneration(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	component := &appsv1.Component{}
	require.NoError(t, fixture.client.Get(context.Background(), fixture.component, component))
	component.Spec.Instances[0].ServiceVersion = "3.5.0"
	component.Generation++
	require.NoError(t, fixture.client.Update(context.Background(), component))

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		fixture.client,
		fixture.client,
		fixture.backupSet,
		fixture.target,
		0,
	)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.Empty(t, jobs)
}

func TestTargetPodServiceVersionSelector(t *testing.T) {
	zero := int32(0)
	tests := []struct {
		name          string
		component     appsv1.ComponentSpec
		templateLabel *string
		want          string
		wantFatal     bool
	}{
		{
			name:          "instance override",
			component:     appsv1.ComponentSpec{ServiceVersion: "3.3.2", Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary", ServiceVersion: "3.4.0"}}},
			templateLabel: ptr.To("canary"),
			want:          "3.4.0",
		},
		{
			name:          "instance inherits component selector",
			component:     appsv1.ComponentSpec{ServiceVersion: "3.3.2", Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary"}}},
			templateLabel: ptr.To("canary"),
			want:          "3.3.2",
		},
		{
			name:          "default instance",
			component:     appsv1.ComponentSpec{ServiceVersion: "3.3.2", Replicas: 2, Instances: []appsv1.InstanceTemplate{{Name: "canary", ServiceVersion: "3.4.0"}}},
			templateLabel: ptr.To(""),
			want:          "3.3.2",
		},
		{
			name:          "declared selector absent",
			component:     appsv1.ComponentSpec{Replicas: 1},
			templateLabel: ptr.To(""),
		},
		{
			name:          "inactive instance template",
			component:     appsv1.ComponentSpec{Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary", Replicas: &zero}}},
			templateLabel: ptr.To("canary"),
			wantFatal:     true,
		},
		{
			name:          "unknown instance template",
			component:     appsv1.ComponentSpec{Replicas: 1},
			templateLabel: ptr.To("unknown"),
			wantFatal:     true,
		},
		{
			name:          "no default instances",
			component:     appsv1.ComponentSpec{Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary"}}},
			templateLabel: ptr.To(""),
			wantFatal:     true,
		},
		{
			name:      "label missing",
			component: appsv1.ComponentSpec{Replicas: 1},
			wantFatal: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "target"}}
			if tt.templateLabel != nil {
				pod.Labels = map[string]string{constant.KBAppInstanceTemplateLabelKey: *tt.templateLabel}
			}
			component := &appsv1.Component{Spec: tt.component}

			got, err := targetPodServiceVersionSelector(component, pod)

			if tt.wantFatal {
				require.Error(t, err)
				require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
