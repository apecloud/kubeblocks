/*
Copyright ApeCloud Inc.

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

package policy

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var (
	// lazy create grpc connection
	newGRPCConn = func(addr string) (*grpc.ClientConn, error) {
		return grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
)

type RollingUpgradePolicy struct {
}

func init() {
	RegisterPolicy(dbaasv1alpha1.RollingPolicy, &RollingUpgradePolicy{})
}

func (r *RollingUpgradePolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	return performRollingUpgrade(params, GetConsensusRollingUpgradeFuncs())
}

func (r *RollingUpgradePolicy) GetPolicyName() string {
	return string(dbaasv1alpha1.RollingPolicy)
}

func performRollingUpgrade(params ReconfigureParams, funcs RollingUpgradeFuncs) (ExecStatus, error) {
	// TODO(zt) rolling kill container
	pods, err := funcs.GetPodsFunc(params)
	if err != nil {
		return ESFailed, err
	}

	// TODO select pod for start
	for i := len(pods); i > 0; i-- {
		pod := &pods[i-1]
		if err := funcs.RestartContainerFunc(pod, params.ContainerName, newGRPCConn); err != nil {
			return ESFailed, err
		}
	}

	return ESNone, nil
}
