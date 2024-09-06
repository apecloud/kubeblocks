/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hook

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
)

const kubeblocksVersionLabelName = "app.kubernetes.io/version"

func PrepareFor(ctx *UpgradeContext) (err error) {
	ctx.From, err = getVersionInfo(ctx, ctx.K8sClient, ctx.Namespace)

	Log("kubeblocks upgrade: [from: %v --> to: %v]", ctx.From, ctx.To)
	return
}

func getVersionInfo(ctx context.Context, client *kubernetes.Clientset, namespace string) (*Version, error) {
	deploy, err := GetKubeBlocksDeploy(ctx, client, namespace, kubeblocksAppComponent)
	if err != nil {
		return nil, err
	}

	labels := deploy.GetLabels()
	if len(labels) == 0 {
		return nil, fmt.Errorf("KubeBlocks deployment has no labels")
	}

	if v, ok := labels[kubeblocksVersionLabelName]; ok {
		return NewVersion(v)
	}
	return nil, fmt.Errorf("KubeBlocks deployment has no version label")
}
