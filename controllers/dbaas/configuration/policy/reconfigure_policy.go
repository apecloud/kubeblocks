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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExecStatus int

const (
	ES_None ExecStatus = iota
	ES_Retry
	ES_Failed
	ES_NotSpport
)

func init() {
	RegisterPolicy(dbaasv1alpha1.AutoReload, &AutoReloadPolicy{})
}

type ReconfigureParams struct {
	Meta *cfgcore.ConfigDiffInformation
	Cfg  *corev1.ConfigMap
	Tpl  *dbaasv1alpha1.ConfigurationTemplateSpec

	Client     client.Client
	Ctx        intctrlutil.RequestCtx
	Cluster    *dbaasv1alpha1.Cluster
	Components []appv1.StatefulSet
}

type ReconfigurePolicy interface {
	Upgrade(params ReconfigureParams) (ExecStatus, error)
}

var upgradePolicyMap map[dbaasv1alpha1.UpgradePolicy]ReconfigurePolicy

func RegisterPolicy(policy dbaasv1alpha1.UpgradePolicy, action ReconfigurePolicy) {
	upgradePolicyMap[policy] = action
}

type AutoReloadPolicy struct{}

func (receiver AutoReloadPolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	_ = params
	return ES_None, nil
}

func NewReconfigurePolicy(tpl *dbaasv1alpha1.ConfigurationTemplateSpec, cfg *cfgcore.ConfigDiffInformation, policy dbaasv1alpha1.UpgradePolicy) (ReconfigurePolicy, error) {
	if !cfg.IsModify {
		// not exec here
		return nil, cfgcore.MakeError("cfg not modify. [%v]", cfg)
	}

	dynamicUpdate, err := IsUpdateDynamicParameters(tpl, cfg)
	if err != nil {
		return nil, err
	}

	actionType := policy
	if dynamicUpdate {
		actionType = dbaasv1alpha1.AutoReload
	}

	if action, ok := upgradePolicyMap[actionType]; ok {
		return action, nil
	}
	return nil, cfgcore.MakeError("not support upgrade policy:[%s]", actionType)
}
