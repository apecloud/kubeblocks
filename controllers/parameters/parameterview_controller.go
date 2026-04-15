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
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
)

const (
	parameterViewSyncedCondition = "Synced"

	parameterViewSubmissionLimit = 10

	parameterViewReasonResolved                  = "Resolved"
	parameterViewReasonReferenceNotFound         = "ReferenceNotFound"
	parameterViewReasonTemplateNotFound          = "TemplateNotFound"
	parameterViewReasonFileNotFound              = "FileNotFound"
	parameterViewReasonUnsupportedContentType    = "UnsupportedContentType"
	parameterViewReasonReadOnly                  = "ReadOnly"
	parameterViewReasonDraftOutdated             = "DraftOutdated"
	parameterViewReasonDiffFailed                = "DiffFailed"
	parameterViewReasonSchemaValidationFailed    = "SchemaValidationFailed"
	parameterViewReasonInvalidMarkerSyntax       = "InvalidMarkerSyntax"
	parameterViewReasonUnsupportedContentChanges = "UnsupportedContentChanges"
	parameterViewReasonApplying                  = "Applying"

	parameterViewSubmissionReasonProcessing        = "Processing"
	parameterViewSubmissionReasonMergeFailed       = "MergeFailed"
	parameterViewSubmissionReasonReconfigureFailed = "ReconfigureFailed"
	parameterViewSubmissionReasonSucceeded         = "Succeeded"

	parameterViewParameterRefLabelKey = "parameters.kubeblocks.io/parameter-ref"
	parameterViewTemplateLabelKey     = "parameters.kubeblocks.io/template-name"
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
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

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
		return r.markInvalid(reqCtx, view, parameterViewReasonReferenceNotFound,
			fmt.Sprintf("referenced ComponentParameter not found: %s", view.Spec.ParameterRef.Name), nil)
	}

	source, err := r.resolveSource(reqCtx.Ctx, compParam, view)
	if err != nil {
		return r.markInvalidForSourceError(reqCtx, view, err)
	}

	if view.Spec.Content.Type != "" &&
		view.Spec.Content.Type != parametersv1alpha1.PlainTextParameterViewContentType &&
		view.Spec.Content.Type != parametersv1alpha1.MarkerLineParameterViewContentType {
		return r.markInvalid(reqCtx, view, parameterViewReasonUnsupportedContentType,
			fmt.Sprintf("content type %q is not supported", view.Spec.Content.Type), nil)
	}

	specChanged := false
	specPatch := client.MergeFrom(view.DeepCopy())
	statusPatch := client.MergeFrom(view.DeepCopy())
	statusChanged := false
	if setParameterViewLabels(view, compParam) {
		specChanged = true
	}
	if view.Spec.Content.Type == "" {
		view.Spec.Content.Type = parametersv1alpha1.PlainTextParameterViewContentType
		specChanged = true
	}
	if syncLatestStatus(view, source) {
		statusChanged = true
	}
	if syncSubmissionResults(view, compParam, source, view.Spec.TemplateName) {
		statusChanged = true
	}
	submissionResultUpdate := updateSubmissionResultStatus(compParam, view.Spec.TemplateName, source)
	sourceViewContent, err := r.renderContent(reqCtx.Ctx, compParam, view, source.content)
	if err != nil {
		return r.markInvalidForViewContentError(reqCtx, view, err)
	}

	hadDraftContent := view.Spec.Content.Text != ""

	if view.Spec.ResetToLatest {
		view.Spec.Content.Text = sourceViewContent
		view.Spec.ResetToLatest = false
		specChanged = true
		if syncBaseRevision(view, source) {
			statusChanged = true
		}
		if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to refresh parameter view")
		}
		return r.markReady(reqCtx, view, composeStatusUpdates(updateBaseAndLatestStatus(source), submissionResultUpdate))
	}

	currentContent := view.Spec.Content.Text
	if view.Spec.Content.Text != "" {
		currentContent, err = r.extractRawContent(reqCtx.Ctx, compParam, view, source.content)
		if err != nil {
			return r.markInvalidForViewContentError(reqCtx, view, err)
		}
	}

	if view.Status.Base.Revision == "" || view.Status.Base.ContentHash == "" {
		if syncBaseRevision(view, source) {
			statusChanged = true
		}
		if view.Spec.Content.Text == "" && view.Spec.Content.Text != sourceViewContent {
			view.Spec.Content.Text = sourceViewContent
			specChanged = true
		}
	}

	if !hadDraftContent {
		if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
		}
		return r.markReady(reqCtx, view, composeStatusUpdates(updateBaseAndLatestStatus(source), submissionResultUpdate))
	}

	if view.Status.Base.Revision != "" && view.Status.Base.ContentHash != "" {
		switch {
		case currentContent == source.content:
			if view.Spec.Content.Text != sourceViewContent {
				view.Spec.Content.Text = sourceViewContent
				specChanged = true
			}
			if syncBaseRevision(view, source) {
				statusChanged = true
			}
		case view.Spec.Mode == parametersv1alpha1.ParameterViewReadOnlyMode:
			if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
			}
			return r.markInvalid(reqCtx, view, parameterViewReasonReadOnly, "content updates are not allowed in ReadOnly mode", composeStatusUpdates(updateLatestStatus(source), submissionResultUpdate))
		case view.Status.Base.ContentHash != source.hash:
			if hashContent(currentContent) == view.Status.Base.ContentHash {
				if view.Spec.Content.Text != sourceViewContent {
					view.Spec.Content.Text = sourceViewContent
					specChanged = true
				}
				if syncBaseRevision(view, source) {
					statusChanged = true
				}
				if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
					return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
				}
				return r.markReady(reqCtx, view, composeStatusUpdates(updateBaseAndLatestStatus(source), submissionResultUpdate))
			}
			equivalent, err := r.equalConfigSemantics(reqCtx.Ctx, compParam, view, currentContent, source.content)
			if err != nil {
				return r.markInvalid(reqCtx, view, parameterViewReasonDiffFailed, err.Error(), nil)
			}
			if equivalent {
				if view.Spec.Content.Text != sourceViewContent {
					view.Spec.Content.Text = sourceViewContent
					specChanged = true
				}
				if syncBaseRevision(view, source) {
					statusChanged = true
				}
				if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
					return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
				}
				return r.markReady(reqCtx, view, composeStatusUpdates(updateBaseAndLatestStatus(source), submissionResultUpdate))
			}
			desiredPatch, err := r.resolveDesiredParameterPatch(reqCtx.Ctx, compParam, view, source.content, currentContent)
			if err == nil {
				if syncBaseRevision(view, source) {
					statusChanged = true
				}
				if desiredParametersContain(compParam.Spec.Desired, desiredPatch) {
					if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
						return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
					}
					return r.markApplying(reqCtx, view, "parameter view update is pending apply", composeStatusUpdates(
						updateBaseAndLatestStatus(source),
						updateSubmissionAndResultStatus(compParam, view.Spec.TemplateName, source, currentContent, desiredPatch),
						submissionResultUpdate,
					))
				}
				if err := r.patchComponentParameterDesired(reqCtx.Ctx, compParam, desiredPatch); err != nil {
					return r.markInvalid(reqCtx, view, parameterViewReasonDiffFailed, err.Error(), composeStatusUpdates(updateLatestStatus(source), submissionResultUpdate))
				}
				if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
					return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
				}
				return r.markApplying(reqCtx, view, "parameter view update has been submitted", composeStatusUpdates(
					updateBaseAndLatestStatus(source),
					updateSubmissionAndResultStatus(compParam, view.Spec.TemplateName, source, currentContent, desiredPatch),
					submissionResultUpdate,
				))
			}
			if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
			}
			return r.markConflict(reqCtx, view,
				fmt.Sprintf("draft is based on an outdated revision for %s/%s; continue editing to retry replay or set resetToLatest=true to discard the draft",
					view.Spec.TemplateName, view.Spec.FileName),
				composeStatusUpdates(updateLatestStatus(source), submissionResultUpdate))
		default:
			desiredPatch, err := r.resolveDesiredParameterPatch(reqCtx.Ctx, compParam, view, source.content, currentContent)
			if err != nil {
				return r.markInvalidForDesiredPatchError(reqCtx, view, err, nil)
			}
			if desiredParametersContain(compParam.Spec.Desired, desiredPatch) {
				if syncBaseRevision(view, source) {
					statusChanged = true
				}
				if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
					return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
				}
				return r.markApplying(reqCtx, view, "parameter view update is pending apply", composeStatusUpdates(
					updateBaseAndLatestStatus(source),
					updateSubmissionAndResultStatus(compParam, view.Spec.TemplateName, source, currentContent, desiredPatch),
					submissionResultUpdate,
				))
			}
			if err := r.patchComponentParameterDesired(reqCtx.Ctx, compParam, desiredPatch); err != nil {
				return r.markInvalid(reqCtx, view, parameterViewReasonDiffFailed, err.Error(), composeStatusUpdates(updateLatestStatus(source), submissionResultUpdate))
			}
			if syncBaseRevision(view, source) {
				statusChanged = true
			}
			if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
			}
			return r.markApplying(reqCtx, view, "parameter view update has been submitted", composeStatusUpdates(
				updateBaseAndLatestStatus(source),
				updateSubmissionAndResultStatus(compParam, view.Spec.TemplateName, source, currentContent, desiredPatch),
				submissionResultUpdate,
			))
		}
	}

	if err := r.patchView(reqCtx, view, specPatch, statusPatch, specChanged, statusChanged); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update parameter view")
	}

	return r.markReady(reqCtx, view, composeStatusUpdates(updateBaseAndLatestStatus(source), submissionResultUpdate))
}

// SetupWithManager sets up the controller with the Manager.
func (r *ParameterViewReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.ParameterView{}).
		Watches(&parametersv1alpha1.ComponentParameter{}, handler.EnqueueRequestsFromMapFunc(r.enqueueByComponentParameter)).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.enqueueByConfigMap)).
		Complete(r)
}

func (r *ParameterViewReconciler) enqueueByComponentParameter(_ context.Context, object client.Object) []reconcile.Request {
	compParam, ok := object.(*parametersv1alpha1.ComponentParameter)
	if !ok {
		return nil
	}
	return r.listParameterViewRequests(context.Background(), compParam.Namespace, client.MatchingLabels(
		buildParameterViewLabels(compParam.Spec.ClusterName, compParam.Spec.ComponentName, compParam.Name, ""),
	))
}

func (r *ParameterViewReconciler) enqueueByConfigMap(ctx context.Context, object client.Object) []reconcile.Request {
	configMap, ok := object.(*corev1.ConfigMap)
	if !ok {
		return nil
	}
	clusterName := configMap.Labels[constant.AppInstanceLabelKey]
	componentName := configMap.Labels[constant.KBAppComponentLabelKey]
	templateName := configMap.Labels[constant.CMConfigurationSpecProviderLabelKey]
	if clusterName == "" || componentName == "" || templateName == "" {
		return nil
	}
	return r.listParameterViewRequests(ctx, configMap.Namespace, client.MatchingLabels(
		buildParameterViewLabels(clusterName, componentName, "", templateName),
	))
}

func (r *ParameterViewReconciler) listParameterViewRequests(ctx context.Context, namespace string, opts ...client.ListOption) []reconcile.Request {
	viewList := &parametersv1alpha1.ParameterViewList{}
	listOpts := append([]client.ListOption{client.InNamespace(namespace)}, opts...)
	if err := r.Client.List(ctx, viewList, listOpts...); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, len(viewList.Items))
	for i := range viewList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&viewList.Items[i]),
		})
	}
	return requests
}

func buildParameterViewLabels(clusterName, componentName, parameterRefName, templateName string) map[string]string {
	labels := constant.GetCompLabels(clusterName, componentName)
	if parameterRefName != "" {
		labels[parameterViewParameterRefLabelKey] = parameterRefName
	}
	if templateName != "" {
		labels[parameterViewTemplateLabelKey] = templateName
	}
	return labels
}

func setParameterViewLabels(view *parametersv1alpha1.ParameterView, compParam *parametersv1alpha1.ComponentParameter) bool {
	expected := buildParameterViewLabels(compParam.Spec.ClusterName, compParam.Spec.ComponentName, compParam.Name, view.Spec.TemplateName)
	if view.Labels == nil {
		view.Labels = map[string]string{}
	}
	changed := false
	for key, value := range expected {
		if view.Labels[key] == value {
			continue
		}
		view.Labels[key] = value
		changed = true
	}
	return changed
}

func syncLatestStatus(view *parametersv1alpha1.ParameterView, source *parameterViewSource) bool {
	changed := false
	if view.Status.FileFormat != source.fileFormat {
		view.Status.FileFormat = source.fileFormat
		changed = true
	}
	if view.Status.Latest.Revision != source.revision {
		view.Status.Latest.Revision = source.revision
		changed = true
	}
	if view.Status.Latest.ContentHash != source.hash {
		view.Status.Latest.ContentHash = source.hash
		changed = true
	}
	return changed
}

func syncBaseRevision(view *parametersv1alpha1.ParameterView, source *parameterViewSource) bool {
	changed := false
	if view.Status.Base.Revision != source.revision {
		view.Status.Base.Revision = source.revision
		changed = true
	}
	if view.Status.Base.ContentHash != source.hash {
		view.Status.Base.ContentHash = source.hash
		changed = true
	}
	return changed
}

func cloneParameterValueMap(src map[string]*string) map[string]*string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]*string, len(src))
	for key, value := range src {
		if value == nil {
			dst[key] = nil
			continue
		}
		copied := *value
		dst[key] = &copied
	}
	return dst
}

func equalParameterValueMap(a, b map[string]*string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, av := range a {
		bv, ok := b[key]
		if !ok {
			return false
		}
		switch {
		case av == nil && bv == nil:
			continue
		case av == nil || bv == nil:
			return false
		case *av != *bv:
			return false
		}
	}
	return true
}

func updateLatestStatus(source *parameterViewSource) func(*parametersv1alpha1.ParameterViewStatus) {
	return func(status *parametersv1alpha1.ParameterViewStatus) {
		status.FileFormat = source.fileFormat
		status.Latest.Revision = source.revision
		status.Latest.ContentHash = source.hash
	}
}

func updateBaseAndLatestStatus(source *parameterViewSource) func(*parametersv1alpha1.ParameterViewStatus) {
	return func(status *parametersv1alpha1.ParameterViewStatus) {
		updateLatestStatus(source)(status)
		status.Base.Revision = source.revision
		status.Base.ContentHash = source.hash
	}
}

func updateSubmissionStatus(revision, content string, parameters map[string]*string) func(*parametersv1alpha1.ParameterViewStatus) {
	return func(status *parametersv1alpha1.ParameterViewStatus) {
		now := metav1.Now()
		submission := parametersv1alpha1.ParameterViewSubmission{
			Revision: parametersv1alpha1.ParameterViewRevision{
				Revision:    revision,
				ContentHash: hashContent(content),
			},
			SubmittedAt: &now,
			Assignments: cloneParameterValueMap(parameters),
			Result: parametersv1alpha1.ParameterViewSubmissionResult{
				Phase:     parametersv1alpha1.ParameterViewSubmissionProcessingPhase,
				Reason:    parameterViewSubmissionReasonProcessing,
				Message:   "submission is being processed by ComponentParameter",
				UpdatedAt: &now,
			},
		}
		status.Submissions = compactSubmissions(prependSubmission(status.Submissions, submission))
	}
}

func updateSubmissionAndResultStatus(compParam *parametersv1alpha1.ComponentParameter, templateName string,
	source *parameterViewSource, content string, parameters map[string]*string) func(*parametersv1alpha1.ParameterViewStatus) {
	return func(status *parametersv1alpha1.ParameterViewStatus) {
		updateSubmissionStatus(source.revision, content, parameters)(status)
		syncSubmissionResultsStatus(status, compParam, source, templateName)
	}
}

func updateSubmissionResultStatus(compParam *parametersv1alpha1.ComponentParameter, templateName string,
	source *parameterViewSource) func(*parametersv1alpha1.ParameterViewStatus) {
	return func(status *parametersv1alpha1.ParameterViewStatus) {
		syncSubmissionResultsStatus(status, compParam, source, templateName)
	}
}

func prependSubmission(existing []parametersv1alpha1.ParameterViewSubmission, submission parametersv1alpha1.ParameterViewSubmission) []parametersv1alpha1.ParameterViewSubmission {
	result := make([]parametersv1alpha1.ParameterViewSubmission, 0, len(existing)+1)
	if duplicated := findSubmission(existing, submission); duplicated != nil {
		submission.SubmittedAt = duplicated.SubmittedAt
		submission.Result = duplicated.Result
	}
	result = append(result, submission)
	for _, item := range existing {
		if item.Revision.Revision == submission.Revision.Revision &&
			item.Revision.ContentHash == submission.Revision.ContentHash &&
			equalParameterValueMap(item.Assignments, submission.Assignments) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func findSubmission(existing []parametersv1alpha1.ParameterViewSubmission, target parametersv1alpha1.ParameterViewSubmission) *parametersv1alpha1.ParameterViewSubmission {
	for i := range existing {
		item := &existing[i]
		if item.Revision.Revision == target.Revision.Revision &&
			item.Revision.ContentHash == target.Revision.ContentHash &&
			equalParameterValueMap(item.Assignments, target.Assignments) {
			return item
		}
	}
	return nil
}

func compactSubmissions(submissions []parametersv1alpha1.ParameterViewSubmission) []parametersv1alpha1.ParameterViewSubmission {
	if len(submissions) == 0 {
		return nil
	}
	compacted := make([]parametersv1alpha1.ParameterViewSubmission, 0, min(len(submissions), parameterViewSubmissionLimit))
	for _, item := range submissions {
		compacted = append(compacted, item)
		if len(compacted) == parameterViewSubmissionLimit {
			break
		}
	}
	return compacted
}

func syncSubmissionResults(view *parametersv1alpha1.ParameterView, compParam *parametersv1alpha1.ComponentParameter,
	source *parameterViewSource, templateName string) bool {
	return syncSubmissionResultsStatus(&view.Status, compParam, source, templateName)
}

func syncSubmissionResultsStatus(status *parametersv1alpha1.ParameterViewStatus, compParam *parametersv1alpha1.ComponentParameter,
	source *parameterViewSource, templateName string) bool {
	if status == nil || len(status.Submissions) == 0 {
		return false
	}
	submission := &status.Submissions[0]
	phase, reason, message := resolveSubmissionResult(compParam, templateName, source)
	return updateSubmissionResult(submission, phase, reason, message)
}

func resolveSubmissionResult(compParam *parametersv1alpha1.ComponentParameter, templateName string,
	source *parameterViewSource) (parametersv1alpha1.ParameterViewSubmissionPhase, string, string) {
	if itemStatus := findConfigItemStatus(compParam.Status.ConfigurationItemStatus, templateName); itemStatus != nil {
		switch itemStatus.Phase {
		case parametersv1alpha1.CMergeFailedPhase:
			return parametersv1alpha1.ParameterViewSubmissionFailedPhase, parameterViewSubmissionReasonMergeFailed, firstNonEmptyPtr(itemStatus.Message, compParam.Status.Message)
		case parametersv1alpha1.CFailedPhase, parametersv1alpha1.CFailedAndPausePhase:
			return parametersv1alpha1.ParameterViewSubmissionFailedPhase, parameterViewSubmissionReasonReconfigureFailed, configItemFailureMessage(itemStatus, compParam.Status.Message)
		case parametersv1alpha1.CFinishedPhase:
			if itemStatus.LastDoneRevision == "" || itemStatus.LastDoneRevision == source.revision {
				return parametersv1alpha1.ParameterViewSubmissionSucceededPhase, parameterViewSubmissionReasonSucceeded, "submission has been applied successfully"
			}
		}
	}

	switch compParam.Status.Phase {
	case parametersv1alpha1.CMergeFailedPhase:
		return parametersv1alpha1.ParameterViewSubmissionFailedPhase, parameterViewSubmissionReasonMergeFailed, firstNonEmpty(compParam.Status.Message, "component parameter merge failed")
	case parametersv1alpha1.CFailedPhase, parametersv1alpha1.CFailedAndPausePhase:
		return parametersv1alpha1.ParameterViewSubmissionFailedPhase, parameterViewSubmissionReasonReconfigureFailed, firstNonEmpty(compParam.Status.Message, "component parameter reconfigure failed")
	case parametersv1alpha1.CFinishedPhase:
		return parametersv1alpha1.ParameterViewSubmissionSucceededPhase, parameterViewSubmissionReasonSucceeded, "submission has been applied successfully"
	default:
		return parametersv1alpha1.ParameterViewSubmissionProcessingPhase, parameterViewSubmissionReasonProcessing, "submission is being processed by ComponentParameter"
	}
}

func updateSubmissionResult(submission *parametersv1alpha1.ParameterViewSubmission,
	phase parametersv1alpha1.ParameterViewSubmissionPhase, reason, message string) bool {
	if submission == nil {
		return false
	}
	changed := false
	if submission.Result.Phase != phase {
		submission.Result.Phase = phase
		changed = true
	}
	if submission.Result.Reason != reason {
		submission.Result.Reason = reason
		changed = true
	}
	if submission.Result.Message != message {
		submission.Result.Message = message
		changed = true
	}
	if changed {
		now := metav1.Now()
		submission.Result.UpdatedAt = &now
	}
	return changed
}

func findConfigItemStatus(items []parametersv1alpha1.ConfigTemplateItemDetailStatus, templateName string) *parametersv1alpha1.ConfigTemplateItemDetailStatus {
	for i := range items {
		if items[i].Name == templateName {
			return &items[i]
		}
	}
	return nil
}

func configItemFailureMessage(itemStatus *parametersv1alpha1.ConfigTemplateItemDetailStatus, fallback string) string {
	if itemStatus == nil {
		return firstNonEmpty(fallback, "component parameter reconfigure failed")
	}
	if itemStatus.ReconcileDetail != nil {
		if msg := firstNonEmpty(itemStatus.ReconcileDetail.ErrMessage, itemStatus.ReconcileDetail.ExecResult); msg != "" {
			return msg
		}
	}
	return firstNonEmptyPtr(itemStatus.Message, fallback)
}

func firstNonEmptyPtr(primary *string, fallback string) string {
	if primary != nil && *primary != "" {
		return *primary
	}
	return firstNonEmpty(fallback)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func composeStatusUpdates(updates ...func(*parametersv1alpha1.ParameterViewStatus)) func(*parametersv1alpha1.ParameterViewStatus) {
	return func(status *parametersv1alpha1.ParameterViewStatus) {
		for _, update := range updates {
			if update != nil {
				update(status)
			}
		}
	}
}

func (r *ParameterViewReconciler) patchView(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView,
	specPatch, statusPatch client.Patch, specChanged, statusChanged bool) error {
	if specChanged {
		if err := r.Client.Patch(reqCtx.Ctx, view, specPatch); err != nil {
			return err
		}
	}
	if statusChanged {
		if err := r.patchObservedStatus(reqCtx, view, statusPatch); err != nil {
			return err
		}
	}
	return nil
}

type parameterViewSource struct {
	content    string
	hash       string
	fileFormat parametersv1alpha1.CfgFileFormat
	revision   string
}

func (r *ParameterViewReconciler) resolveSource(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView) (*parameterViewSource, error) {
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
		revision:   r.resolveSourceRevision(ctx, compParam, item),
	}, nil
}

func (r *ParameterViewReconciler) resolveSourceRevision(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, item *parametersv1alpha1.ConfigTemplateItemDetail) string {
	running := &corev1.ConfigMap{}
	runningKey := client.ObjectKey{
		Namespace: compParam.Namespace,
		Name:      parameterscore.GetComponentCfgName(compParam.Spec.ClusterName, compParam.Spec.ComponentName, item.Name),
	}
	if err := r.Client.Get(ctx, runningKey, running); err == nil && running.Annotations != nil {
		if revision := running.Annotations[constant.ConfigurationRevision]; revision != "" {
			return revision
		}
	}
	return strconv.FormatInt(compParam.Generation, 10)
}

func (r *ParameterViewReconciler) resolveFileFormat(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView) (parametersv1alpha1.CfgFileFormat, error) {
	cfgCtx, err := r.resolveConfigContext(ctx, compParam, view)
	if err != nil {
		return "", err
	}
	if cfgCtx.fileConfig.FileFormatConfig == nil {
		return "", fmt.Errorf("file format not found for %s/%s", view.Spec.TemplateName, view.Spec.FileName)
	}
	return cfgCtx.fileConfig.FileFormatConfig.Format, nil
}

func (r *ParameterViewReconciler) resolveContent(ctx context.Context, compParam *parametersv1alpha1.ComponentParameter,
	item *parametersv1alpha1.ConfigTemplateItemDetail, fileName string) (string, error) {
	running := &corev1.ConfigMap{}
	runningKey := client.ObjectKey{
		Namespace: compParam.Namespace,
		Name:      parameterscore.GetComponentCfgName(compParam.Spec.ClusterName, compParam.Spec.ComponentName, item.Name),
	}
	if err := r.Client.Get(ctx, runningKey, running); err == nil {
		if content, ok := running.Data[fileName]; ok {
			return content, nil
		}
	} else if !errors.IsNotFound(err) {
		return "", err
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

func (r *ParameterViewReconciler) resolveConfigContext(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView) (*parameterViewConfigContext, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: ctrl.Request{NamespacedName: client.ObjectKeyFromObject(compParam)},
	}
	fetchTask, err := prepareReconcileTask(reqCtx, r.Client, compParam)
	if err != nil {
		return nil, err
	}
	configDescs, paramsDefs, err := parameters.ResolveCmpdParametersDefs(ctx, r.Client, fetchTask.ComponentDefObj)
	if err != nil {
		return nil, err
	}
	templates, err := resolveComponentTemplate(ctx, r.Client, fetchTask.ComponentDefObj)
	if err != nil {
		return nil, err
	}
	for _, desc := range configDescs {
		if desc.TemplateName == view.Spec.TemplateName && desc.Name == view.Spec.FileName {
			return &parameterViewConfigContext{
				componentDef: fetchTask.ComponentDefObj,
				configDescs:  configDescs,
				paramsDefs:   paramsDefs,
				templates:    templates,
				fileConfig:   desc,
			}, nil
		}
	}
	return nil, fmt.Errorf("file not found for %s/%s", view.Spec.TemplateName, view.Spec.FileName)
}

func (r *ParameterViewReconciler) resolveDesiredParameterPatch(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView,
	sourceContent, updatedContent string) (map[string]*string, error) {
	cfgCtx, err := r.resolveConfigContext(ctx, compParam, view)
	if err != nil {
		return nil, err
	}
	descs := []parametersv1alpha1.ComponentConfigDescription{cfgCtx.fileConfig}
	baseData := map[string]string{view.Spec.FileName: sourceContent}
	updatedData := map[string]string{view.Spec.FileName: updatedContent}
	patch, _, err := parameterscore.CreateConfigPatch(baseData, updatedData, descs, false)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", parameterViewReasonDiffFailed, err)
	}
	if !patch.IsModify {
		return nil, fmt.Errorf("%s: edited content cannot be represented as desired parameter updates", parameterViewReasonUnsupportedContentChanges)
	}
	if err := parameterscore.ValidateConfigPatch(patch, descs); err != nil {
		return nil, fmt.Errorf("%s: %w", parameterViewReasonSchemaValidationFailed, err)
	}

	desiredPatch := make(map[string]*string)
	for _, filePatch := range parameterscore.GenerateVisualizedParamsList(patch, descs) {
		if filePatch.Key != view.Spec.FileName {
			continue
		}
		for _, param := range filePatch.Parameters {
			desiredPatch[param.Key] = param.Value
		}
	}
	if len(desiredPatch) == 0 {
		return nil, fmt.Errorf("%s: edited content does not contain any supported parameter updates", parameterViewReasonUnsupportedContentChanges)
	}
	if _, err := parameters.ClassifyComponentParameters(
		parametersv1alpha1.ComponentParameters(desiredPatch),
		cfgCtx.paramsDefs,
		cfgCtx.componentDef.Spec.Configs,
		cfgCtx.templates,
		cfgCtx.configDescs,
	); err != nil {
		return nil, fmt.Errorf("%s: %w", parameterViewReasonSchemaValidationFailed, err)
	}

	valueManager := parameters.NewValueManager(cfgCtx.paramsDefs, descs)
	updatedParams, err := parameterscore.FromStringMap(desiredPatch, valueManager.BuildValueTransformer(view.Spec.FileName))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", parameterViewReasonSchemaValidationFailed, err)
	}
	mergedData, err := parameters.MergeAndValidateConfigs(baseData, []parameterscore.ParamPairs{{
		Key:           view.Spec.FileName,
		UpdatedParams: updatedParams,
	}}, cfgCtx.paramsDefs, descs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", parameterViewReasonSchemaValidationFailed, err)
	}
	semanticPatch, _, err := parameterscore.CreateConfigPatch(mergedData, updatedData, descs, false)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", parameterViewReasonUnsupportedContentChanges, err)
	}
	if semanticPatch.IsModify {
		return nil, fmt.Errorf("%s: edited content contains changes that cannot be represented as desired parameter updates", parameterViewReasonUnsupportedContentChanges)
	}
	return desiredPatch, nil
}

func (r *ParameterViewReconciler) renderContent(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView, rawContent string) (string, error) {
	switch view.Spec.Content.Type {
	case "", parametersv1alpha1.PlainTextParameterViewContentType:
		return rawContent, nil
	case parametersv1alpha1.MarkerLineParameterViewContentType:
		cfgCtx, err := r.resolveConfigContext(ctx, compParam, view)
		if err != nil {
			return "", err
		}
		return r.renderMarkerLineContent(cfgCtx, view, rawContent)
	default:
		return "", fmt.Errorf("content type %q is not supported", view.Spec.Content.Type)
	}
}

func (r *ParameterViewReconciler) extractRawContent(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView, sourceContent string) (string, error) {
	switch view.Spec.Content.Type {
	case "", parametersv1alpha1.PlainTextParameterViewContentType:
		return view.Spec.Content.Text, nil
	case parametersv1alpha1.MarkerLineParameterViewContentType:
		sourceViewContent, err := r.renderContent(ctx, compParam, view, sourceContent)
		if err != nil {
			return "", err
		}
		if view.Status.Base.ContentHash == "" || view.Status.Base.ContentHash == hashContent(sourceContent) {
			if err := validateMarkerLineContent(view.Spec.Content.Text, sourceViewContent); err != nil {
				return "", err
			}
		}
		_, _, rawContent, err := parseMarkerLineContent(view.Spec.Content.Text)
		return rawContent, err
	default:
		return "", fmt.Errorf("content type %q is not supported", view.Spec.Content.Type)
	}
}

func (r *ParameterViewReconciler) equalConfigSemantics(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, view *parametersv1alpha1.ParameterView, leftContent, rightContent string) (bool, error) {
	cfgCtx, err := r.resolveConfigContext(ctx, compParam, view)
	if err != nil {
		return false, err
	}
	descs := []parametersv1alpha1.ComponentConfigDescription{cfgCtx.fileConfig}
	leftValues, err := parameterscore.TransformConfigFileToKeyValueMap(view.Spec.FileName, descs, []byte(leftContent))
	if err != nil {
		return false, err
	}
	rightValues, err := parameterscore.TransformConfigFileToKeyValueMap(view.Spec.FileName, descs, []byte(rightContent))
	if err != nil {
		return false, err
	}
	if len(leftValues) != len(rightValues) {
		return false, nil
	}
	for key, leftValue := range leftValues {
		rightValue, ok := rightValues[key]
		if !ok || rightValue != leftValue {
			return false, nil
		}
	}
	return true, nil
}

func hashContent(content string) string {
	hash, err := intctrlutil.ComputeHash(content)
	if err != nil {
		panic(err)
	}
	return hash
}

func containsPhrase(message, phrase string) bool {
	return strings.Contains(strings.ToLower(message), strings.ToLower(phrase))
}

func trimReasonPrefix(message, reason string) string {
	prefix := reason + ": "
	if strings.HasPrefix(message, prefix) {
		return strings.TrimPrefix(message, prefix)
	}
	return message
}

func desiredParametersContain(inputs *parametersv1alpha1.ParameterInputs, patch map[string]*string) bool {
	if inputs == nil || len(inputs.Assignments) == 0 || len(patch) == 0 {
		return false
	}
	for key, expected := range patch {
		actual, ok := inputs.Assignments[key]
		if !ok {
			return false
		}
		switch {
		case expected == nil && actual == nil:
		case expected != nil && actual != nil && *expected == *actual:
		default:
			return false
		}
	}
	return true
}

func (r *ParameterViewReconciler) patchComponentParameterDesired(ctx context.Context,
	compParam *parametersv1alpha1.ComponentParameter, patchValues map[string]*string) error {
	if len(patchValues) == 0 {
		return nil
	}
	patch := client.MergeFrom(compParam.DeepCopy())
	if compParam.Spec.Desired == nil {
		compParam.Spec.Desired = &parametersv1alpha1.ParameterInputs{}
	}
	if compParam.Spec.Desired.Assignments == nil {
		compParam.Spec.Desired.Assignments = map[string]*string{}
	}
	for key, value := range patchValues {
		compParam.Spec.Desired.Assignments[key] = value
	}
	return r.Client.Patch(ctx, compParam, patch)
}

type parameterViewConfigContext struct {
	componentDef *appsv1.ComponentDefinition
	configDescs  []parametersv1alpha1.ComponentConfigDescription
	paramsDefs   []*parametersv1alpha1.ParametersDefinition
	templates    map[string]*corev1.ConfigMap
	fileConfig   parametersv1alpha1.ComponentConfigDescription
}

func (r *ParameterViewReconciler) markReady(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView,
	updateStatus func(*parametersv1alpha1.ParameterViewStatus)) (ctrl.Result, error) {
	return r.patchStatus(reqCtx, view, parametersv1alpha1.ParameterViewSyncedPhase, "", metav1.ConditionTrue, parameterViewReasonResolved, "parameter view is synced", updateStatus)
}

func (r *ParameterViewReconciler) markConflict(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView,
	msg string, updateStatus func(*parametersv1alpha1.ParameterViewStatus)) (ctrl.Result, error) {
	return r.patchStatus(reqCtx, view, parametersv1alpha1.ParameterViewConflictPhase, msg, metav1.ConditionFalse, parameterViewReasonDraftOutdated, msg, updateStatus)
}

func (r *ParameterViewReconciler) markApplying(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView,
	msg string, updateStatus func(*parametersv1alpha1.ParameterViewStatus)) (ctrl.Result, error) {
	if _, err := r.patchStatus(reqCtx, view, parametersv1alpha1.ParameterViewApplyingPhase, msg, metav1.ConditionFalse, parameterViewReasonApplying, msg, updateStatus); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ParameterViewReconciler) markInvalid(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView,
	reason, msg string, updateStatus func(*parametersv1alpha1.ParameterViewStatus)) (ctrl.Result, error) {
	return r.patchStatus(reqCtx, view, parametersv1alpha1.ParameterViewInvalidPhase, msg, metav1.ConditionFalse, reason, msg, updateStatus)
}

func (r *ParameterViewReconciler) markInvalidForSourceError(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView, err error) (ctrl.Result, error) {
	msg := err.Error()
	switch {
	case containsPhrase(msg, "template not found"):
		return r.markInvalid(reqCtx, view, parameterViewReasonTemplateNotFound, msg, nil)
	case containsPhrase(msg, "file not found"):
		return r.markInvalid(reqCtx, view, parameterViewReasonFileNotFound, msg, nil)
	default:
		return r.markInvalid(reqCtx, view, parameterViewReasonDiffFailed, msg, nil)
	}
}

func (r *ParameterViewReconciler) markInvalidForDesiredPatchError(reqCtx intctrlutil.RequestCtx,
	view *parametersv1alpha1.ParameterView, err error, updateStatus func(*parametersv1alpha1.ParameterViewStatus)) (ctrl.Result, error) {
	msg := err.Error()
	switch {
	case containsPhrase(msg, parameterViewReasonDiffFailed):
		return r.markInvalid(reqCtx, view, parameterViewReasonDiffFailed, trimReasonPrefix(msg, parameterViewReasonDiffFailed), updateStatus)
	case containsPhrase(msg, parameterViewReasonSchemaValidationFailed):
		return r.markInvalid(reqCtx, view, parameterViewReasonSchemaValidationFailed, trimReasonPrefix(msg, parameterViewReasonSchemaValidationFailed), updateStatus)
	default:
		return r.markInvalid(reqCtx, view, parameterViewReasonUnsupportedContentChanges, trimReasonPrefix(msg, parameterViewReasonUnsupportedContentChanges), updateStatus)
	}
}

func (r *ParameterViewReconciler) markInvalidForViewContentError(reqCtx intctrlutil.RequestCtx, view *parametersv1alpha1.ParameterView, err error) (ctrl.Result, error) {
	msg := err.Error()
	switch {
	case containsPhrase(msg, "cannot be modified"):
		return r.markInvalid(reqCtx, view, parameterViewReasonUnsupportedContentChanges, msg, nil)
	case containsPhrase(msg, "marker"):
		return r.markInvalid(reqCtx, view, parameterViewReasonInvalidMarkerSyntax, msg, nil)
	default:
		return r.markInvalid(reqCtx, view, parameterViewReasonUnsupportedContentChanges, msg, nil)
	}
}

func (r *ParameterViewReconciler) patchStatus(reqCtx intctrlutil.RequestCtx,
	view *parametersv1alpha1.ParameterView, phase parametersv1alpha1.ParameterViewPhase,
	message string, conditionStatus metav1.ConditionStatus, reason, conditionMessage string,
	updateStatus func(*parametersv1alpha1.ParameterViewStatus)) (ctrl.Result, error) {
	patch := client.MergeFrom(view.DeepCopy())
	if updateStatus != nil {
		updateStatus(&view.Status)
	}
	view.Status.ObservedGeneration = view.Generation
	view.Status.Phase = phase
	view.Status.Message = message
	meta.SetStatusCondition(&view.Status.Conditions, metav1.Condition{
		Type:               parameterViewSyncedCondition,
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

func (r *ParameterViewReconciler) patchObservedStatus(reqCtx intctrlutil.RequestCtx,
	view *parametersv1alpha1.ParameterView, patch client.Patch) error {
	if err := r.Client.Status().Patch(reqCtx.Ctx, view, patch); err != nil {
		return err
	}
	return nil
}
