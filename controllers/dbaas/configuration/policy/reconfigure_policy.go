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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ExecStatus int

const (
	ESNone ExecStatus = iota
	ESRetry
	ESFailed
	ESNotSupport
)

func init() {
	RegisterPolicy(dbaasv1alpha1.AutoReload, &AutoReloadPolicy{})
}

type ReconfigureParams struct {
	Restart bool
	TplName string
	Meta    *cfgcore.ConfigDiffInformation
	Cfg     *corev1.ConfigMap
	Tpl     *dbaasv1alpha1.ConfigurationTemplateSpec

	ContainerName    []string
	Client           client.Client
	Ctx              intctrlutil.RequestCtx
	Cluster          *dbaasv1alpha1.Cluster
	ClusterComponent *dbaasv1alpha1.ClusterComponent
	Component        *dbaasv1alpha1.ClusterDefinitionComponent
	ComponentUnits   []appv1.StatefulSet
}

func (param *ReconfigureParams) ComponentType() dbaasv1alpha1.ComponentType {
	return param.Component.ComponentType
}

func (param *ReconfigureParams) GetConfigKey() string {
	for _, tpl := range param.Component.ConfigSpec.ConfigTemplateRefs {
		if tpl.ConfigMapTplRef == param.TplName {
			return tpl.VolumeName
		}
	}
	return ""
}

func (param *ReconfigureParams) GetModifyVersion() string {
	hash, err := cfgcore.ComputeHash(param.Cfg.Data)
	if err != nil {
		param.Ctx.Log.Error(err, "failed to cal configuration version!")
		return ""
	}

	return hash
}

type ReconfigurePolicy interface {
	Upgrade(params ReconfigureParams) (ExecStatus, error)
	GetPolicyName() string
}

var upgradePolicyMap = map[dbaasv1alpha1.UpgradePolicy]ReconfigurePolicy{}

func RegisterPolicy(policy dbaasv1alpha1.UpgradePolicy, action ReconfigurePolicy) {
	upgradePolicyMap[policy] = action
}

type AutoReloadPolicy struct{}

func (receiver AutoReloadPolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	_ = params
	return ESNone, nil
}

func (receiver AutoReloadPolicy) GetPolicyName() string {
	return string(dbaasv1alpha1.AutoReload)
}

func NewReconfigurePolicy(tpl *dbaasv1alpha1.ConfigurationTemplateSpec, cfg *cfgcore.ConfigDiffInformation, policy dbaasv1alpha1.UpgradePolicy, restart bool) (ReconfigurePolicy, error) {
	if !cfg.IsModify {
		// not exec here
		return nil, cfgcore.MakeError("cfg not modify. [%v]", cfg)
	}

	actionType := policy
	if !restart {
		if dynamicUpdate, err := isUpdateDynamicParameters(tpl, cfg); err != nil {
			return nil, err
		} else if dynamicUpdate {
			actionType = dbaasv1alpha1.AutoReload
		}
	}

	if action, ok := upgradePolicyMap[actionType]; ok {
		return action, nil
	}
	return nil, cfgcore.MakeError("not support upgrade policy:[%s]", actionType)
}
