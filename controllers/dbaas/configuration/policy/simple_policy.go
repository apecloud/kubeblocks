/*
Copyright 2022.

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
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

func init() {
	RegisterPolicy(dbaasv1alpha1.NormalPolicy, &SimplePolicy{})
}

type SimplePolicy struct {
	componentType dbaasv1alpha1.ComponentType
}

func (s *SimplePolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	params.Ctx.Log.V(1).Info("simple policy begin....")

	switch s.componentType {
	case dbaasv1alpha1.Stateful:
		// process sts
	case dbaasv1alpha1.Consensus:
		// process consensus
	case dbaasv1alpha1.Stateless:
		// process deployment
	default:
		return ES_NotSpport, cfgcore.MakeError("not support component type:[%s]", s.componentType)
	}

	return ES_None, nil
}
