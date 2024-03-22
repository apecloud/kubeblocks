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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type CustomOpsHandler struct{}

var _ OpsHandler = CustomOpsHandler{}

func init() {
	customBehaviour := OpsBehaviour{
		OpsHandler: CustomOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.CustomType, customBehaviour)
}

// ActionStartedCondition the started condition when handling the stop request.
func (c CustomOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	opsDefName := common.ToCamelCase(opsRes.OpsRequest.Spec.CustomSpec.OpsDefinitionRef)
	return &metav1.Condition{
		Type:               appsv1alpha1.ConditionTypeCustomOperation,
		Status:             metav1.ConditionTrue,
		Reason:             opsDefName + "Starting",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to handle %s on the Cluster: %s", opsDefName, opsRes.OpsRequest.Spec.ClusterRef),
	}, nil
}

func (c CustomOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for stop opsRequest.
func (c CustomOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		oldOpsRequest        = opsRes.OpsRequest.DeepCopy()
		patch                = client.MergeFrom(oldOpsRequest)
		opsRequestPhase      = opsRes.OpsRequest.Status.Phase
		customSpec           = opsRes.OpsRequest.Spec.CustomSpec
		workflowContext      = NewWorkflowContext(reqCtx, cli, opsRes)
		compCount            = len(customSpec.CustomOpsComponents)
		completedActionCount int
		compFailedCount      int
		compCompleteCount    int
	)
	// TODO: support Parallelism
	for _, v := range customSpec.CustomOpsComponents {
		// 1. init component action progress and preCheck if the conditions for executing ops are met.
		passed := c.initCompActionStatusAndPreCheck(reqCtx, cli, opsRes, v)
		if !passed {
			compCompleteCount += 1
			compFailedCount += 1
			continue
		}
		// 2. do workflow
		workflowStatus, err := workflowContext.Run(&v)
		if err != nil {
			return opsRequestPhase, 0, err
		}
		if workflowStatus.IsCompleted {
			compCompleteCount += 1
			if workflowStatus.ExistFailure {
				compFailedCount += 1
			}
		}
		completedActionCount += workflowStatus.CompletedCount
	}
	// sync progress
	opsRes.OpsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedActionCount, len(customSpec.CustomOpsComponents)*len(opsRes.OpsDef.Spec.Actions))
	if !reflect.DeepEqual(opsRes.OpsRequest.Status, oldOpsRequest.Status) {
		if err := cli.Status().Patch(reqCtx.Ctx, opsRes.OpsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	// check if the ops has been finished.
	if compCompleteCount == compCount {
		if compFailedCount == 0 {
			opsRequestPhase = appsv1alpha1.OpsSucceedPhase
		} else {
			opsRequestPhase = appsv1alpha1.OpsFailedPhase
		}
	}
	return opsRequestPhase, 0, nil
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (c CustomOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (c CustomOpsHandler) checkExpression(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rule *appsv1alpha1.Rule,
	compCustomOSpec appsv1alpha1.CustomOpsComponent) error {
	opsSpec := opsRes.OpsRequest.Spec
	if opsSpec.Force {
		return nil
	}
	componentObjName := constant.GenerateClusterComponentName(opsSpec.ClusterRef, compCustomOSpec.ComponentName)
	comp := &appsv1alpha1.Component{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: componentObjName, Namespace: opsRes.OpsRequest.Namespace}, comp); err != nil {
		return err
	}
	params := covertParametersToMap(compCustomOSpec.Parameters)
	// get the built-in objects and covert the json tag
	getBuiltInObjs := func() (map[string]interface{}, error) {
		b, err := json.Marshal(map[string]interface{}{
			"cluster":    opsRes.Cluster,
			"component":  comp,
			"parameters": params,
		})
		if err != nil {
			return nil, err
		}
		data := map[string]interface{}{}
		if err = json.Unmarshal(b, &data); err != nil {
			return nil, err
		}
		return data, nil
	}

	data, err := getBuiltInObjs()
	if err != nil {
		return err
	}
	tmpl, err := template.New("opsDefTemplate").Parse(rule.Expression)
	if err != nil {
		return err
	}
	var buf strings.Builder
	if err = tmpl.Execute(&buf, data); err != nil {
		return err
	}
	if buf.String() == "false" {
		return fmt.Errorf(rule.Message)
	}
	return nil
}

// initCompActionProgressDetails initializes the action's progressDetails and preCheck if the conditions for executing ops are met.
func (c CustomOpsHandler) initCompActionStatusAndPreCheck(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	compCustomSpec appsv1alpha1.CustomOpsComponent) bool {
	if opsRes.OpsRequest.Status.Components == nil {
		opsRes.OpsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{}
	}
	compStatus := opsRes.OpsRequest.Status.Components[compCustomSpec.ComponentName]
	compStatus.Phase = opsRes.Cluster.Status.Components[compCustomSpec.ComponentName].Phase
	if len(compStatus.ProgressDetails) == 0 {
		// 1. do preChecks
		for _, v := range opsRes.OpsDef.Spec.PreConditions {
			if v.Rule != nil {
				if err := c.checkExpression(reqCtx, cli, opsRes, v.Rule, compCustomSpec); err != nil {
					compStatus.PreCheckResult = &appsv1alpha1.PreCheckResult{Pass: false, Message: err.Error()}
					opsRes.OpsRequest.Status.Components[compCustomSpec.ComponentName] = compStatus
					opsRes.Recorder.Event(opsRes.OpsRequest, corev1.EventTypeWarning, "PreCheckFailed", err.Error())
					return false
				}
				compStatus.PreCheckResult = &appsv1alpha1.PreCheckResult{Pass: true}
			}
		}
		// 2. init action progress details
		for i := range opsRes.OpsDef.Spec.Actions {
			compStatus.ProgressDetails = append(compStatus.ProgressDetails, appsv1alpha1.ProgressStatusDetail{
				Status:     appsv1alpha1.PendingProgressStatus,
				ActionName: opsRes.OpsDef.Spec.Actions[i].Name,
			})
		}
		opsRes.OpsRequest.Status.Components[compCustomSpec.ComponentName] = compStatus
	}
	return true
}

func covertParametersToMap(parameters []appsv1alpha1.Parameter) map[string]string {
	params := map[string]string{}
	for _, v := range parameters {
		params[v.Name] = v.Value
	}
	return params
}

// initOpsDefAndValidate inits the opsDefinition to OpsResource and validates if the opsRequest is valid.
func initOpsDefAndValidate(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) error {
	customSpec := opsRes.OpsRequest.Spec.CustomSpec
	if customSpec == nil {
		return intctrlutil.NewFatalError("spec.customSpec can not be empty if opsType is Custom.")
	}
	opsDef := &appsv1alpha1.OpsDefinition{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: customSpec.OpsDefinitionRef}, opsDef); err != nil {
		return err
	}
	opsRes.OpsDef = opsDef
	// 1. validate OpenApV3Schema
	parametersSchema := opsDef.Spec.ParametersSchema
	for _, v := range customSpec.CustomOpsComponents {
		// covert to type map[string]interface{}
		params, err := common.CoverStringToInterfaceBySchemaType(parametersSchema.OpenAPIV3Schema, covertParametersToMap(v.Parameters))
		if err != nil {
			return err
		}
		if parametersSchema != nil && parametersSchema.OpenAPIV3Schema != nil {
			if err = common.ValidateDataWithSchema(parametersSchema.OpenAPIV3Schema, params); err != nil {
				return err
			}
		}

		// 2. validate component and componentDef
		if len(opsRes.OpsDef.Spec.ComponentDefinitionRefs) > 0 {
			comp := opsRes.Cluster.Spec.GetComponentByName(v.ComponentName)
			if comp == nil {
				return intctrlutil.NewNotFound(`can not found component "%s" in cluster "%s"`, v.ComponentName, opsRes.Cluster.Name)
			}
			compDef, err := component.GetCompDefinition(reqCtx, cli, opsRes.Cluster, v.ComponentName)
			if err != nil {
				return err
			}
			if len(opsDef.Spec.ComponentDefinitionRefs) > 0 {
				var componentDefMatched bool
				for _, v := range opsDef.Spec.ComponentDefinitionRefs {
					if v.Name == compDef.Name {
						componentDefMatched = true
						break
					}
				}
				if !componentDefMatched {
					return intctrlutil.NewFatalError(fmt.Sprintf(`not supported componnet definition "%s"`, compDef.Name))
				}
			}
		}
	}
	return nil
}
