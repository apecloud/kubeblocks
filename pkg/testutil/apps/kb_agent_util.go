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
	"fmt"

	. "github.com/onsi/ginkgo/v2"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	kbagent "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/testutil"
)

func MockKBAgentClient(mock func(*kbagent.MockClientMockRecorder)) {
	cli := kbagent.NewMockClient(gomock.NewController(GinkgoT()))
	if mock != nil {
		mock(cli.EXPECT())
	}
	kbagent.SetMockClient(cli, nil)
}

func MockKBAgentClientDefault() {
	MockKBAgentClient(func(recorder *kbagent.MockClientMockRecorder) {
		recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
			return kbagentproto.ActionResponse{}, nil
		}).AnyTimes()
	})
}

func MockKBAgentClient4HScale(testCtx *testutil.TestContext, clusterKey types.NamespacedName, compName, podAnnotationKey4Test string, replicas int) {
	MockKBAgentClient(func(recorder *kbagent.MockClientMockRecorder) {
		recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
			rsp := kbagentproto.ActionResponse{}
			if req.Action != "memberLeave" {
				return rsp, nil
			}
			var podList corev1.PodList
			labels := client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: compName,
			}
			if err := testCtx.Cli.List(ctx, &podList, labels, client.InNamespace(clusterKey.Namespace)); err != nil {
				return rsp, err
			}
			for _, pod := range podList.Items {
				if pod.Annotations == nil {
					panic(fmt.Sprintf("pod annotations is nil: %s", pod.Name))
				}
				if pod.Annotations[podAnnotationKey4Test] == fmt.Sprintf("%d", replicas) {
					continue
				}
				pod.Annotations[podAnnotationKey4Test] = fmt.Sprintf("%d", replicas)
				if err := testCtx.Cli.Update(ctx, &pod); err != nil {
					return rsp, err
				}
			}
			return rsp, nil
		}).AnyTimes()
	})
}
