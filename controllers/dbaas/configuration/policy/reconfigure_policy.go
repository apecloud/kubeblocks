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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ExecStatus int

const (
	ES_None ExecStatus = iota
	ES_Retry
	ES_Failed
	ES_NotSpport
)

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

type NoneExecPolicy struct{}

func (receiver NoneExecPolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	return ES_None, nil
}

func NewReconfigurePolicy(tpl *dbaasv1alpha1.ConfigurationTemplateSpec, cfg *cfgcore.ConfigDiffInformation) (ReconfigurePolicy, error) {
	var (
		dynamicUpdate = true
		params        []string
	)

	if !cfg.IsModify {
		// not exec here
		return nil, cfgcore.MakeError("cfg not modify. [%v]", cfg)
	}

	dynamicUpdate, err := IsUpdateDynamicParameters(params, tpl, cfg)
	if err != nil {
		return nil, err
	}

	// TODO(zt) support rolling policy
	if dynamicUpdate {
		return NoneExecPolicy{}, nil
	} else {
		return &SimplePolicy{dbaasv1alpha1.Stateful}, nil
	}
}
