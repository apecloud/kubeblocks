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
	"k8s.io/client-go/util/retry"
)

const (
	// kubeblocksAppComponent the value of app.kubernetes.io/component label for KubeBlocks deployment
	kubeblocksAppComponent = "apps"
	// dataprotectionAppComponent the value of app.kubernetes.io/component label for DataProtection deployment
	dataprotectionAppComponent = "dataprotection"
)

type StopOperator struct{}

func (p *StopOperator) Handle(ctx *UpgradeContext) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := stopKubeBlocksDeploy(ctx, ctx.K8sClient, ctx.Namespace, kubeblocksAppComponent, GetKubeBlocksDeploy); err != nil {
			return err
		}
		return stopKubeBlocksDeploy(ctx, ctx.K8sClient, ctx.Namespace, dataprotectionAppComponent, GetKubeBlocksDeploy)
	})
}
