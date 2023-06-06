/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// CreateRestartOpsRequest creates an OpsRequest of restart type for testing.
func CreateRestartOpsRequest(testCtx *testutil.TestContext, clusterName, opsRequestName string, componentNames []string) *appsv1alpha1.OpsRequest {
	opsRequest := NewOpsRequestObj(opsRequestName, testCtx.DefaultNamespace, clusterName, appsv1alpha1.RestartType)
	componentList := make([]appsv1alpha1.ComponentOps, len(componentNames))
	for i := range componentNames {
		componentList[i] = appsv1alpha1.ComponentOps{ComponentName: componentNames[i]}
	}
	opsRequest.Spec.RestartList = componentList
	return CreateK8sResource(testCtx, opsRequest).(*appsv1alpha1.OpsRequest)
}

// NewOpsRequestObj only generates the OpsRequest Object, instead of actually creating this resource.
func NewOpsRequestObj(opsRequestName, namespace, clusterName string, opsType appsv1alpha1.OpsType) *appsv1alpha1.OpsRequest {
	return &appsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsRequestName,
			Namespace: namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.OpsRequestTypeLabelKey: string(opsType),
			},
		},
		Spec: appsv1alpha1.OpsRequestSpec{
			ClusterRef: clusterName,
			Type:       opsType,
		},
	}
}

// CreateOpsRequest calls the api to create the OpsRequest resource.
func CreateOpsRequest(ctx context.Context, testCtx testutil.TestContext, opsRequest *appsv1alpha1.OpsRequest) *appsv1alpha1.OpsRequest {
	gomega.Expect(testCtx.CreateObj(ctx, opsRequest)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(CheckObjExists(&testCtx, client.ObjectKeyFromObject(opsRequest), opsRequest, true)).Should(gomega.Succeed())
	return opsRequest
}

// GetOpsRequestCompPhase gets the component phase of testing OpsRequest for verification.
func GetOpsRequestCompPhase(ctx context.Context, testCtx testutil.TestContext, opsName, componentName string) func(g gomega.Gomega) appsv1alpha1.ClusterComponentPhase {
	return func(g gomega.Gomega) appsv1alpha1.ClusterComponentPhase {
		tmpOps := &appsv1alpha1.OpsRequest{}
		g.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: opsName,
			Namespace: testCtx.DefaultNamespace}, tmpOps)).Should(gomega.Succeed())
		if tmpOps.Status.Components == nil {
			return ""
		}
		return tmpOps.Status.Components[componentName].Phase
	}
}

// GetOpsRequestPhase gets the testing opsRequest phase for verification.
func GetOpsRequestPhase(testCtx *testutil.TestContext, opsKey types.NamespacedName) func(gomega.Gomega) appsv1alpha1.OpsPhase {
	return func(g gomega.Gomega) appsv1alpha1.OpsPhase {
		tmpOps := &appsv1alpha1.OpsRequest{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, opsKey, tmpOps)).To(gomega.Succeed())
		return tmpOps.Status.Phase
	}
}
