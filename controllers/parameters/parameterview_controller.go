/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
)

const (
	parameterViewReadyCondition = "Ready"
)

// ParameterViewReconciler reconciles a ParameterView object.
type ParameterViewReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameterviews,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameterviews/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameterviews/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *ParameterViewReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("ParameterViewReconciler").
			WithValues("Namespace", req.Namespace, "ParameterView", req.Name),
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, view); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: view.Namespace,
		Name:      view.Spec.ParameterRef.Name,
	}, compParam); err != nil {
		return r.markInvalid(reqCtx, view, fmt.Sprintf("referenced ComponentParameter not found: %s", view.Spec.ParameterRef.Name))
	}

	source, err := r.resolveSource(reqCtx.Ctx, compParam, view)
	if err != nil {
		return r.markInvalid(reqCtx, view, err.Error())
	}

	if view.Spec.Content.Type != "" && view.Spec.Content.Type != parametersv1alpha1.PlainTextParameterViewContentType {
		return r.markInvalid(reqCtx, view, fmt.Sprintf("content type %q is not supported in phase 1", view.Spec.Content.Type))
	}

	specChanged := false
	specPatch := client.MergeFrom(view.DeepCopy())
	if view.Spec.Content.Type == "" {
		view.Spec.Content.Type = parametersv1alpha1.PlainTextParameterViewContentType
		specChanged = true
	}
	if view.Spec.FileFormat == "" {
		view.Spec.FileFormat = source.fileFormat
		specChanged = true
	} else if view.Spec.FileFormat != source.fileFormat {
		return r.markInvalid(reqCtx, view, fmt.Sprintf("fileFormat %q does not match source file format %q", view.Spec.FileFormat, source.fileFormat))
	}

	if view.Spec.SourceGeneration == 0 || view.Spec.Content.Text == "" {
		if view.Spec.SourceGeneration != source.generation {
			view.Spec.SourceGeneration = source.generation
			specChanged = true
		}
		if view.Spec.ContentHash != source.hash {
			view.Spec.ContentHash = source.hash
			specChanged = true
		}
		if view.Spec.Content.Text != source.content {
			view.Spec.Content.Text = source.content
			specChanged = true
		}
	} else if view.Spec.ContentHash != "" && view.Spec.ContentHash != source.hash {
		if view.Spec.Content.Text != source.content {
			if specChanged {
				if err := r.Client.Patch(reqCtx.Ctx, view, specPatch); err != nil {
					return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view spec")
				}
			}
			return r.markConflict(reqCtx, view, fmt.Sprintf("source content changed for %s/%s", view.Spec.TemplateName, view.Spec.FileName))
		}
		view.Spec.SourceGeneration = source.generation
		view.Spec.ContentHash = source.hash
		specChanged = true
	}

	if specChanged {
		if err := r.Client.Patch(reqCtx.Ctx, view, specPatch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view spec")
		}
	}

	return r.markReady(reqCtx, view)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ParameterViewReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.ParameterView{}).
		Complete(r)
}

type parameterViewSource struct {
	content    string
	hash       string
	fileFormat parametersv1alpha1.CfgFileFormat
	generation int64
}

func (r *ParameterViewReconciler) resolveSource(ctx context.Context, compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView) (*parameterViewSource, error) {
	item := parameters.GetConfigTemplateItem(&compParam.Spec, view.Spec.TemplateName)
	if item == nil {
		return nil, fmt.Errorf("template not found in ComponentParameter: %s", view.Spec.TemplateName)
	}

	fileFormat, err := r.resolveFileFormat(ctx, compParam, view)
	if err != nil {
		return nil, err
	}

	content, err := r.resolveContent(ctx, compParam, item, view.Spec.FileName)
	if err != nil {
		return nil, err
	}

	return &parameterViewSource{
		content:    content,
		hash:       hashContent(content),
		fileFormat: fileFormat,
		generation: compParam.Generation,
	}, nil
}

func (r *ParameterViewReconciler) resolveFileFormat(ctx context.Context, compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView) (parametersv1alpha1.CfgFileFormat, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: ctrl.Request{NamespacedName: client.ObjectKeyFromObject(compParam)},
	}
	fetchTask, err := prepareReconcileTask(reqCtx, r.Client, compParam)
	if err != nil {
		return "", err
	}
	configDescs, _, err := parameters.ResolveCmpdParametersDefs(ctx, r.Client, fetchTask.ComponentDefObj)
	if err != nil {
		return "", err
	}
	for _, desc := range configDescs {
		if desc.TemplateName == view.Spec.TemplateName && desc.Name == view.Spec.FileName && desc.FileFormatConfig != nil {
			return desc.FileFormatConfig.Format, nil
		}
	}
	return "", fmt.Errorf("file format not found for %s/%s", view.Spec.TemplateName, view.Spec.FileName)
}

func (r *ParameterViewReconciler) resolveContent(ctx context.Context, compParam *parametersv1alpha1.ComponentParameter, item *parametersv1alpha1.ConfigTemplateItemDetail, fileName string) (string, error) {
	configMaps, err := resolveComponentRefConfigMap(ctx, r.Client, compParam.Namespace, compParam.Spec.ClusterName, compParam.Spec.ComponentName)
	if err == nil {
		if cm, ok := configMaps[item.Name]; ok && cm != nil {
			if content, ok := cm.Data[fileName]; ok {
				return content, nil
			}
		}
	}

	if item.ConfigFileParams != nil {
		if fileParams, ok := item.ConfigFileParams[fileName]; ok && fileParams.Content != nil {
			return *fileParams.Content, nil
		}
	}

	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: ctrl.Request{NamespacedName: client.ObjectKeyFromObject(compParam)},
	}
	fetchTask, err := prepareReconcileTask(reqCtx, r.Client, compParam)
	if err != nil {
		return "", err
	}
	templates, err := resolveComponentTemplate(ctx, r.Client, fetchTask.ComponentDefObj)
	if err != nil {
		return "", err
	}
	tpl, ok := templates[item.Name]
	if !ok || tpl == nil {
		return "", fmt.Errorf("template source not found for %s", item.Name)
	}
	content, ok := tpl.Data[fileName]
	if !ok {
		return "", fmt.Errorf("file not found in template source: %s", fileName)
	}
	return content, nil
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func (r *ParameterViewReconciler) markReady(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView) (ctrl.Result, error) {
	return r.patchStatus(reqCtx, view, parametersv1alpha1.ParameterViewReadyPhase, "", metav1.ConditionTrue, "Resolved", "parameter view is ready")
}

func (r *ParameterViewReconciler) markConflict(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView, msg string) (ctrl.Result, error) {
	return r.patchStatus(reqCtx, view, parametersv1alpha1.ParameterViewConflictPhase, msg, metav1.ConditionFalse, "SourceChanged", msg)
}

func (r *ParameterViewReconciler) markInvalid(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView, msg string) (ctrl.Result, error) {
	return r.patchStatus(reqCtx, view, parametersv1alpha1.ParameterViewInvalidPhase, msg, metav1.ConditionFalse, "InvalidSpec", msg)
}

func (r *ParameterViewReconciler) patchStatus(
	reqCtx intctrlutil.RequestCtx,
	view *parametersv1alpha1.ParameterView,
	phase parametersv1alpha1.ParameterViewPhase,
	message string,
	conditionStatus metav1.ConditionStatus,
	reason string,
	conditionMessage string,
) (ctrl.Result, error) {
	patch := client.MergeFrom(view.DeepCopy())
	view.Status.ObservedGeneration = view.Generation
	view.Status.Phase = phase
	view.Status.Message = message
	apiMeta.SetStatusCondition(&view.Status.Conditions, metav1.Condition{
		Type:               parameterViewReadyCondition,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            conditionMessage,
		ObservedGeneration: view.Generation,
		LastTransitionTime: metav1.Now(),
	})
	if err := r.Client.Status().Patch(reqCtx.Ctx, view, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view status")
	}
	return intctrlutil.Reconciled()
}
