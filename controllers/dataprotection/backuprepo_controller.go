/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package dataprotection

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/yaml"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	// TODO: make it configurable
	defaultPreCheckTimeout = 15 * time.Minute
	defaultCheckInterval   = 1 * time.Minute

	preCheckContainerName = "pre-check"
)

var (
	// for testing
	wallClock clock.Clock = &clock.RealClock{}
)

type reconcileContext struct {
	intctrlutil.RequestCtx
	repo       *dpv1alpha1.BackupRepo
	provider   *storagev1alpha1.StorageProvider
	Parameters map[string]string
	renderCtx  renderContext
	digest     string
}

func (r *reconcileContext) getDigest() string {
	if r.digest != "" {
		return r.digest
	}
	content := ""
	content += stableSerializeMap(r.Parameters)
	content += r.provider.Spec.StorageClassTemplate
	content += r.provider.Spec.PersistentVolumeClaimTemplate
	content += r.provider.Spec.CSIDriverSecretTemplate
	content += r.provider.Spec.DatasafedConfigTemplate
	r.digest = md5Digest(content)
	return r.digest
}

func (r *reconcileContext) digestChanged() bool {
	return !r.hasSameDigest(r.repo)
}

func (r *reconcileContext) preCheckFinished() bool {
	cond := meta.FindStatusCondition(r.repo.Status.Conditions, ConditionTypePreCheckPassed)
	return cond != nil && cond.Status != metav1.ConditionUnknown
}

func (r *reconcileContext) hasSameDigest(obj client.Object) bool {
	return obj.GetAnnotations()[dataProtectionBackupRepoDigestAnnotationKey] == r.getDigest()
}

func (r *reconcileContext) preCheckResourceName() string {
	return cutName(fmt.Sprintf("pre-check-%s-%s", r.repo.UID[:8], r.repo.Name))
}

// BackupRepoReconciler reconciles a BackupRepo object
type BackupRepoReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	RestConfig *rest.Config

	secretRefMapper   refObjectMapper
	providerRefMapper refObjectMapper
}

// full access on BackupRepos
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuprepos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuprepos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuprepos/finalizers,verbs=update

// watch StorageProviders
// +kubebuilder:rbac:groups=storage.kubeblocks.io,resources=storageproviders,verbs=get;list;watch

// watch or update Backups
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;update;patch

// create or delete StorageClasses
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch;create;delete

// create or delete PVCs
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// create or delete Secrets
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// create or delete Jobs
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *BackupRepoReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("backuprepo", req.NamespacedName)
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      logger,
		Recorder: r.Recorder,
	}

	// TODO: better event recording

	// get repo object
	repo := &dpv1alpha1.BackupRepo{}
	if err := r.Get(ctx, req.NamespacedName, repo); err != nil {
		return checkedRequeueWithError(err, reqCtx.Log, "failed to get BackupRepo")
	}

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, repo, dptypes.DataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, repo)
	})
	if res != nil {
		return *res, err
	}

	// add references
	if repo.Spec.Credential != nil {
		r.secretRefMapper.setRef(repo, types.NamespacedName{
			Name:      repo.Spec.Credential.Name,
			Namespace: repo.Spec.Credential.Namespace,
		})
	}
	r.providerRefMapper.setRef(repo, types.NamespacedName{Name: repo.Spec.StorageProviderRef})

	// check storage provider
	provider, err := r.checkStorageProvider(reqCtx, repo)
	if err != nil {
		_ = r.updateStatus(reqCtx, repo)
		return checkedRequeueWithError(err, reqCtx.Log, "check storage provider status failed")
	}

	// check parameters for rendering templates
	parameters, err := r.checkParameters(reqCtx, repo)
	if err != nil {
		_ = r.updateStatus(reqCtx, repo)
		return checkedRequeueWithError(err, reqCtx.Log, "check parameters failed")
	}

	reconCtx := &reconcileContext{
		RequestCtx: reqCtx,
		repo:       repo,
		provider:   provider,
		Parameters: parameters,
		renderCtx: renderContext{
			Parameters: parameters,
		},
	}

	// create StorageClass and Secret for the CSI driver
	err = r.createStorageClassAndSecret(reconCtx)
	if err != nil {
		_ = r.updateStatus(reqCtx, repo)
		return checkedRequeueWithError(err, reqCtx.Log,
			"failed to create storage class and secret")
	}

	// check PVC template
	err = r.checkPVCTemplate(reconCtx)
	if err != nil {
		_ = r.updateStatus(reqCtx, repo)
		return checkedRequeueWithError(err, reqCtx.Log,
			"failed to check PVC template")
	}

	// pre-check the repo by running a real job
	if repo.Status.Phase != dpv1alpha1.BackupRepoDeleting {
		err = r.preCheckRepo(reconCtx)
		if err != nil {
			_ = r.updateStatus(reqCtx, repo)
			return checkedRequeueWithError(err, reqCtx.Log, "failed to pre-check")
		}
	}

	// update status phase to ready if all conditions are met
	if err = r.updateStatus(reqCtx, repo); err != nil {
		return checkedRequeueWithError(err, reqCtx.Log,
			"failed to update BackupRepo status")
	}

	if reconCtx.preCheckFinished() {
		// clear pre-check resources
		if err := r.removePreCheckResources(reconCtx); err != nil {
			return checkedRequeueWithError(err, reqCtx.Log,
				"failed to remove pre-check resources")
		}
	}

	if repo.Status.Phase == dpv1alpha1.BackupRepoReady {
		// update tool config if needed
		err = r.updateToolConfigSecrets(reconCtx)
		if err != nil {
			return checkedRequeueWithError(err, reqCtx.Log,
				"failed to update tool config secrets")
		}

		// check associated backups, to create PVC in their namespaces
		if err = r.prepareForAssociatedBackups(reconCtx); err != nil {
			return checkedRequeueWithError(err, reqCtx.Log,
				"check associated backups failed")
		}
	}

	return ctrl.Result{}, nil
}

func (r *BackupRepoReconciler) updateStatus(reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) error {
	old := repo.DeepCopy()
	// not allow to transit to other phase if it is deleting
	if repo.Status.Phase != dpv1alpha1.BackupRepoDeleting {
		phase := dpv1alpha1.BackupRepoFailed
		basicCheckingPassed := meta.IsStatusConditionTrue(repo.Status.Conditions, ConditionTypeStorageProviderReady) &&
			meta.IsStatusConditionTrue(repo.Status.Conditions, ConditionTypeParametersChecked) &&
			meta.IsStatusConditionTrue(repo.Status.Conditions, ConditionTypeStorageClassCreated) &&
			meta.IsStatusConditionTrue(repo.Status.Conditions, ConditionTypePVCTemplateChecked)
		if basicCheckingPassed {
			cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypePreCheckPassed)
			if cond != nil && cond.Status == metav1.ConditionTrue {
				phase = dpv1alpha1.BackupRepoReady
			} else if cond != nil && cond.Status == metav1.ConditionUnknown {
				phase = dpv1alpha1.BackupRepoPreChecking
			}
		}
		repo.Status.Phase = phase
	}
	repo.Status.IsDefault = repo.Annotations[dptypes.DefaultBackupRepoAnnotationKey] == trueVal

	// update other fields
	if repo.Status.BackupPVCName == "" {
		repo.Status.BackupPVCName = randomNameForDerivedObject(repo, "pvc")
	}
	if repo.Status.ToolConfigSecretName == "" {
		repo.Status.ToolConfigSecretName = randomNameForDerivedObject(repo, "tool-config")
	}
	if repo.Status.ObservedGeneration != repo.Generation {
		repo.Status.ObservedGeneration = repo.Generation
	}

	if !reflect.DeepEqual(old.Status, repo.Status) {
		if err := r.Client.Status().Patch(reqCtx.Ctx, repo, client.MergeFrom(old)); err != nil {
			return fmt.Errorf("updateStatus failed: %w", err)
		}
	}
	return nil
}

func (r *BackupRepoReconciler) updateConditionInDefer(ctx context.Context, repo *dpv1alpha1.BackupRepo,
	condType string, reason string, statusPtr *metav1.ConditionStatus, messagePtr *string, err *error) {
	status := metav1.ConditionTrue
	message := ""
	if *err != nil {
		status = metav1.ConditionFalse
		message = (*err).Error()
	}
	if statusPtr != nil {
		status = *statusPtr
	}
	if messagePtr != nil {
		message = *messagePtr
	}
	updateErr := updateCondition(ctx, r.Client, repo, condType, status, reason, message)
	if *err == nil {
		*err = updateErr
	}
}

func (r *BackupRepoReconciler) checkStorageProvider(
	reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) (provider *storagev1alpha1.StorageProvider, err error) {
	reason := ReasonUnknownError
	defer func() {
		r.updateConditionInDefer(reqCtx.Ctx, repo, ConditionTypeStorageProviderReady, reason, nil, nil, &err)
	}()

	// get storage provider object
	providerKey := client.ObjectKey{Name: repo.Spec.StorageProviderRef}
	provider = &storagev1alpha1.StorageProvider{}
	err = r.Client.Get(reqCtx.Ctx, providerKey, provider)
	if err != nil {
		if apierrors.IsNotFound(err) {
			reason = ReasonStorageProviderNotFound
		}
		return nil, err
	}

	// check its spec
	switch {
	case repo.AccessByMount():
		if provider.Spec.StorageClassTemplate == "" &&
			provider.Spec.PersistentVolumeClaimTemplate == "" {
			// both StorageClassTemplate and PersistentVolumeClaimTemplate are empty.
			// in this case, we are unable to create a backup PVC.
			reason = ReasonInvalidStorageProvider
			return provider, newDependencyError("both StorageClassTemplate and PersistentVolumeClaimTemplate are empty")
		}
		csiInstalledCond := meta.FindStatusCondition(provider.Status.Conditions, storagev1alpha1.ConditionTypeCSIDriverInstalled)
		if csiInstalledCond == nil || csiInstalledCond.Status != metav1.ConditionTrue {
			reason = ReasonStorageProviderNotReady
			return provider, newDependencyError("CSI driver is not installed")
		}
	case repo.AccessByTool():
		if provider.Spec.DatasafedConfigTemplate == "" {
			reason = ReasonInvalidStorageProvider
			return provider, newDependencyError("DatasafedConfigTemplate is empty")
		}
	}

	// check its status
	reason = ReasonStorageProviderReady
	return provider, nil
}

func (r *BackupRepoReconciler) checkParameters(reqCtx intctrlutil.RequestCtx,
	repo *dpv1alpha1.BackupRepo) (parameters map[string]string, err error) {
	reason := ReasonUnknownError
	defer func() {
		r.updateConditionInDefer(reqCtx.Ctx, repo, ConditionTypeParametersChecked, reason, nil, nil, &err)
	}()

	// collect parameters for rendering templates
	parameters, err = r.collectParameters(reqCtx, repo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			reason = ReasonCredentialSecretNotFound
		}
		return nil, err
	}
	// TODO: verify parameters
	reason = ReasonParametersChecked
	return parameters, nil
}

func (r *BackupRepoReconciler) createStorageClassAndSecret(reconCtx *reconcileContext) (err error) {

	reason := ReasonUnknownError
	defer func() {
		r.updateConditionInDefer(reconCtx.Ctx, reconCtx.repo, ConditionTypeStorageClassCreated, reason, nil, nil, &err)
	}()

	oldRepo := reconCtx.repo.DeepCopy()

	// create secret for the CSI driver if it's not exist,
	// or update the secret if the template or values are updated
	if reconCtx.provider.Spec.CSIDriverSecretTemplate != "" {
		if reconCtx.repo.Status.GeneratedCSIDriverSecret == nil {
			reconCtx.repo.Status.GeneratedCSIDriverSecret = &corev1.SecretReference{
				Name:      randomNameForDerivedObject(reconCtx.repo, "secret"),
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
			}
		}
		reconCtx.renderCtx.CSIDriverSecretRef = *reconCtx.repo.Status.GeneratedCSIDriverSecret
		// create or update the secret for CSI
		if _, err = r.createOrUpdateSecretForCSIDriver(reconCtx); err != nil {
			reason = ReasonPrepareCSISecretFailed
			return err
		}
	}

	if reconCtx.provider.Spec.StorageClassTemplate != "" {
		// create storage class if it's not exist
		if reconCtx.repo.Status.GeneratedStorageClassName == "" {
			reconCtx.repo.Status.GeneratedStorageClassName = randomNameForDerivedObject(reconCtx.repo, "sc")
		}
		if _, err = r.createStorageClass(reconCtx); err != nil {
			reason = ReasonPrepareStorageClassFailed
			return err
		}
	}

	if !meta.IsStatusConditionTrue(reconCtx.repo.Status.Conditions, ConditionTypeStorageClassCreated) {
		setCondition(reconCtx.repo, ConditionTypeStorageClassCreated,
			metav1.ConditionTrue, ReasonStorageClassCreated, "")
	}

	if !reflect.DeepEqual(oldRepo.Status, reconCtx.repo.Status) {
		err := r.Client.Status().Patch(reconCtx.Ctx, reconCtx.repo, client.MergeFrom(oldRepo))
		if err != nil {
			return fmt.Errorf("failed to patch backup repo: %w", err)
		}
	}
	reason = ReasonStorageClassCreated
	return nil
}

func (r *BackupRepoReconciler) createOrUpdateSecretForCSIDriver(
	reconCtx *reconcileContext) (created bool, err error) {

	secret := &corev1.Secret{}
	secret.Name = reconCtx.repo.Status.GeneratedCSIDriverSecret.Name
	secret.Namespace = reconCtx.repo.Status.GeneratedCSIDriverSecret.Namespace

	shouldUpdateFunc := func() bool {
		oldDigest := secret.Annotations[dataProtectionBackupRepoDigestAnnotationKey]
		return oldDigest != reconCtx.getDigest()
	}

	return createOrUpdateObject(reconCtx.Ctx, r.Client, secret, func() error {
		// render secret template
		content, err := renderTemplate("secret", reconCtx.provider.Spec.CSIDriverSecretTemplate, reconCtx.renderCtx)
		if err != nil {
			return fmt.Errorf("failed to render secret template: %w", err)
		}
		secretStringData := map[string]string{}
		if err = yaml.Unmarshal([]byte(content), &secretStringData); err != nil {
			return fmt.Errorf("failed to unmarshal secret content: %w", err)
		}
		secretData := make(map[string][]byte, len(secretStringData))
		for k, v := range secretStringData {
			secretData[k] = []byte(v)
		}
		secret.Data = secretData

		// set labels and annotations
		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		secret.Labels[dataProtectionBackupRepoKey] = reconCtx.repo.Name

		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		secret.Annotations[dataProtectionBackupRepoDigestAnnotationKey] = reconCtx.getDigest()

		if err := controllerutil.SetControllerReference(reconCtx.repo, secret, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}
		return nil
	}, shouldUpdateFunc)
}

func (r *BackupRepoReconciler) createStorageClass(
	reconCtx *reconcileContext) (created bool, err error) {

	storageClass := &storagev1.StorageClass{}
	storageClass.Name = reconCtx.repo.Status.GeneratedStorageClassName
	return createObjectIfNotExist(reconCtx.Ctx, r.Client, storageClass,
		func() error {
			// render storage class template
			content, err := renderTemplate("sc", reconCtx.provider.Spec.StorageClassTemplate, reconCtx.renderCtx)
			if err != nil {
				return fmt.Errorf("failed to render storage class template: %w", err)
			}
			if err = yaml.Unmarshal([]byte(content), storageClass); err != nil {
				return fmt.Errorf("failed to unmarshal storage class: %w", err)
			}

			// create storage class object
			storageClass.Labels = map[string]string{
				dataProtectionBackupRepoKey: reconCtx.repo.Name,
			}
			bindingMode := storagev1.VolumeBindingImmediate
			storageClass.VolumeBindingMode = &bindingMode
			if reconCtx.repo.Spec.PVReclaimPolicy != "" {
				storageClass.ReclaimPolicy = &reconCtx.repo.Spec.PVReclaimPolicy
			}
			if err := controllerutil.SetControllerReference(reconCtx.repo, storageClass, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			return nil
		})
}

func (r *BackupRepoReconciler) checkPVCTemplate(reconCtx *reconcileContext) (err error) {
	reason := ReasonUnknownError
	defer func() {
		r.updateConditionInDefer(reconCtx.Ctx, reconCtx.repo, ConditionTypePVCTemplateChecked, reason, nil, nil, &err)
	}()

	if !reconCtx.repo.AccessByMount() || reconCtx.provider.Spec.PersistentVolumeClaimTemplate == "" {
		reason = ReasonSkipped
		return nil
	}
	if reconCtx.digestChanged() {
		pvc := &corev1.PersistentVolumeClaim{}
		err := r.constructPVCByTemplate(reconCtx, pvc, reconCtx.provider.Spec.PersistentVolumeClaimTemplate)
		if err != nil {
			reason = ReasonBadPVCTemplate
			return err
		}
	}
	reason = ReasonPVCTemplateChecked
	return nil
}

func (r *BackupRepoReconciler) updateToolConfigSecrets(reconCtx *reconcileContext) (err error) {
	if !reconCtx.repo.AccessByTool() {
		return nil
	}
	if reconCtx.repo.Annotations[dataProtectionNeedUpdateToolConfigAnnotationKey] != trueVal {
		return nil
	}
	// render tool config template
	content, err := renderTemplate("tool-config", reconCtx.provider.Spec.DatasafedConfigTemplate, reconCtx.renderCtx)
	if err != nil {
		return err
	}
	// update existing tool config secrets
	secretList := &corev1.SecretList{}
	err = r.Client.List(reconCtx.Ctx, secretList, client.MatchingLabels{
		dataProtectionBackupRepoKey:   reconCtx.repo.Name,
		dataProtectionIsToolConfigKey: trueVal,
	})
	if err != nil {
		return err
	}
	for idx := range secretList.Items {
		secret := &secretList.Items[idx]
		oldDigest := secret.Annotations[dataProtectionBackupRepoDigestAnnotationKey]
		if oldDigest == reconCtx.getDigest() {
			continue
		}
		patch := client.MergeFrom(secret.DeepCopy())
		constructToolConfigSecret(secret, content)
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		secret.Annotations[dataProtectionBackupRepoDigestAnnotationKey] = reconCtx.getDigest()
		if err = r.Client.Patch(reconCtx.Ctx, secret, patch); err != nil {
			return err
		}
	}

	return updateAnnotations(reconCtx.Ctx, r.Client, reconCtx.repo, map[string]string{
		dataProtectionNeedUpdateToolConfigAnnotationKey: "false",
	})
}

func (r *BackupRepoReconciler) preCheckRepo(reconCtx *reconcileContext) (err error) {
	if reconCtx.digestChanged() {
		// invalidate the old status. reconCtx.preCheckFinished() depends on this value
		err := updateCondition(reconCtx.Ctx, r.Client, reconCtx.repo, ConditionTypePreCheckPassed,
			metav1.ConditionUnknown, ReasonDigestChanged, "")
		if err != nil {
			return err
		}

		err = updateAnnotations(reconCtx.Ctx, r.Client, reconCtx.repo, map[string]string{
			dataProtectionBackupRepoDigestAnnotationKey:     reconCtx.getDigest(),
			dataProtectionNeedUpdateToolConfigAnnotationKey: trueVal,
		})
		if err != nil {
			return err
		}
	}
	if reconCtx.preCheckFinished() {
		return nil
	}

	status := metav1.ConditionUnknown
	reason := ReasonUnknownError
	message := ""
	defer func() {
		if message == "" && err != nil {
			message = err.Error()
		}
		r.updateConditionInDefer(reconCtx.Ctx, reconCtx.repo, ConditionTypePreCheckPassed, reason, &status, &message, &err)
	}()
	var job *batchv1.Job
	var pvc *corev1.PersistentVolumeClaim
	switch {
	case reconCtx.repo.AccessByMount():
		job, pvc, err = r.runPreCheckJobForMounting(reconCtx)
	case reconCtx.repo.AccessByTool():
		job, err = r.runPreCheckJobForTool(reconCtx)
	default:
		err = fmt.Errorf("unknown access method: %s", reconCtx.repo.Spec.AccessMethod)
	}
	if err != nil {
		return err
	}

	finished, jobStatus, failureReason := utils.IsJobFinished(job)
	if !finished {
		duration := wallClock.Since(job.CreationTimestamp.Time)
		if duration > defaultPreCheckTimeout {
			// HACK: mark as failure
			jobStatus = batchv1.JobFailed
			failureReason = "timeout"
		} else {
			// Job and Pod both have activeDeadlineSeconds, but neither of them is suitable for our scenario.
			// If job.spec.activeDeadlineSeconds is set, when the run times out, the job controller will delete
			// the running pods directly to stop them; since the pods are deleted, we may not have time to collect
			// the error logs.
			// In the meantime, pod.spec.activeDeadlineSeconds may fail in some cases. When the configuration
			// of a PVC based backup repository is wrong, the PVC provisioning will fail, which makes the pod
			// get stuck in the "Pending" state, but activeDeadlineSeconds seems to start counting from the
			// "Running" state, so the pod will not fail due to timeout.
			return intctrlutil.NewRequeueError(defaultCheckInterval, "wait job to finish")
		}
	}

	if jobStatus == batchv1.JobFailed {
		status = metav1.ConditionFalse
		reason = ReasonPreCheckFailed

		// collect logs and events from these objects
		info, err := r.collectPreCheckFailureMessage(reconCtx, job, pvc)
		if err != nil {
			return fmt.Errorf("failed to collectPreCheckFailureMessage, err: %w", err)
		}
		message = "Pre-check job failed, information collected for diagnosis.\n\n"
		message += fmt.Sprintf("Job failure message: %s\n\n", failureReason)
		message += info
		// max length of metav1.Condition.Message is 32K
		const messageLimit = 32 * 1024
		if len(message) > messageLimit {
			message = message[:messageLimit]
		}
	} else {
		status = metav1.ConditionTrue
		reason = ReasonPreCheckPassed
	}
	return nil
}

func (r *BackupRepoReconciler) removePreCheckResources(reconCtx *reconcileContext) error {
	objects := []client.Object{
		&batchv1.Job{},
		&corev1.PersistentVolumeClaim{},
		&corev1.Secret{},
	}
	name := reconCtx.preCheckResourceName()
	namespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	objKey := client.ObjectKey{Name: name, Namespace: namespace}
	for _, obj := range objects {
		err := r.Client.Get(reconCtx.Ctx, objKey, obj)
		if err == nil {
			err = intctrlutil.BackgroundDeleteObject(r.Client, reconCtx.Ctx, obj)
		}
		if err == nil || apierrors.IsNotFound(err) {
			continue
		}
		return err
	}
	return nil
}

func (r *BackupRepoReconciler) runPreCheckJobForMounting(reconCtx *reconcileContext) (job *batchv1.Job, pvc *corev1.PersistentVolumeClaim, err error) {
	namespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	// create PVC
	pvcName := reconCtx.preCheckResourceName()
	pvc, err = r.createRepoPVC(reconCtx, pvcName, namespace, map[string]string{
		dataProtectionBackupRepoDigestAnnotationKey: reconCtx.getDigest(),
	})
	if err != nil {
		return nil, nil, err
	}
	// run pre-check job
	job = &batchv1.Job{}
	job.Name = reconCtx.preCheckResourceName()
	job.Namespace = namespace
	_, err = createObjectIfNotExist(reconCtx.Ctx, r.Client, job, func() error {
		job.Spec = batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:            preCheckContainerName,
						Image:           viper.GetString(constant.KBToolsImage),
						ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
						Command: []string{
							"sh", "-c", `set -ex; echo "pre-check" > /backup/precheck.txt; sync`,
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "backup-pvc",
							MountPath: "/backup",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "backup-pvc",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvcName,
							},
						},
					}},
				},
			},
			BackoffLimit: pointer.Int32(2),
		}
		for i := range job.Spec.Template.Spec.Containers {
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&job.Spec.Template.Spec.Containers[i])
		}
		job.Labels = map[string]string{
			dataProtectionBackupRepoKey: reconCtx.repo.Name,
		}
		job.Annotations = map[string]string{
			dataProtectionBackupRepoDigestAnnotationKey: reconCtx.getDigest(),
		}
		return controllerutil.SetControllerReference(reconCtx.repo, job, r.Scheme)
	})
	if err != nil {
		return nil, nil, err
	}

	// these resources were created for the old generation of the backupRepo,
	// so remove them and then retry.
	if !reconCtx.hasSameDigest(pvc) || !reconCtx.hasSameDigest(job) {
		err = r.removePreCheckResources(reconCtx)
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("pre-check job or PVC digest not match, try again")
	}
	return job, pvc, nil
}

func (r *BackupRepoReconciler) runPreCheckJobForTool(reconCtx *reconcileContext) (job *batchv1.Job, err error) {
	namespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	// create tool config
	secretName := reconCtx.preCheckResourceName()
	secret, err := r.createToolConfigSecret(reconCtx, secretName, namespace, map[string]string{
		dataProtectionBackupRepoDigestAnnotationKey: reconCtx.getDigest(),
	})
	if err != nil {
		return nil, err
	}
	// run pre-check job
	job = &batchv1.Job{}
	job.Name = reconCtx.preCheckResourceName()
	job.Namespace = namespace
	_, err = createObjectIfNotExist(reconCtx.Ctx, r.Client, job, func() error {
		job.Spec = batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:            preCheckContainerName,
						Image:           viper.GetString(constant.KBToolsImage),
						ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
						Command: []string{
							"sh", "-c",
							`
set -ex
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
echo "pre-check" | datasafed push - /precheck.txt`,
						},
					}},
				},
			},
			BackoffLimit: pointer.Int32(2),
		}
		job.Labels = map[string]string{
			dataProtectionBackupRepoKey: reconCtx.repo.Name,
		}
		job.Annotations = map[string]string{
			dataProtectionBackupRepoDigestAnnotationKey: reconCtx.getDigest(),
		}
		for i := range job.Spec.Template.Spec.Containers {
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&job.Spec.Template.Spec.Containers[i])
		}
		utils.InjectDatasafedWithConfig(&job.Spec.Template.Spec, secretName, "")
		return controllerutil.SetControllerReference(reconCtx.repo, job, r.Scheme)
	})
	if err != nil {
		return nil, err
	}

	// these resources were created for the old generation of the backupRepo,
	// so remove them and then retry.
	if !reconCtx.hasSameDigest(secret) || !reconCtx.hasSameDigest(job) {
		err = r.removePreCheckResources(reconCtx)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("pre-check job or tool config secret digest not match, try again")
	}
	return job, nil
}

func (r *BackupRepoReconciler) collectPreCheckFailureMessage(reconCtx *reconcileContext, job *batchv1.Job, pvc *corev1.PersistentVolumeClaim) (string, error) {
	podList, err := utils.GetAssociatedPodsOfJob(reconCtx.Ctx, r.Client, job.Namespace, job.Name)
	if err != nil {
		return "", err
	}
	// sort pod with latest creation place front
	slices.SortFunc(podList.Items, func(a, b corev1.Pod) int {
		if a.CreationTimestamp.Equal(&(b.CreationTimestamp)) {
			return 0
		}
		if a.CreationTimestamp.Before(&(b.CreationTimestamp)) {
			return 1
		}
		return -1
	})

	var message string

	// collect failure logs from the pod
	const contentLimit = 4 * 1024
	failureLogs, err := r.collectFailedPodLogs(reconCtx.Ctx, podList, preCheckContainerName, contentLimit)
	if err != nil {
		return "", err
	}
	if failureLogs == "" {
		message += "No logs are available.\n\n"
	} else {
		message += fmt.Sprintf("Logs from the pre-check job:\n%s\n", utils.PrependSpaces(failureLogs, 2))
	}

	collectEvents := func(object client.Object) error {
		gvk, err := r.Client.GroupVersionKindFor(object)
		if err != nil {
			return err
		}
		events, err := fetchObjectEvents(reconCtx.Ctx, r.Client, object)
		if err != nil {
			return err
		}
		// kind := object.GetObjectKind().GroupVersionKind().Kind
		kind := gvk.Kind
		if len(events.Items) == 0 {
			message += fmt.Sprintf("No events are available for %s/%s.\n\n", kind, client.ObjectKeyFromObject(object))
		} else {
			content := utils.EventsToString(events)
			if len(content) > contentLimit {
				content = content[:contentLimit] + "[truncated]"
			}
			message += fmt.Sprintf("Events from %s/%s:\n%s\n", kind, client.ObjectKeyFromObject(object), content)
		}
		return nil
	}

	// collect events from the latest pod
	if len(podList.Items) > 0 {
		if err := collectEvents(&podList.Items[0]); err != nil {
			return "", err
		}
	}
	// collect events from the pvc
	if pvc != nil {
		if err := collectEvents(pvc); err != nil {
			return "", err
		}
	}
	// collect events from the job
	if err := collectEvents(job); err != nil {
		return "", err
	}
	return message, nil
}

func (r *BackupRepoReconciler) collectFailedPodLogs(ctx context.Context,
	podList *corev1.PodList, containerName string, limit int64) (string, error) {
	typedCli, err := corev1client.NewForConfig(r.RestConfig)
	if err != nil {
		return "", err
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodFailed {
			currOpts := &corev1.PodLogOptions{
				Container: containerName,
			}
			req := typedCli.Pods(pod.Namespace).GetLogs(pod.Name, currOpts)
			stream, err := req.Stream(ctx)
			if err != nil {
				return "", err
			}
			limited := io.LimitReader(stream, limit)
			data, _ := io.ReadAll(limited)
			return string(data), nil
		}
	}
	return "", nil
}

func (r *BackupRepoReconciler) constructPVCByTemplate(
	reconCtx *reconcileContext, pvc *corev1.PersistentVolumeClaim, tmpl string) error {
	// fill render values
	reconCtx.renderCtx.GeneratedStorageClassName = reconCtx.repo.Status.GeneratedStorageClassName

	content, err := renderTemplate("pvc", tmpl, reconCtx.renderCtx)
	if err != nil {
		return fmt.Errorf("failed to render PVC template: %w", err)
	}
	if err = yaml.Unmarshal([]byte(content), pvc); err != nil {
		return fmt.Errorf("failed to unmarshal PVC object: %w", err)
	}
	return nil
}

func (r *BackupRepoReconciler) listAssociatedBackups(
	ctx context.Context, repo *dpv1alpha1.BackupRepo, extraSelector map[string]string) ([]*dpv1alpha1.Backup, error) {
	// list backups associated with the repo
	backupList := &dpv1alpha1.BackupList{}
	selectors := client.MatchingLabels{
		dataProtectionBackupRepoKey: repo.Name,
	}
	for k, v := range extraSelector {
		selectors[k] = v
	}
	err := r.Client.List(ctx, backupList, selectors)
	var filtered []*dpv1alpha1.Backup
	for idx := range backupList.Items {
		backup := &backupList.Items[idx]
		if backup.Status.Phase == dpv1alpha1.BackupPhaseFailed {
			continue
		}
		filtered = append(filtered, backup)
	}
	return filtered, err
}

func (r *BackupRepoReconciler) prepareForAssociatedBackups(reconCtx *reconcileContext) error {
	backups, err := r.listAssociatedBackups(reconCtx.Ctx, reconCtx.repo, map[string]string{
		dataProtectionWaitRepoPreparationKey: trueVal,
	})
	if err != nil {
		return err
	}
	// return any error to reconcile the repo
	var retErr error
	for _, backup := range backups {
		switch {
		case reconCtx.repo.AccessByMount():
			if _, err := r.createRepoPVC(reconCtx, reconCtx.repo.Status.BackupPVCName, backup.Namespace, nil); err != nil {
				reconCtx.Log.Error(err, "failed to check or create PVC", "namespace", backup.Namespace)
				retErr = err
				continue
			}
		case reconCtx.repo.AccessByTool():
			if _, err := r.createToolConfigSecret(reconCtx, reconCtx.repo.Status.ToolConfigSecretName, backup.Namespace, nil); err != nil {
				reconCtx.Log.Error(err, "failed to check or create tool config secret", "namespace", backup.Namespace)
				retErr = err
				continue
			}
		default:
			retErr = fmt.Errorf("unknown access method: %s", reconCtx.repo.Spec.AccessMethod)
		}

		if backup.Labels[dataProtectionWaitRepoPreparationKey] != "" {
			patch := client.MergeFrom(backup.DeepCopy())
			delete(backup.Labels, dataProtectionWaitRepoPreparationKey)
			if err = r.Client.Patch(reconCtx.Ctx, backup, patch); err != nil {
				reconCtx.Log.Error(err, "failed to patch backup",
					"backup", client.ObjectKeyFromObject(backup))
				retErr = err
				continue
			}
		}
	}
	return retErr
}

func (r *BackupRepoReconciler) createRepoPVC(reconCtx *reconcileContext,
	name, namespace string, extraAnnos map[string]string) (*corev1.PersistentVolumeClaim, error) {

	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Name = name
	pvc.Namespace = namespace
	_, err := createObjectIfNotExist(reconCtx.Ctx, r.Client, pvc,
		func() error {
			if reconCtx.provider.Spec.PersistentVolumeClaimTemplate != "" {
				// construct the PVC object by rendering the template
				err := r.constructPVCByTemplate(reconCtx, pvc, reconCtx.provider.Spec.PersistentVolumeClaimTemplate)
				if err != nil {
					return err
				}
				// overwrite PVC name and namespace
				pvc.Name = name
				pvc.Namespace = namespace
			} else {
				// set storage class name to PVC, other fields will be set with default value later
				storageClassName := reconCtx.repo.Status.GeneratedStorageClassName
				pvc.Spec = corev1.PersistentVolumeClaimSpec{
					StorageClassName: &storageClassName,
				}
			}
			// add a referencing label
			if pvc.Labels == nil {
				pvc.Labels = make(map[string]string)
			}
			pvc.Labels[dataProtectionBackupRepoKey] = reconCtx.repo.Name
			// extra annotations
			if pvc.Annotations == nil {
				pvc.Annotations = make(map[string]string)
			}
			for k, v := range extraAnnos {
				pvc.Annotations[k] = v
			}
			// set default values if not set
			if len(pvc.Spec.AccessModes) == 0 {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
			}
			if pvc.Spec.VolumeMode == nil {
				volumeMode := corev1.PersistentVolumeFilesystem
				pvc.Spec.VolumeMode = &volumeMode
			}
			if pvc.Spec.Resources.Requests == nil {
				pvc.Spec.Resources.Requests = corev1.ResourceList{}
			}
			// note: pvc.Spec.Resources.Requests.Storage() never returns nil
			if pvc.Spec.Resources.Requests.Storage().IsZero() {
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = reconCtx.repo.Spec.VolumeCapacity
			}
			if err := controllerutil.SetControllerReference(reconCtx.repo, pvc, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			return nil
		})

	return pvc, err
}

func constructToolConfigSecret(secret *corev1.Secret, content string) {
	secret.Data = map[string][]byte{
		"datasafed.conf": []byte(content),
	}
}

func (r *BackupRepoReconciler) createToolConfigSecret(reconCtx *reconcileContext,
	name, namespace string, extraAnnos map[string]string) (*corev1.Secret, error) {

	secret := &corev1.Secret{}
	secret.Name = name
	secret.Namespace = namespace
	_, err := createObjectIfNotExist(reconCtx.Ctx, r.Client, secret,
		func() error {
			content, err := renderTemplate("tool-config", reconCtx.provider.Spec.DatasafedConfigTemplate, reconCtx.renderCtx)
			if err != nil {
				return fmt.Errorf("failed to render tool config template: %w", err)
			}
			constructToolConfigSecret(secret, content)

			// add a referencing label
			secret.Labels = map[string]string{
				dataProtectionBackupRepoKey:   reconCtx.repo.Name,
				dataProtectionIsToolConfigKey: trueVal,
			}
			secret.Annotations = map[string]string{
				dataProtectionBackupRepoDigestAnnotationKey: reconCtx.getDigest(),
			}
			for k, v := range extraAnnos {
				secret.Annotations[k] = v
			}
			if err := controllerutil.SetControllerReference(reconCtx.repo, secret, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			return nil
		})

	return secret, err
}

func (r *BackupRepoReconciler) collectParameters(
	reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) (map[string]string, error) {
	values := make(map[string]string)
	for k, v := range repo.Spec.Config {
		values[k] = v
	}
	// merge with secret values
	if repo.Spec.Credential != nil {
		secretObj := &corev1.Secret{}
		err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: repo.Spec.Credential.Namespace,
			Name:      repo.Spec.Credential.Name,
		}, secretObj)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret: %w", err)
		}
		for k, v := range secretObj.Data {
			values[k] = string(v)
		}
	}
	return values, nil
}

func (r *BackupRepoReconciler) deleteExternalResources(
	reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) error {
	// set phase to deleting, so no new Backup can reference to this repo
	if repo.Status.Phase != dpv1alpha1.BackupRepoDeleting {
		patch := client.MergeFrom(repo.DeepCopy())
		repo.Status.Phase = dpv1alpha1.BackupRepoDeleting
		if err := r.Client.Status().Patch(reqCtx.Ctx, repo, patch); err != nil {
			return err
		}
	}

	// TODO: block deletion if any BackupPolicy is referencing to this repo

	// check if the repo is still being used by any backup
	if backups, err := r.listAssociatedBackups(reqCtx.Ctx, repo, nil); err != nil {
		return err
	} else if len(backups) > 0 {
		_ = updateCondition(reqCtx.Ctx, r.Client, repo, ConditionTypeDerivedObjectsDeleted,
			metav1.ConditionFalse, ReasonHaveAssociatedBackups,
			"some backups still refer to this repo")
		return fmt.Errorf("some backups still refer to this repo")
	}

	// delete pre-check jobs
	if err := r.deleteJobs(reqCtx, repo); err != nil {
		return err
	}

	// delete PVCs
	if cleared, err := r.deletePVCs(reqCtx, repo); err != nil {
		return err
	} else if !cleared {
		_ = updateCondition(reqCtx.Ctx, r.Client, repo, ConditionTypeDerivedObjectsDeleted,
			metav1.ConditionFalse, ReasonHaveResidualPVCs,
			"maybe the derived PVCs are still in use")
		return fmt.Errorf("derived PVCs are still in use")
	}

	// delete derived storage classes
	if err := r.deleteStorageClasses(reqCtx, repo); err != nil {
		return err
	}

	// delete derived secrets (secret for CSI and tool configs)
	if err := r.deleteSecrets(reqCtx, repo); err != nil {
		return err
	}

	// update condition status
	err := updateCondition(reqCtx.Ctx, r.Client, repo, ConditionTypeDerivedObjectsDeleted,
		metav1.ConditionTrue, ReasonDerivedObjectsDeleted, "")
	if err != nil {
		return fmt.Errorf("failed to update condition: %w", err)
	}

	// maintain mappers
	r.secretRefMapper.removeRef(repo)
	r.providerRefMapper.removeRef(repo)

	return nil
}

func (r *BackupRepoReconciler) deleteJobs(reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) error {
	jobList := &batchv1.JobList{}
	if err := r.Client.List(reqCtx.Ctx, jobList,
		client.MatchingLabels(map[string]string{
			dataProtectionBackupRepoKey: repo.Name,
		})); err != nil {
		return fmt.Errorf("failed to list Jobs: %w", err)
	}

	for _, job := range jobList.Items {
		if !isOwned(repo, &job) {
			continue
		}
		reqCtx.Log.Info("deleting job", "name", job.Name, "namespace", job.Namespace)
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &job); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupRepoReconciler) deletePVCs(reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) (cleared bool, err error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(reqCtx.Ctx, pvcList,
		client.MatchingLabels(map[string]string{
			dataProtectionBackupRepoKey: repo.Name,
		})); err != nil {
		return false, fmt.Errorf("failed to list PVCs: %w", err)
	}

	for _, pvc := range pvcList.Items {
		if !isOwned(repo, &pvc) {
			continue
		}
		reqCtx.Log.Info("deleting PVC", "name", pvc.Name, "namespace", pvc.Namespace)
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &pvc); err != nil {
			return false, err
		}
	}
	// make sure all derived PVCs are deleted
	cleared = true
	for _, pvc := range pvcList.Items {
		if !isOwned(repo, &pvc) {
			continue
		}
		err = r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(&pvc), &corev1.PersistentVolumeClaim{})
		if !apierrors.IsNotFound(err) {
			cleared = false
			break
		}
	}
	return cleared, nil
}

func (r *BackupRepoReconciler) deleteStorageClasses(reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) error {
	scList := &storagev1.StorageClassList{}
	if err := r.Client.List(reqCtx.Ctx, scList,
		client.MatchingLabels(map[string]string{
			dataProtectionBackupRepoKey: repo.Name,
		})); err != nil {
		return fmt.Errorf("failed to list StorageClasses: %w", err)
	}

	for _, sc := range scList.Items {
		if !isOwned(repo, &sc) {
			continue
		}
		reqCtx.Log.Info("deleting StorageClass", "storageclass", sc.Name)
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &sc); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupRepoReconciler) deleteSecrets(reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) error {
	secretList := &corev1.SecretList{}
	if err := r.Client.List(reqCtx.Ctx, secretList,
		client.MatchingLabels(map[string]string{
			dataProtectionBackupRepoKey: repo.Name,
		})); err != nil {
		return fmt.Errorf("failed to list Secret: %w", err)
	}

	for _, secret := range secretList.Items {
		if !isOwned(repo, &secret) {
			continue
		}
		reqCtx.Log.Info("deleting Secret", "secret", client.ObjectKeyFromObject(&secret))
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &secret); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupRepoReconciler) mapBackupToRepo(ctx context.Context, obj client.Object) []ctrl.Request {
	backup := obj.(*dpv1alpha1.Backup)
	repoName, ok := backup.Labels[dataProtectionBackupRepoKey]
	if !ok {
		return nil
	}
	// ignore failed backups
	if backup.Status.Phase == dpv1alpha1.BackupPhaseFailed {
		return nil
	}
	// we should reconcile the BackupRepo when:
	//   1. the Backup needs to use the BackupRepo, but it's not ready for the namespace.
	//   2. the Backup is being deleted, because it may block the deletion of the BackupRepo.
	shouldReconcileRepo := backup.Labels[dataProtectionWaitRepoPreparationKey] == trueVal ||
		!backup.DeletionTimestamp.IsZero()
	if shouldReconcileRepo {
		return []ctrl.Request{{
			NamespacedName: client.ObjectKey{Name: repoName},
		}}
	}
	return nil
}

func (r *BackupRepoReconciler) mapProviderToRepos(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.providerRefMapper.mapToRequests(obj)
}

func (r *BackupRepoReconciler) mapSecretToRepos(ctx context.Context, obj client.Object) []ctrl.Request {
	// check if the secret is created by this controller
	owner := metav1.GetControllerOf(obj)
	if owner != nil {
		apiGVStr := dpv1alpha1.GroupVersion.String()
		if owner.APIVersion == apiGVStr && owner.Kind == "BackupRepo" {
			return []ctrl.Request{{
				NamespacedName: types.NamespacedName{
					Name:      owner.Name,
					Namespace: obj.GetNamespace(),
				},
			}}
		}
	}

	// get repos which is referencing this secret
	return r.secretRefMapper.mapToRequests(obj)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupRepoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Event{}, "involvedObject.uid", func(rawObj client.Object) []string {
		event := rawObj.(*corev1.Event)
		return []string{string(event.InvolvedObject.UID)}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.BackupRepo{}).
		Watches(&storagev1alpha1.StorageProvider{}, handler.EnqueueRequestsFromMapFunc(r.mapProviderToRepos)).
		Watches(&dpv1alpha1.Backup{}, handler.EnqueueRequestsFromMapFunc(r.mapBackupToRepo)).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.mapSecretToRepos)).
		Owns(&storagev1.StorageClass{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// ============================================================================
// helper functions
// ============================================================================

// dependencyError indicates that the error itself cannot be resolved
// unless the dependent object is updated.
type dependencyError struct {
	msg string
}

func (e *dependencyError) Error() string {
	return e.msg
}

func newDependencyError(msg string) error {
	return &dependencyError{msg: msg}
}

func isDependencyError(err error) bool {
	de, ok := err.(*dependencyError)
	return ok || errors.As(err, &de)
}

func checkedRequeueWithError(err error, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if re, ok := err.(intctrlutil.RequeueError); ok {
		return intctrlutil.RequeueAfter(re.RequeueAfter(), logger, re.Reason())
	}
	if apierrors.IsNotFound(err) || isDependencyError(err) {
		return intctrlutil.Reconciled()
	}
	return intctrlutil.RequeueWithError(err, logger, msg, keysAndValues...)
}

type renderContext struct {
	Parameters                map[string]string
	CSIDriverSecretRef        corev1.SecretReference
	GeneratedStorageClassName string
}

func renderTemplate(name, tpl string, rCtx renderContext) (string, error) {
	fmap := sprig.TxtFuncMap()
	t, err := template.New(name).Funcs(fmap).Parse(tpl)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	err = t.Execute(&b, rCtx)
	return b.String(), err
}

func createOrUpdateObject[T any, PT generics.PObject[T]](
	ctx context.Context,
	c client.Client,
	obj PT,
	mutateFunc func() error,
	shouldUpdate func() bool) (created bool, err error) {
	key := client.ObjectKeyFromObject(obj)
	err = c.Get(ctx, key, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to check existence of object %s: %w", key, err)
	}
	var patch client.Patch
	if err == nil {
		// object already exists, check if it needs to be updated
		if !shouldUpdate() {
			return false, nil
		}
		patch = client.MergeFrom(PT(obj.DeepCopy()))
	}
	if mutateFunc != nil {
		err := mutateFunc()
		if err != nil {
			return false, err
		}
	}
	if patch != nil {
		err = c.Patch(ctx, obj, patch)
		if err != nil {
			err = fmt.Errorf("failed to patch object %s: %w", key, err)
		}
		return false, err
	} else {
		err = c.Create(ctx, obj)
		if err != nil {
			return false, fmt.Errorf("failed to create object %s: %w", key, err)
		}
		return true, nil
	}
}

func createObjectIfNotExist[T any, PT generics.PObject[T]](
	ctx context.Context,
	c client.Client,
	obj PT,
	mutateFunc func() error) (created bool, err error) {
	noUpdate := func() bool { return false }
	return createOrUpdateObject(ctx, c, obj, mutateFunc, noUpdate)
}

func setCondition(
	repo *dpv1alpha1.BackupRepo, condType string, status metav1.ConditionStatus,
	reason string, message string) {
	cond := metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: repo.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	meta.SetStatusCondition(&repo.Status.Conditions, cond)
}

func updateCondition(
	ctx context.Context, c client.Client, repo *dpv1alpha1.BackupRepo,
	condType string, status metav1.ConditionStatus, reason string, message string) error {
	cond := meta.FindStatusCondition(repo.Status.Conditions, condType)
	if cond != nil {
		// skip
		if cond.Status == status && cond.Reason == reason && cond.Message == message {
			return nil
		}
	}
	patch := client.MergeFrom(repo.DeepCopy())
	setCondition(repo, condType, status, reason, message)
	return c.Status().Patch(ctx, repo, patch)
}

func updateAnnotations(ctx context.Context, c client.Client,
	repo *dpv1alpha1.BackupRepo, annotations map[string]string) error {
	patch := client.MergeFrom(repo.DeepCopy())
	if repo.Annotations == nil {
		repo.Annotations = make(map[string]string)
	}
	updated := false
	for k, v := range annotations {
		if curr, ok := repo.Annotations[k]; !ok || curr != v {
			repo.Annotations[k] = v
			updated = true
		}
	}
	if !updated {
		return nil
	}
	return c.Patch(ctx, repo, patch)
}

func md5Digest(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func stableSerializeMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sb := strings.Builder{}
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(m[k])
		sb.WriteByte(';')
	}
	return sb.String()
}

func isOwned(owner client.Object, dependent client.Object) bool {
	ownerUID := owner.GetUID()
	for _, ref := range dependent.GetOwnerReferences() {
		if ref.UID == ownerUID {
			return true
		}
	}
	return false
}

func randomNameForDerivedObject(repo *dpv1alpha1.BackupRepo, prefix string) string {
	// the final name should not exceed 63 characters
	const maxBaseNameLength = 56
	baseName := fmt.Sprintf("%s-%s", prefix, repo.Name)
	if len(baseName) > maxBaseNameLength {
		baseName = baseName[:maxBaseNameLength]
	}
	return baseName + "-" + rand.String(6)
}

func cutName(name string) string {
	if len(name) > 63 {
		return name[:63]
	}
	return name
}

// this method requires the corresponding field index to be added to the Manager
func fetchObjectEvents(ctx context.Context, cli client.Client, object client.Object) (*corev1.EventList, error) {
	eventList := &corev1.EventList{}
	err := cli.List(ctx, eventList, client.MatchingFields{
		"involvedObject.uid": string(object.GetUID()),
	})
	if err != nil {
		return nil, err
	}
	return eventList, nil
}
