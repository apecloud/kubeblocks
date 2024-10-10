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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
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
	opsMgr.RegisterOps(opsv1alpha1.CustomType, customBehaviour)
}

// ActionStartedCondition the started condition when handling the stop request.
func (c CustomOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	opsDefName := common.ToCamelCase(opsRes.OpsRequest.Spec.CustomOps.OpsDefinitionName)
	return &metav1.Condition{
		Type:               opsv1alpha1.ConditionTypeCustomOperation,
		Status:             metav1.ConditionTrue,
		Reason:             opsDefName + "Starting",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to handle %s on the Cluster: %s", opsDefName, opsRes.OpsRequest.Spec.GetClusterName()),
	}, nil
}

func (c CustomOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for stop opsRequest.
func (c CustomOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		oldOpsRequest        = opsRes.OpsRequest.DeepCopy()
		opsRequestPhase      = opsRes.OpsRequest.Status.Phase
		customSpec           = opsRes.OpsRequest.Spec.CustomOps
		workflowContext      = NewWorkflowContext(reqCtx, cli, opsRes)
		compCount            = len(customSpec.CustomOpsComponents)
		completedActionCount int
		compFailedCount      int
		compCompleteCount    int
	)
	// TODO: support Parallelism
	for _, v := range customSpec.CustomOpsComponents {
		// 1. init component action progress and preCheck if the conditions for executing ops are met.
		requeueAfter, passed := c.initCompActionStatusAndPreCheck(reqCtx, cli, opsRes, v)
		if requeueAfter != 0 {
			return opsRequestPhase, requeueAfter, nil
		}
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
	if err := syncProgressToOpsRequest(reqCtx, cli, opsRes, oldOpsRequest, completedActionCount, compCount*len(opsRes.OpsDef.Spec.Actions)); err != nil {
		return opsRequestPhase, 0, err
	}
	// check if the ops has been finished.
	if compCompleteCount != compCount {
		return opsRequestPhase, 0, nil
	}
	if compFailedCount == 0 {
		return opsv1alpha1.OpsSucceedPhase, 0, nil
	}
	return opsv1alpha1.OpsFailedPhase, 0, nil
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (c CustomOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (c CustomOpsHandler) listComponents(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1.Cluster,
	componentName string) ([]appsv1.Component, error) {
	if cluster.Spec.GetComponentByName(componentName) != nil {
		comp, err := component.GetComponentByName(reqCtx.Ctx, cli, cluster.Namespace,
			constant.GenerateClusterComponentName(cluster.Name, componentName))
		if err != nil {
			return nil, err
		}
		return []appsv1.Component{*comp}, nil
	}
	return intctrlutil.ListShardingComponents(reqCtx.Ctx, cli, cluster, componentName)
}

func (c CustomOpsHandler) checkExpression(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rule *opsv1alpha1.Rule,
	compCustomItem opsv1alpha1.CustomOpsComponent) error {
	opsSpec := opsRes.OpsRequest.Spec
	if opsSpec.Force {
		return nil
	}
	comps, err := c.listComponents(reqCtx, cli, opsRes.Cluster, compCustomItem.ComponentName)
	if err != nil {
		return err
	}
	for _, comp := range comps {
		params, err := covertParametersToMap(reqCtx.Ctx, cli, compCustomItem.Parameters, opsRes.OpsRequest.Namespace)
		if err != nil {
			return err
		}
		// get the built-in objects and covert the json tag
		getBuiltInObjs := func() (map[string]interface{}, error) {
			b, err := json.Marshal(map[string]interface{}{
				"cluster":    opsRes.Cluster,
				"component":  &comp,
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
			if needWaitPreConditionDeadline(opsRes.OpsRequest) {
				return intctrlutil.NewRequeueError(time.Second, rule.Message)
			}
			return fmt.Errorf(rule.Message)
		}
	}
	return nil
}

// initCompActionProgressDetails initializes the action's progressDetails and preCheck if the conditions for executing ops are met.
func (c CustomOpsHandler) initCompActionStatusAndPreCheck(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	compCustomItem opsv1alpha1.CustomOpsComponent) (time.Duration, bool) {
	if opsRes.OpsRequest.Status.Components == nil {
		opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	compStatus := opsRes.OpsRequest.Status.Components[compCustomItem.ComponentName]
	compStatus.Phase = opsRes.Cluster.Status.Components[compCustomItem.ComponentName].Phase
	if len(compStatus.ProgressDetails) == 0 {
		// 1. do preChecks
		for _, v := range opsRes.OpsDef.Spec.PreConditions {
			if v.Rule != nil {
				if err := c.checkExpression(reqCtx, cli, opsRes, v.Rule, compCustomItem); err != nil {
					compStatus.PreCheckResult = &opsv1alpha1.PreCheckResult{Pass: false, Message: err.Error()}
					opsRes.OpsRequest.Status.Components[compCustomItem.ComponentName] = compStatus
					opsRes.Recorder.Event(opsRes.OpsRequest, corev1.EventTypeWarning, "PreCheckFailed", err.Error())
					if intctrlutil.IsRequeueError(err) {
						return err.(intctrlutil.RequeueError).RequeueAfter(), false
					}
					return 0, false
				}
				compStatus.PreCheckResult = &opsv1alpha1.PreCheckResult{Pass: true}
			}
		}
		// 2. init action progress details
		for i := range opsRes.OpsDef.Spec.Actions {
			compStatus.ProgressDetails = append(compStatus.ProgressDetails, opsv1alpha1.ProgressStatusDetail{
				Status:     opsv1alpha1.PendingProgressStatus,
				ActionName: opsRes.OpsDef.Spec.Actions[i].Name,
			})
		}
		opsRes.OpsRequest.Status.Components[compCustomItem.ComponentName] = compStatus
	}
	return 0, true
}

func covertParametersToMap(ctx context.Context,
	cli client.Client,
	parameters []opsv1alpha1.Parameter,
	opsNamespace string) (map[string]string, error) {
	params := map[string]string{}
	var err error
	for _, v := range parameters {
		value := v.Value
		if value == "" && v.ValueFrom != nil {
			if value, err = resolveParameterValue(ctx, cli, v.ValueFrom, opsNamespace); err != nil {
				return nil, err
			}
		}
		params[v.Name] = value
	}
	return params, nil
}

func resolveParameterValue(ctx context.Context,
	cli client.Client,
	valueFrom *opsv1alpha1.ParameterSource,
	opsNamespace string) (string, error) {
	resolveObjectKey := func(nativeObjRef corev1.LocalObjectReference, object client.Object, getValue func(obj client.Object) (string, error)) (string, error) {
		if err := cli.Get(ctx, client.ObjectKey{Name: nativeObjRef.Name, Namespace: opsNamespace}, object); err != nil {
			return "", err
		}
		return getValue(object)
	}
	switch {
	case valueFrom.ConfigMapKeyRef != nil:
		return resolveObjectKey(valueFrom.ConfigMapKeyRef.LocalObjectReference, &corev1.ConfigMap{}, func(obj client.Object) (string, error) {
			cm := obj.(*corev1.ConfigMap)
			key := valueFrom.ConfigMapKeyRef.Key
			if v, ok := cm.Data[key]; ok {
				return v, nil
			}
			if v, ok := cm.BinaryData[key]; ok {
				return string(v), nil
			}
			return "", intctrlutil.NewFatalError(fmt.Sprintf(`key "%s" is not found in the ConfigMap "%s"`, key, valueFrom.ConfigMapKeyRef.Name))
		})
	case valueFrom.SecretKeyRef != nil:
		return resolveObjectKey(valueFrom.SecretKeyRef.LocalObjectReference, &corev1.Secret{}, func(obj client.Object) (string, error) {
			secret := obj.(*corev1.Secret)
			key := valueFrom.SecretKeyRef.Key
			if v, ok := secret.Data[key]; ok {
				return string(v), nil
			}
			if v, ok := secret.StringData[key]; ok {
				return v, nil
			}
			return "", intctrlutil.NewFatalError(fmt.Sprintf(`key "%s" is not found in the Secret "%s"`, key, valueFrom.SecretKeyRef.Name))
		})
	}
	return "", nil
}

// initOpsDefAndValidate inits the opsDefinition to OpsResource and validates if the opsRequest is valid.
func initOpsDefAndValidate(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) error {
	customSpec := opsRes.OpsRequest.Spec.CustomOps
	if customSpec == nil {
		return intctrlutil.NewFatalError("spec.custom can not be empty if opsType is Custom.")
	}
	opsDef := &opsv1alpha1.OpsDefinition{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: customSpec.OpsDefinitionName}, opsDef); err != nil {
		return err
	}
	opsRes.OpsDef = opsDef
	// 1. validate OpenApV3Schema
	parametersSchema := opsDef.Spec.ParametersSchema
	if parametersSchema == nil {
		return nil
	}
	for _, v := range customSpec.CustomOpsComponents {
		paramsMap, err := covertParametersToMap(reqCtx.Ctx, cli, v.Parameters, opsRes.OpsRequest.Namespace)
		if err != nil {
			return err
		}
		// covert to type map[string]interface{}
		params, err := common.CoverStringToInterfaceBySchemaType(parametersSchema.OpenAPIV3Schema, paramsMap)
		if err != nil {
			return intctrlutil.NewFatalError(err.Error())
		}
		if parametersSchema != nil && parametersSchema.OpenAPIV3Schema != nil {
			if err = common.ValidateDataWithSchema(parametersSchema.OpenAPIV3Schema, params); err != nil {
				return intctrlutil.NewFatalError(err.Error())
			}
		}

		// 2. validate component and componentDef
		if len(opsRes.OpsDef.Spec.ComponentInfos) > 0 {
			// get component definition
			compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, v.ComponentName)
			compDef, err := component.GetCompDefByName(reqCtx.Ctx, cli, compSpec.ComponentDef)
			if err != nil {
				return err
			}
			if len(opsDef.Spec.ComponentInfos) > 0 {
				var componentDefMatched bool
				for _, c := range opsDef.Spec.ComponentInfos {
					if component.CompDefMatched(compDef.Name, c.ComponentDefinitionName) {
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
