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

package operations

import (
	"context"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

// NewOpsRequestObj only generates the OpsRequest Object, instead of actually creating this resource.
func NewOpsRequestObj(opsRequestName, namespace, clusterName string, opsType opsv1alpha1.OpsType) *opsv1alpha1.OpsRequest {
	return &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsRequestName,
			Namespace: namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.OpsRequestTypeLabelKey: string(opsType),
			},
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			ClusterName: clusterName,
			Type:        opsType,
		},
	}
}

// CreateOpsRequest calls the api to create the OpsRequest resource.
func CreateOpsRequest(ctx context.Context, testCtx testutil.TestContext, opsRequest *opsv1alpha1.OpsRequest) *opsv1alpha1.OpsRequest {
	gomega.Expect(testCtx.CreateObj(ctx, opsRequest)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(opsRequest), opsRequest, true)).Should(gomega.Succeed())
	return opsRequest
}

// GetOpsRequestPhase gets the testing opsRequest phase for verification.
func GetOpsRequestPhase(testCtx *testutil.TestContext, opsKey types.NamespacedName) func(gomega.Gomega) opsv1alpha1.OpsPhase {
	return func(g gomega.Gomega) opsv1alpha1.OpsPhase {
		tmpOps := &opsv1alpha1.OpsRequest{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, opsKey, tmpOps)).To(gomega.Succeed())
		return tmpOps.Status.Phase
	}
}
