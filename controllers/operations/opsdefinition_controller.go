/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"fmt"
	"text/template"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/kube-openapi/pkg/validation/spec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// OpsDefinitionReconciler reconciles a OpsDefinition object
type OpsDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=operations.kubeblocks.io,resources=opsdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operations.kubeblocks.io,resources=opsdefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operations.kubeblocks.io,resources=opsdefinitions/finalizers,verbs=update

func (r *OpsDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("opsDefinition", req.NamespacedName),
	}

	opsDef := &opsv1alpha1.OpsDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, opsDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, opsDef, opsDefinitionFinalizerName, func() (*ctrl.Result, error) {
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	if opsDef.Status.ObservedGeneration == opsDef.Generation &&
		opsDef.Status.Phase == opsv1alpha1.AvailablePhase {
		return intctrlutil.Reconciled()
	}

	// check go template of the expression.
	for _, v := range opsDef.Spec.PreConditions {
		if v.Rule == nil {
			continue
		}
		if _, err = template.New("opsDefTemplate").Parse(v.Rule.Expression); err != nil {
			return r.updateStatusUnavailable(reqCtx, opsDef, err)
		}
	}
	if opsDef.Spec.ParametersSchema != nil {
		out := &apiextensions.JSONSchemaProps{}
		if err = apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(opsDef.Spec.ParametersSchema.OpenAPIV3Schema, out, nil); err != nil {
			return r.updateStatusUnavailable(reqCtx, opsDef, err)
		}
		openapiSchema := &spec.Schema{}
		if err = validation.ConvertJSONSchemaPropsWithPostProcess(out, openapiSchema, validation.StripUnsupportedFormatsPostProcess); err != nil {
			return r.updateStatusUnavailable(reqCtx, opsDef, err)
		}
	}

	if err = r.validateComponentInfos(reqCtx, opsDef); err != nil {
		return r.updateStatusUnavailable(reqCtx, opsDef, err)
	}

	statusPatch := client.MergeFrom(opsDef.DeepCopy())
	opsDef.Status.ObservedGeneration = opsDef.Generation
	opsDef.Status.Phase = opsv1alpha1.AvailablePhase
	if err = r.Client.Status().Patch(reqCtx.Ctx, opsDef, statusPatch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, opsDef)
	return intctrlutil.Reconciled()
}

func (r *OpsDefinitionReconciler) updateStatusUnavailable(reqCtx intctrlutil.RequestCtx, opsDef *opsv1alpha1.OpsDefinition, err error) (ctrl.Result, error) {
	statusPatch := client.MergeFrom(opsDef.DeepCopy())
	opsDef.Status.Phase = opsv1alpha1.UnavailablePhase
	opsDef.Status.ObservedGeneration = opsDef.Generation
	opsDef.Status.Message = err.Error()
	if err = r.Client.Status().Patch(reqCtx.Ctx, opsDef, statusPatch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *OpsDefinitionReconciler) validateOpsDefinitionSpec(reqCtx intctrlutil.RequestCtx, opsDef *opsv1alpha1.OpsDefinition) error {
	if err := r.validateComponentInfos(reqCtx, opsDef); err != nil {
		return err
	}
	return nil
}

func (r *OpsDefinitionReconciler) validateComponentInfos(reqCtx intctrlutil.RequestCtx, opsDef *opsv1alpha1.OpsDefinition) error {
	compDefCache := make(map[string]*appsv1.ComponentDefinition)

	for _, compInfo := range opsDef.Spec.ComponentInfos {
		if compInfo.ServiceName != "" {
			compDef, err := r.getComponentDefinition(reqCtx, compInfo.ComponentDefinitionName, compDefCache)
			if err != nil {
				return fmt.Errorf("failed to get componentDefinition %s: %w", compInfo.ComponentDefinitionName, err)
			}

			if !r.isServiceNameValid(compDef, compInfo.ServiceName) {
				return fmt.Errorf("serviceName %s not found in componentDefinition %s services", compInfo.ServiceName, compInfo.ComponentDefinitionName)
			}
		}

		for _, imageMapping := range compInfo.ImageMappings {
			if err := r.validateImageMappingContainers(opsDef.Spec.Actions, imageMapping, compInfo.ComponentDefinitionName); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *OpsDefinitionReconciler) getComponentDefinition(reqCtx intctrlutil.RequestCtx, compDefName string, cache map[string]*appsv1.ComponentDefinition) (*appsv1.ComponentDefinition, error) {
	if compDef, ok := cache[compDefName]; ok {
		return compDef, nil
	}

	compDef := &appsv1.ComponentDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: compDefName}, compDef); err != nil {
		return nil, err
	}

	cache[compDefName] = compDef
	return compDef, nil
}

func (r *OpsDefinitionReconciler) isServiceNameValid(compDef *appsv1.ComponentDefinition, serviceName string) bool {
	for _, svc := range compDef.Spec.Services {
		if svc.Name == serviceName {
			return true
		}
	}
	return false
}

func (r *OpsDefinitionReconciler) validateImageMappingContainers(actions []opsv1alpha1.OpsAction, imageMapping opsv1alpha1.ImageMappings, compDefName string) error {
	allowedContainers := make(map[string]bool)
	for _, action := range actions {
		if action.Workload != nil && action.Workload.PodSpec.Containers != nil {
			for _, container := range action.Workload.PodSpec.Containers {
				allowedContainers[container.Name] = true
			}
		}
	}

	for containerName := range imageMapping.Images {
		if !allowedContainers[containerName] {
			return fmt.Errorf("container %s in imageMappings not found in workload actions for component %s", containerName, compDefName)
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpsDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&opsv1alpha1.OpsDefinition{}).
		Complete(r)
}
