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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

const deprecatedTargetComponentServiceVersionSelectorEnvName = "DP_TARGET_COMPONENT_SERVICE_VERSION_SELECTOR"

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
				{Name: dptypes.DPTargetComponentServiceVersion, Value: "spoofed-pod-version"},
				{Name: deprecatedTargetComponentServiceVersionSelectorEnvName, Value: "spoofed-pod-selector"},
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
			Labels: map[string]string{
				DataProtectionInternalPostReadyLabelKey: DataProtectionInternalPostReadyLabelValue,
			},
			OwnerReferences: []metav1.OwnerReference{postReadyControllerRef(
				appsv1.APIVersion, appsv1.ComponentKind, component.Name, component.UID)},
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

func useInstanceTargetOwnership(
	t *testing.T,
	fixture postReadyTargetEnvFixture,
	instanceTemplateName,
	podTemplateLabel string) {
	t.Helper()
	ctx := context.Background()
	instanceSet := &workloadsv1.InstanceSet{}
	require.NoError(t, fixture.client.Get(ctx, fixture.component, instanceSet))
	podKey := types.NamespacedName{Namespace: fixture.component.Namespace, Name: fixture.component.Name + "-0"}
	pod := &corev1.Pod{}
	require.NoError(t, fixture.client.Get(ctx, podKey, pod))
	instance := &workloadsv1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			UID:       "instance-uid",
			OwnerReferences: []metav1.OwnerReference{postReadyControllerRef(
				workloadsv1.GroupVersion.String(), workloadsv1.InstanceSetKind, instanceSet.Name, instanceSet.UID)},
		},
		Spec: workloadsv1.InstanceSpec{InstanceTemplateName: instanceTemplateName},
	}
	require.NoError(t, fixture.client.Create(ctx, instance))
	pod.OwnerReferences = []metav1.OwnerReference{postReadyControllerRef(
		workloadsv1.GroupVersion.String(), "Instance", instance.Name, instance.UID)}
	pod.Labels[constant.KBAppInstanceTemplateLabelKey] = podTemplateLabel
	require.NoError(t, fixture.client.Update(ctx, pod))
}

func markPostReadyTargetOwnerTerminating(
	t *testing.T,
	fixture postReadyTargetEnvFixture,
	object client.Object) {
	t.Helper()
	ctx := context.Background()
	object.SetFinalizers(append(object.GetFinalizers(), "test.kubeblocks.io/finalizer"))
	require.NoError(t, fixture.client.Update(ctx, object))
	require.NoError(t, fixture.client.Delete(ctx, object))
	require.NoError(t, fixture.client.Get(ctx, client.ObjectKeyFromObject(object), object))
	require.False(t, object.GetDeletionTimestamp().IsZero())
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
	require.Equal(t, []string{"3.4.0"}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))
	require.Empty(t, postReadyJobEnvValues(env, deprecatedTargetComponentServiceVersionSelectorEnvName))

	component := &appsv1.Component{}
	require.NoError(t, fixture.client.Get(context.Background(), fixture.component, component))
	component.Spec.Instances[0].ServiceVersion = "3.5.0"
	component.Generation++
	component.Status.ObservedGeneration = component.Generation
	require.NoError(t, fixture.client.Update(context.Background(), component))

	env = build()
	require.Equal(t, []string{"shared-nothing"}, postReadyJobEnvValues(env, dptypes.DPTargetClusterTopology))
	require.Equal(t, []string{"3.5.0"}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))
	require.Empty(t, postReadyJobEnvValues(env, deprecatedTargetComponentServiceVersionSelectorEnvName))
}

func TestBuildPostReadyActionJobsFailsClosedWhenInternalRestoreOwnerIsMissing(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	fixture.manager.Restore.OwnerReferences = nil

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		fixture.client,
		fixture.client,
		fixture.backupSet,
		fixture.target,
		0)

	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
	require.Empty(t, jobs)
}

func TestBuildPostReadyActionJobsRejectsInvalidInternalRestoreIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*dpv1alpha1.Restore)
	}{
		{
			name: "marker missing",
			mutate: func(restore *dpv1alpha1.Restore) {
				delete(restore.Labels, DataProtectionInternalPostReadyLabelKey)
			},
		},
		{
			name: "marker conflict",
			mutate: func(restore *dpv1alpha1.Restore) {
				restore.Labels[DataProtectionInternalPostReadyLabelKey] = "false"
			},
		},
		{
			name: "marker and owner missing",
			mutate: func(restore *dpv1alpha1.Restore) {
				delete(restore.Labels, DataProtectionInternalPostReadyLabelKey)
				restore.OwnerReferences = nil
			},
		},
		{
			name: "wrong owner apiVersion",
			mutate: func(restore *dpv1alpha1.Restore) {
				restore.OwnerReferences[0].APIVersion = "v1"
			},
		},
		{
			name: "wrong owner kind",
			mutate: func(restore *dpv1alpha1.Restore) {
				restore.OwnerReferences[0].Kind = "Secret"
			},
		},
		{
			name: "wrong owner name",
			mutate: func(restore *dpv1alpha1.Restore) {
				restore.OwnerReferences[0].Name = "not-the-component"
			},
		},
		{
			name: "owner is not controller",
			mutate: func(restore *dpv1alpha1.Restore) {
				restore.OwnerReferences[0].Controller = nil
			},
		},
		{
			name: "wrong owner uid",
			mutate: func(restore *dpv1alpha1.Restore) {
				restore.OwnerReferences[0].UID = "wrong-component-uid"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newPostReadyTargetEnvFixture(t)
			tt.mutate(fixture.manager.Restore)

			jobs, err := fixture.manager.BuildPostReadyActionJobs(
				intctrlutil.RequestCtx{Ctx: context.Background()},
				fixture.client,
				fixture.client,
				fixture.backupSet,
				fixture.target,
				0,
			)

			require.Error(t, err)
			require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
			require.Empty(t, jobs)
		})
	}
}

func TestBuildPostReadyActionJobsIgnoresMutatedRestoreTargetEnv(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	fixture.manager.Restore.Spec.Env = []corev1.EnvVar{
		{Name: "KEEP_RESTORE_ENV", Value: "kept"},
		{Name: dptypes.DPTargetClusterTopology, Value: "mutated-restore-topology"},
		{Name: dptypes.DPTargetComponentServiceVersion, Value: "mutated-restore-version"},
		{Name: deprecatedTargetComponentServiceVersionSelectorEnvName, Value: "mutated-restore-selector"},
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
	require.Equal(t, []string{"3.4.0"}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))
	require.Empty(t, postReadyJobEnvValues(env, deprecatedTargetComponentServiceVersionSelectorEnvName))
	require.Equal(t, []string{"kept"}, postReadyJobEnvValues(env, "KEEP_RESTORE_ENV"))
}

func TestBuildPostReadyActionJobsPublishesEmptyExpectedServiceVersionWhenAppsHasNone(t *testing.T) {
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
	require.Equal(t, []string{""}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))
}

func TestBuildPostReadyActionJobsUsesInstanceAPITemplateName(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	// The label is controller-owned implementation detail and intentionally conflicts
	// with the authoritative public Instance API value.
	useInstanceTargetOwnership(t, fixture, "canary", "unknown")

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()}, fixture.client, fixture.client, fixture.backupSet, fixture.target, 0)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	env := jobs[0].Spec.Template.Spec.Containers[0].Env
	require.Equal(t, []string{"3.4.0"}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))
}

func TestBuildPostReadyActionJobsDoesNotFallbackFromEmptyInstanceAPITemplateName(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	// Empty is the authoritative default-instance sentinel. The spoofed Pod label
	// must not redirect this target to the canary instance template.
	useInstanceTargetOwnership(t, fixture, "", "canary")

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()}, fixture.client, fixture.client, fixture.backupSet, fixture.target, 0)

	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err)
	require.Empty(t, jobs)
}

func TestBuildPostReadyActionJobsUsesDefaultFromEmptyInstanceAPITemplateName(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	ctx := context.Background()
	component := &appsv1.Component{}
	require.NoError(t, fixture.client.Get(ctx, fixture.component, component))
	component.Spec.Replicas = 2
	component.Generation++
	component.Status.ObservedGeneration = component.Generation
	require.NoError(t, fixture.client.Update(ctx, component))
	// The public Instance API selects the default instance even though the Pod
	// label tries to redirect dispatch to the canary override.
	useInstanceTargetOwnership(t, fixture, "", "canary")

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: ctx}, fixture.client, fixture.client, fixture.backupSet, fixture.target, 0)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	env := jobs[0].Spec.Template.Spec.Containers[0].Env
	require.Equal(t, []string{"3.3.2"}, postReadyJobEnvValues(env, dptypes.DPTargetComponentServiceVersion))
}

func TestBuildPostReadyActionJobsWaitsForTerminatingInstanceSet(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	instanceSet := &workloadsv1.InstanceSet{}
	require.NoError(t, fixture.client.Get(context.Background(), fixture.component, instanceSet))
	markPostReadyTargetOwnerTerminating(t, fixture, instanceSet)

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()}, fixture.client, fixture.client, fixture.backupSet, fixture.target, 0)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.ErrorContains(t, err, "target InstanceSet "+fixture.component.String()+" is terminating")
	require.Empty(t, jobs)
}

func TestBuildPostReadyActionJobsWaitsForTerminatingInstance(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	useInstanceTargetOwnership(t, fixture, "canary", "canary")
	instanceKey := types.NamespacedName{Namespace: fixture.component.Namespace, Name: fixture.component.Name + "-0"}
	instance := &workloadsv1.Instance{}
	require.NoError(t, fixture.client.Get(context.Background(), instanceKey, instance))
	markPostReadyTargetOwnerTerminating(t, fixture, instance)

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()}, fixture.client, fixture.client, fixture.backupSet, fixture.target, 0)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.ErrorContains(t, err, "target Instance "+instanceKey.String()+" is terminating")
	require.Empty(t, jobs)
}

func TestBuildPostReadyActionJobsWaitsForTerminatingParentInstanceSet(t *testing.T) {
	fixture := newPostReadyTargetEnvFixture(t)
	useInstanceTargetOwnership(t, fixture, "canary", "canary")
	instanceSet := &workloadsv1.InstanceSet{}
	require.NoError(t, fixture.client.Get(context.Background(), fixture.component, instanceSet))
	markPostReadyTargetOwnerTerminating(t, fixture, instanceSet)

	jobs, err := fixture.manager.BuildPostReadyActionJobs(
		intctrlutil.RequestCtx{Ctx: context.Background()}, fixture.client, fixture.client, fixture.backupSet, fixture.target, 0)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.ErrorContains(t, err, "target InstanceSet "+fixture.component.String()+" is terminating")
	require.Empty(t, jobs)
}

func TestValidateTargetPodOwnershipKeepsInstanceSetLabelContract(t *testing.T) {
	canary := "canary"
	empty := ""
	tests := []struct {
		name      string
		label     *string
		want      string
		wantFatal bool
	}{
		{name: "override", label: &canary, want: "canary"},
		{name: "default", label: &empty},
		{name: "missing", wantFatal: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newPostReadyTargetEnvFixture(t)
			ctx := context.Background()
			component := &appsv1.Component{}
			require.NoError(t, fixture.client.Get(ctx, fixture.component, component))
			podKey := types.NamespacedName{Namespace: fixture.component.Namespace, Name: fixture.component.Name + "-0"}
			pod := &corev1.Pod{}
			require.NoError(t, fixture.client.Get(ctx, podKey, pod))
			if tt.label == nil {
				delete(pod.Labels, constant.KBAppInstanceTemplateLabelKey)
			} else {
				pod.Labels[constant.KBAppInstanceTemplateLabelKey] = *tt.label
			}

			got, err := validateTargetPodOwnership(
				intctrlutil.RequestCtx{Ctx: ctx}, fixture.client, pod, component)

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

func TestTargetPodExpectedServiceVersion(t *testing.T) {
	zero := int32(0)
	tests := []struct {
		name                 string
		component            appsv1.ComponentSpec
		instanceTemplateName string
		want                 string
		wantFatal            bool
	}{
		{
			name:                 "instance override",
			component:            appsv1.ComponentSpec{ServiceVersion: "3.3.2", Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary", ServiceVersion: "3.4.0"}}},
			instanceTemplateName: "canary",
			want:                 "3.4.0",
		},
		{
			name:                 "instance inherits component service version",
			component:            appsv1.ComponentSpec{ServiceVersion: "3.3.2", Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary"}}},
			instanceTemplateName: "canary",
			want:                 "3.3.2",
		},
		{
			name:      "default instance",
			component: appsv1.ComponentSpec{ServiceVersion: "3.3.2", Replicas: 2, Instances: []appsv1.InstanceTemplate{{Name: "canary", ServiceVersion: "3.4.0"}}},
			want:      "3.3.2",
		},
		{
			name:      "expected service version absent",
			component: appsv1.ComponentSpec{Replicas: 1},
		},
		{
			name:                 "inactive instance template",
			component:            appsv1.ComponentSpec{Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary", Replicas: &zero}}},
			instanceTemplateName: "canary",
			wantFatal:            true,
		},
		{
			name:                 "unknown instance template",
			component:            appsv1.ComponentSpec{Replicas: 1},
			instanceTemplateName: "unknown",
			wantFatal:            true,
		},
		{
			name:      "no default instances",
			component: appsv1.ComponentSpec{Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary"}}},
			wantFatal: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "target"}}
			component := &appsv1.Component{Spec: tt.component}

			got, err := targetPodExpectedServiceVersion(component, pod, tt.instanceTemplateName)

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
