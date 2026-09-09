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

func TestShardingComponents(t *testing.T) {
	member := resourceTestComponent("shard-0", "db")
	member.Labels[constant.KBAppShardingNameLabelKey] = "shard"
	ordinary := resourceTestComponent("app", "app")
	otherCluster := member.DeepCopy()
	otherCluster.Name = "other-shard-0"
	otherCluster.Labels[constant.AppInstanceLabelKey] = "other"
	otherNamespace := member.DeepCopy()
	otherNamespace.Namespace = "other"
	r := &ComponentDrivenParameterReconciler{Client: resourceTestClient(t, member, ordinary, otherCluster, otherNamespace)}
	requests := r.shardingComponents(context.Background(), &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"}})
	require.Len(t, requests, 1)
	require.Equal(t, client.ObjectKeyFromObject(member), requests[0].NamespacedName)
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
