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

package operations

import (
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	var (
		reAction   = reconfigureAction{}
		opsManager = GetOpsManager()
	)

	reconfigureBehaviour := &OpsBehaviour{
		FromClusterPhases: []dbaasv1alpha1.Phase{
			// all phase
			dbaasv1alpha1.RunningPhase,
			dbaasv1alpha1.FailedPhase,
			dbaasv1alpha1.AbnormalPhase,
		},
		ToClusterPhase:         dbaasv1alpha1.UpdatingPhase,
		Action:                 reAction.reconfigure,
		ActionStartedCondition: dbaasv1alpha1.NewReconfigureCondition,
		ReconcileAction:        reAction.ReconcileAction,
	}
	cfgcore.RegisterConfigEventHandler("ops_status_reconfigure", &reAction)
	opsManager.RegisterOps(dbaasv1alpha1.ReconfigureType, reconfigureBehaviour)
}

type reconfigureAction struct {
}

func (r *reconfigureAction) Handle(eventContext cfgcore.ConfigEventContext, lastOpsRequest string, phase dbaasv1alpha1.Phase, err error) error {
	var (
		opsRequest = &dbaasv1alpha1.OpsRequest{}
		cm         = eventContext.Cfg
		cli        = eventContext.Client
		ctx        = eventContext.ReqCtx.Ctx
		log        = eventContext.ReqCtx.Log
	)

	opsRes := &OpsResource{
		Ctx:        ctx,
		OpsRequest: opsRequest,
		Recorder:   eventContext.ReqCtx.Recorder,
		Client:     cli,
		Cluster:    eventContext.Cluster,
	}

	if len(lastOpsRequest) == 0 {
		return nil
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Name:      lastOpsRequest,
		Namespace: cm.Namespace,
	}, opsRequest); err != nil {
		return err
	}
	if !isReconfigureOpsRequest(opsRequest) {
		log.Info(fmt.Sprintf("ops request not reconfigure ops, and pass, cr: %v", client.ObjectKeyFromObject(opsRequest)))
		return nil
	}

	switch phase {
	case dbaasv1alpha1.SucceedPhase:
		return PatchOpsStatus(opsRes, dbaasv1alpha1.RunningPhase, dbaasv1alpha1.NewSucceedCondition(opsRequest))
	case dbaasv1alpha1.FailedPhase:
		return PatchOpsStatus(opsRes, dbaasv1alpha1.RunningPhase, dbaasv1alpha1.NewFailedCondition(opsRequest, err))
	default:
		return PatchOpsStatus(opsRes, dbaasv1alpha1.RunningPhase, dbaasv1alpha1.NewReconfigureRunningCondition(opsRequest, dbaasv1alpha1.ReasonReconfigureRunning))
	}
}

func (r reconfigureAction) ReconcileAction(opsRes *OpsResource) (dbaasv1alpha1.Phase, time.Duration, error) {
	status := opsRes.OpsRequest.Status
	if len(status.Conditions) == 0 {
		return status.Phase, 30 * time.Second, nil
	}
	condition := status.Conditions[len(status.Conditions)-1]
	if condition.Type == dbaasv1alpha1.ConditionTypeSucceed && condition.Status == metav1.ConditionTrue {
		return dbaasv1alpha1.SucceedPhase, 0, nil
	}
	if condition.Type == dbaasv1alpha1.ConditionTypeFailed && condition.Status == metav1.ConditionFalse {
		return dbaasv1alpha1.FailedPhase, 0, nil
	}
	return dbaasv1alpha1.RunningPhase, 30 * time.Second, nil
}

func isReconfigureOpsRequest(request *dbaasv1alpha1.OpsRequest) bool {
	return request.Spec.Type == dbaasv1alpha1.ReconfigureType && request.Spec.Reconfigure != nil
}

func (r *reconfigureAction) reconfigure(resource *OpsResource) error {
	if !isReconfigureOpsRequest(resource.OpsRequest) {
		return cfgcore.MakeError("invalid reconfigure params.")
	}

	var (
		spec              = &resource.OpsRequest.Spec
		clusterName       = spec.ClusterRef
		componentName     = spec.Reconfigure.ComponentName
		cluster           = resource.Cluster
		clusterDefinition = &dbaasv1alpha1.ClusterDefinition{}
		appVersion        = &dbaasv1alpha1.ClusterVersion{}
	)

	if err := resource.Client.Get(resource.Ctx, client.ObjectKey{
		Name:      cluster.Spec.ClusterDefRef,
		Namespace: cluster.Namespace,
	}, clusterDefinition); err != nil {
		return cfgcore.WrapError(err, "failed to get clusterdefinition[%s]", cluster.Spec.ClusterDefRef)
	}

	if err := resource.Client.Get(resource.Ctx, client.ObjectKey{
		Name:      cluster.Spec.ClusterVersionRef,
		Namespace: cluster.Namespace,
	}, appVersion); err != nil {
		return cfgcore.WrapError(err, "failed to get appversion[%s]", cluster.Spec.ClusterVersionRef)
	}

	tpls, err := getConfigTemplatesFromComponent(
		cluster.Spec.Components,
		clusterDefinition.Spec.Components,
		appVersion.Spec.Components,
		componentName)
	if err != nil {
		return cfgcore.WrapError(err, "failed to get config template[%s]", componentName)
	}
	return r.performUpgrade(clusterName, componentName, spec.Reconfigure, resource, tpls)
}

func (r *reconfigureAction) performUpgrade(clusterName, componentName string,
	reconfigure *dbaasv1alpha1.UpgradeConfiguration,
	resource *OpsResource,
	tpls []dbaasv1alpha1.ConfigTemplate) error {
	findTpl := func(tplName string) *dbaasv1alpha1.ConfigTemplate {
		if len(tplName) == 0 && len(tpls) == 1 {
			return &tpls[0]
		}
		for _, tpl := range tpls {
			if tpl.Name == tplName {
				return &tpl
			}
		}
		return nil
	}

	// Update params to configmap
	for _, config := range reconfigure.Configurations {
		tpl := findTpl(config.Name)
		if tpl == nil {
			return processMergedFailed(resource, true,
				cfgcore.MakeError("failed to reconfigure, not exist config[%s], all configs: %v",
					config.Name, tpls))
		}
		if len(tpl.ConfigConstraintRef) == 0 {
			return processMergedFailed(resource, true,
				cfgcore.MakeError("current tpl not support reconfigure, tpl: %v", tpl))
		}
		if failed, err := updateCfgParams(config, *tpl, client.ObjectKey{
			Name:      cfgcore.GetComponentCMName(clusterName, componentName, *tpl),
			Namespace: resource.Cluster.Namespace,
		}, resource.Ctx, resource.Client, resource.OpsRequest.Name); err != nil {
			return processMergedFailed(resource, failed, err)
		}
		// merged successfully
		if err := PatchOpsStatus(resource, dbaasv1alpha1.RunningPhase,
			dbaasv1alpha1.NewReconfigureRunningCondition(resource.OpsRequest, dbaasv1alpha1.ReasonReconfigureMerged)); err != nil {
			return err
		}
	}
	return nil
}

func processMergedFailed(resource *OpsResource, isInvalid bool, err error) error {
	if !isInvalid {
		return cfgcore.WrapError(err, "failed to update param!")
	}

	// if failed to validate configure
	if err := PatchOpsStatus(resource, dbaasv1alpha1.FailedPhase,
		dbaasv1alpha1.NewFailedCondition(resource.OpsRequest, err)); err != nil {
		return err
	}
	return nil
}
