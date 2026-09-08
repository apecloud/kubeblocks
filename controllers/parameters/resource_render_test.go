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

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func resourceTestComponent(name, def string) *appsv1.Component {
	return &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "test-" + name, Namespace: "ns",
		Labels: map[string]string{constant.AppInstanceLabelKey: "test"}}, Spec: appsv1.ComponentSpec{CompDef: def,
		Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
		}}},
	}}
}

func storageVar(def string) appsv1.EnvVar {
	return appsv1.EnvVar{Name: "DISK", ValueFrom: &appsv1.VarSource{ResourceVarRef: &appsv1.ResourceVarSelector{
		ClusterObjectReference: appsv1.ClusterObjectReference{CompDef: def},
		ResourceVars:           appsv1.ResourceVars{Storage: &appsv1.NamedVar{Name: "data"}},
	}}}
}

func resourceTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, parametersv1alpha1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestResourceRenderFingerprint(t *testing.T) {
	ctx := context.Background()
	cmpd := &appsv1.ComponentDefinition{}
	original := resourceTestComponent("db", "db")
	cli := resourceTestClient(t)
	baseline, err := resourceRenderInputFingerprint(ctx, cli, cmpd, original, nil)
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		change  func(*appsv1.Component)
		changed bool
	}{
		{"storage", func(c *appsv1.Component) {
			c.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
		}, true},
		{"cpu", func(c *appsv1.Component) { c.Spec.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("1500m") }, true},
		{"memory limit", func(c *appsv1.Component) {
			c.Spec.Resources.Limits = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")}
		}, true},
		{"equivalent units", func(c *appsv1.Component) {
			c.Spec.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("1000m")
			c.Spec.Resources.Requests[corev1.ResourceMemory] = resource.MustParse("1024Mi")
			c.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1073741824")
		}, false},
		{"metadata", func(c *appsv1.Component) {
			c.Generation++
			c.ResourceVersion = "99"
			c.Annotations = map[string]string{"unrelated": "changed"}
		}, false},
		{"replicas", func(c *appsv1.Component) { c.Spec.Replicas++ }, false},
		{"deleted volume", func(c *appsv1.Component) { c.Spec.VolumeClaimTemplates = nil }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			comp := original.DeepCopy()
			tc.change(comp)
			actual, err := resourceRenderInputFingerprint(ctx, cli, cmpd, comp, nil)
			require.NoError(t, err)
			require.Equal(t, tc.changed, baseline != actual)
		})
	}
	original.Spec.VolumeClaimTemplates = append(original.Spec.VolumeClaimTemplates, appsv1.PersistentVolumeClaimTemplate{Name: "log"})
	before, err := resourceRenderInputFingerprint(ctx, cli, cmpd, original, nil)
	require.NoError(t, err)
	original.Spec.VolumeClaimTemplates[0], original.Spec.VolumeClaimTemplates[1] = original.Spec.VolumeClaimTemplates[1], original.Spec.VolumeClaimTemplates[0]
	after, err := resourceRenderInputFingerprint(ctx, cli, cmpd, original, nil)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestResourceRenderReferences(t *testing.T) {
	ctx := context.Background()
	target := resourceTestComponent("app", "app-def")
	source := resourceTestComponent("db", "db-def")
	cli := resourceTestClient(t, source, target)
	cmpd := &appsv1.ComponentDefinition{Spec: appsv1.ComponentDefinitionSpec{Vars: []appsv1.EnvVar{storageVar("db-def")}}}
	synth := &component.SynthesizedComponent{Namespace: "ns", ClusterName: "test", Name: "app", CompDefName: "app-def", Comp2CompDefs: map[string]string{"app": "app-def", "db": "db-def"}}
	first, err := resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.NoError(t, err)
	snap := newResourceSnapshotReader(cli, target)
	cached, err := resourceRenderInputFingerprint(ctx, snap, cmpd, target, synth)
	require.NoError(t, err)
	require.Equal(t, first, cached)
	source.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	require.NoError(t, cli.Update(ctx, source))
	next, err := resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.NoError(t, err)
	require.NotEqual(t, first, next)
	cached, err = resourceRenderInputFingerprint(ctx, snap, cmpd, target, synth)
	require.NoError(t, err)
	require.Equal(t, first, cached)
	values, err := component.ResolveResourceRenderVars(ctx, snap, synth, cmpd.Spec.Vars)
	require.NoError(t, err)
	require.Equal(t, "1073741824", values["DISK"])
	require.NoError(t, cli.Delete(ctx, source))
	_, err = resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.Error(t, err)
	optional := true
	cmpd.Spec.Vars[0].ValueFrom.ResourceVarRef.Optional = &optional
	absent, err := resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.NoError(t, err)
	require.NotEqual(t, next, absent)
	// A new source must invalidate the optional-missing snapshot.
	source.ResourceVersion = ""
	require.NoError(t, cli.Create(ctx, source))
	restored, err := resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.NoError(t, err)
	require.NotEqual(t, absent, restored)
	// Unrelated Secret variables are not resolved by the fingerprint builder.
	cmpd.Spec.Vars = append(cmpd.Spec.Vars, appsv1.EnvVar{Name: "PASSWORD", ValueFrom: &appsv1.VarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "missing"}, Key: "password"}}})
	unchanged, err := resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.NoError(t, err)
	require.Equal(t, restored, unchanged)
}

func TestResourcePayloadMerge(t *testing.T) {
	expected := &parametersv1alpha1.ConfigTemplateItemDetail{Payload: parametersv1alpha1.Payload{constant.ResourceRenderInputsPayload: json.RawMessage(`{"version":"v1","digest":"new"}`)}}
	dest := &parametersv1alpha1.ConfigTemplateItemDetail{Payload: parametersv1alpha1.Payload{"external": json.RawMessage(`{"revision":7}`), constant.ResourceRenderInputsPayload: json.RawMessage(`{"digest":"old"}`), constant.ReplicasPayload: json.RawMessage(`2`)}}
	mergeResourcePayload(dest, expected)
	require.JSONEq(t, `{"revision":7}`, string(dest.Payload["external"]))
	require.Equal(t, expected.Payload[constant.ResourceRenderInputsPayload], dest.Payload[constant.ResourceRenderInputsPayload])
	require.NotContains(t, dest.Payload, constant.ReplicasPayload)
	before := dest.DeepCopy()
	mergeResourcePayload(dest, expected)
	require.Equal(t, before, dest)
}

func TestResourceInputDependents(t *testing.T) {
	ctx := context.Background()
	source := resourceTestComponent("db", "db-def")
	consumer := resourceTestComponent("app", "app-def")
	unrelated := resourceTestComponent("other", "plain-def")
	otherCluster := resourceTestComponent("other-app", "app-def")
	otherCluster.Labels[constant.AppInstanceLabelKey] = "other"
	otherNamespace := resourceTestComponent("other-ns", "app-def")
	otherNamespace.Namespace = "different"
	def := &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "app-def"}, Spec: appsv1.ComponentDefinitionSpec{Vars: []appsv1.EnvVar{storageVar("db-def")}}}
	cli := resourceTestClient(t, source, consumer, unrelated, otherCluster, otherNamespace, def,
		&appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "db-def"}}, &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "plain-def"}})
	reconciler := &ComponentDrivenParameterReconciler{Client: cli}
	requests := reconciler.resourceInputDependents(ctx, source)
	require.Len(t, requests, 1)
	require.Equal(t, client.ObjectKeyFromObject(consumer), requests[0].NamespacedName)
	// Deleting the source must still enqueue declarations which no longer resolve.
	require.NoError(t, cli.Delete(ctx, source))
	require.Equal(t, requests, reconciler.resourceInputDependents(ctx, source))
	require.Equal(t, requests, reconciler.resourceInputDependents(ctx, &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"}}))
	require.Len(t, reconciler.resourceInputDependents(ctx, def), 3)
}

func TestResourceRenderRejectsStaleInput(t *testing.T) {
	recorded, err := json.Marshal(resourceRenderFingerprint{Version: "v1", Digest: "old"})
	require.NoError(t, err)
	item := parametersv1alpha1.ConfigTemplateItemDetail{Payload: parametersv1alpha1.Payload{constant.ResourceRenderInputsPayload: recorded}}
	status := &parametersv1alpha1.ConfigTemplateItemDetailStatus{Phase: parametersv1alpha1.CFinishedPhase}
	err = syncImpl(&TaskContext{resourceInputs: resourceRenderFingerprint{Version: "v1", Digest: "new"}}, &Task{}, item, status, "7", nil)
	require.ErrorContains(t, err, "waiting for ComponentParameter")
	require.Equal(t, parametersv1alpha1.CPendingPhase, status.Phase)
}

func TestResourceRenderMultipleReferences(t *testing.T) {
	ctx := context.Background()
	target := resourceTestComponent("app", "app-def")
	first := resourceTestComponent("db-0", "db-def")
	second := resourceTestComponent("db-1", "db-def")
	cli := resourceTestClient(t, target, first, second)
	ref := storageVar("db-def")
	all := true
	ref.ValueFrom.ResourceVarRef.MultipleClusterObjectOption = &appsv1.MultipleClusterObjectOption{
		RequireAllComponentObjects: &all, Strategy: appsv1.MultipleClusterObjectStrategyIndividual,
	}
	cmpd := &appsv1.ComponentDefinition{Spec: appsv1.ComponentDefinitionSpec{Vars: []appsv1.EnvVar{ref}}}
	synth := &component.SynthesizedComponent{Namespace: "ns", ClusterName: "test", Name: "app", CompDefName: "app-def",
		Comp2CompDefs: map[string]string{"app": "app-def", "db-0": "db-def", "db-1": "db-def"}, CompDef2CompCnt: map[string]int32{"db-def": 2}}
	before, err := resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.NoError(t, err)
	values, err := component.ResolveResourceRenderVars(ctx, cli, synth, cmpd.Spec.Vars)
	require.NoError(t, err)
	require.Equal(t, "1073741824", values["DISK_DB_0"])
	require.Equal(t, "1073741824", values["DISK_DB_1"])
	second.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	require.NoError(t, cli.Update(ctx, second))
	after, err := resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.NoError(t, err)
	require.NotEqual(t, before, after)
	delete(synth.Comp2CompDefs, "db-1")
	_, err = resourceRenderInputFingerprint(ctx, cli, cmpd, target, synth)
	require.ErrorContains(t, err, "insufficient component objects")
}

func TestResourceRenderSeedsLegacyPayload(t *testing.T) {
	ctx := context.Background()
	comp := resourceTestComponent("db", "db")
	cli := resourceTestClient(t)
	def := &appsv1.ComponentDefinition{}
	spec := &parametersv1alpha1.ComponentParameterSpec{ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{Name: "config"}}}
	require.NoError(t, applyResourceRenderInputs(ctx, cli, def, comp, spec))
	require.Contains(t, spec.ConfigItemDetails[0].Payload, constant.ResourceRenderInputsPayload)
	before := spec.DeepCopy()
	require.NoError(t, applyResourceRenderInputs(ctx, cli, def, comp, spec))
	require.Equal(t, before, spec)
}

func TestResourceSnapshotKeepsMissingReferents(t *testing.T) {
	ctx := context.Background()
	target := resourceTestComponent("app", "app-def")
	cli := resourceTestClient(t, target)
	snap := newResourceSnapshotReader(cli, target)
	source := resourceTestComponent("db", "db-def")
	require.Error(t, snap.Get(ctx, client.ObjectKeyFromObject(source), &appsv1.Component{}))
	require.NoError(t, cli.Create(ctx, source))
	require.Error(t, snap.Get(ctx, client.ObjectKeyFromObject(source), &appsv1.Component{}))
	require.NoError(t, newResourceSnapshotReader(cli, target).Get(ctx, client.ObjectKeyFromObject(source), &appsv1.Component{}))
}

func TestResourceRenderUnchangedOutputSkipsApply(t *testing.T) {
	ctx := context.Background()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: "ns", Annotations: map[string]string{
		constant.ConfigurationRevision: "2", constant.LastAppliedConfigAnnotationKey: `{"my.cnf":"cache=9"}`,
	}, Labels: map[string]string{}}, Data: map[string]string{"my.cnf": "cache=9"}}
	for _, label := range reconfigureRequiredLabels {
		cm.Labels[label] = "test"
	}
	cm.Labels[constant.CMConfigurationTypeLabelKey] = constant.ConfigInstanceType
	cli := resourceTestClient(t, cm)
	// No Cluster, workload or database runtime exists. Any attempt to apply the
	// configuration would fail while fetching those resources.
	r := &ReconfigureReconciler{Client: cli}
	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
	require.NoError(t, err)
	require.Zero(t, result.RequeueAfter)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(cm), cm))
	require.Contains(t, cm.Annotations, core.GenerateRevisionPhaseKey("2"))
	revisions := RetrieveRevision(cm.Annotations)
	require.Len(t, revisions, 1)
	require.Equal(t, parametersv1alpha1.CFinishedPhase, revisions[0].Phase)
	require.Contains(t, revisions[0].Result.Message, "not been modified")
}
