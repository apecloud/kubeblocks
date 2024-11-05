/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"context"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/testutil"
)

func CreateK8sResource(testCtx *testutil.TestContext, obj client.Object) client.Object {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, obj)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(CheckObjExists(testCtx, client.ObjectKeyFromObject(obj),
		obj, true)).Should(gomega.Succeed())
	return obj
}

func CheckedCreateK8sResource(testCtx *testutil.TestContext, obj client.Object) client.Object {
	gomega.Expect(testCtx.CheckedCreateObj(testCtx.Ctx, obj)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(CheckObjExists(testCtx, client.ObjectKeyFromObject(obj),
		obj, true)).Should(gomega.Succeed())
	return obj
}

// GetClusterComponentPhase gets the component phase of testing cluster for verification.
func GetClusterComponentPhase(testCtx *testutil.TestContext, clusterKey types.NamespacedName, componentName string) func(g gomega.Gomega) appsv1.ComponentPhase {
	return func(g gomega.Gomega) appsv1.ComponentPhase {
		tmpCluster := &appsv1.Cluster{}
		g.Expect(testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterKey.Name,
			Namespace: clusterKey.Namespace}, tmpCluster)).Should(gomega.Succeed())
		return tmpCluster.Status.Components[componentName].Phase
	}
}

// GetClusterPhase gets the testing cluster's phase in status for verification.
func GetClusterPhase(testCtx *testutil.TestContext, clusterKey types.NamespacedName) func(gomega.Gomega) appsv1.ClusterPhase {
	return func(g gomega.Gomega) appsv1.ClusterPhase {
		cluster := &appsv1.Cluster{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(gomega.Succeed())
		return cluster.Status.Phase
	}
}

// GetClusterGeneration gets the testing cluster's metadata.generation.
func GetClusterGeneration(testCtx *testutil.TestContext, clusterKey types.NamespacedName) func(gomega.Gomega) int64 {
	return func(g gomega.Gomega) int64 {
		cluster := &appsv1.Cluster{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(gomega.Succeed())
		return cluster.GetGeneration()
	}
}

// GetClusterObservedGeneration gets the testing cluster's ObservedGeneration in status for verification.
func GetClusterObservedGeneration(testCtx *testutil.TestContext, clusterKey types.NamespacedName) func(gomega.Gomega) int64 {
	return func(g gomega.Gomega) int64 {
		cluster := &appsv1.Cluster{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(gomega.Succeed())
		return cluster.Status.ObservedGeneration
	}
}

// ClusterReconciled checks if the testing cluster has been reconciled.
func ClusterReconciled(testCtx *testutil.TestContext, clusterKey types.NamespacedName) func(gomega.Gomega) bool {
	return func(g gomega.Gomega) bool {
		cluster := &appsv1.Cluster{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(gomega.Succeed())
		return cluster.Status.ObservedGeneration > 0 && cluster.Status.ObservedGeneration == cluster.Generation
	}
}

// NewPVCSpec creates appsv1alpha1.PersistentVolumeClaimSpec.
func NewPVCSpec(size string) appsv1.PersistentVolumeClaimSpec {
	return appsv1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(size),
			},
		},
	}
}
