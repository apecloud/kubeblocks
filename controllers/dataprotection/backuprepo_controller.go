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
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

// BackupRepoReconciler reconciles a BackupRepo object
type BackupRepoReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

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
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to get BackupRepo")
	}

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, repo, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
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

	// check storage provider status
	provider, err := r.checkStorageProviderStatus(reqCtx, repo)
	if err != nil {
		_ = r.updateStatus(reqCtx, repo)
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "check storage provider status failed")
	}
	if !meta.IsStatusConditionTrue(repo.Status.Conditions, ConditionTypeStorageProviderReady) {
		// update status phase to failed
		if err := r.updateStatus(reqCtx, repo); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "update status phase failed")
		}
		// will reconcile again after the storage provider becomes ready
		return intctrlutil.Reconciled()
	}

	// create StorageClass and Secret for the CSI driver
	err = r.createStorageClassAndSecret(reqCtx, repo, provider)
	if err != nil {
		_ = r.updateStatus(reqCtx, repo)
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log,
			"failed to create storage class and secret")
	}

	// TODO: implement pre-check logic
	//  1. try to create a PVC and observe its status
	//  2. create a pre-check job, mount with the PVC and check job status
	//  3. pre-check again if the secret object for CSI got updated

	// update status phase to ready if all conditions are met
	if err = r.updateStatus(reqCtx, repo); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log,
			"failed to update BackupRepo status")
	}

	// check associated backups, to create PVC in their namespaces
	if repo.Status.Phase == dpv1alpha1.BackupRepoReady {
		if err = r.createPVCForAssociatedBackups(reqCtx, repo); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log,
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
		if meta.IsStatusConditionTrue(repo.Status.Conditions, ConditionTypeStorageProviderReady) &&
			meta.IsStatusConditionTrue(repo.Status.Conditions, ConditionTypeStorageClassCreated) {
			phase = dpv1alpha1.BackupRepoReady
		}
		repo.Status.Phase = phase
	}
	repo.Status.IsDefault = repo.Annotations[dptypes.DefaultBackupRepoAnnotationKey] == trueVal

	if !reflect.DeepEqual(old.Status, repo.Status) {
		if err := r.Client.Status().Patch(reqCtx.Ctx, repo, client.MergeFrom(old)); err != nil {
			return fmt.Errorf("updateStatus failed: %w", err)
		}
	}
	return nil
}

func (r *BackupRepoReconciler) checkStorageProviderStatus(
	reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) (*storagev1alpha1.StorageProvider, error) {
	var condType = ConditionTypeStorageProviderReady
	var status metav1.ConditionStatus
	var reason string
	var message string

	// get storage provider object
	providerKey := client.ObjectKey{Name: repo.Spec.StorageProviderRef}
	provider := &storagev1alpha1.StorageProvider{}
	err := r.Client.Get(reqCtx.Ctx, providerKey, provider)
	if err != nil {
		if apierrors.IsNotFound(err) {
			status = metav1.ConditionFalse
			reason = ReasonStorageProviderNotFound
		} else {
			status = metav1.ConditionUnknown
			reason = ReasonUnknownError
			message = err.Error()
		}
		_ = updateCondition(reqCtx.Ctx, r.Client, repo, condType, status, reason, message)
		return nil, err
	}

	// check its status
	if provider.Status.Phase == storagev1alpha1.StorageProviderReady {
		status = metav1.ConditionTrue
		reason = ReasonStorageProviderReady
	} else {
		status = metav1.ConditionFalse
		reason = ReasonStorageProviderNotReady
		message = fmt.Sprintf("storage provider %s is not ready, status: %s",
			provider.Name, provider.Status.Phase)
	}
	if updateErr := updateCondition(reqCtx.Ctx, r.Client, repo, condType, status, reason, message); updateErr != nil {
		return nil, updateErr
	}
	return provider, nil
}

func (r *BackupRepoReconciler) createStorageClassAndSecret(
	reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo, provider *storagev1alpha1.StorageProvider) error {

	// collect parameters for rendering templates
	parameters, err := r.collectParameters(reqCtx, repo)
	if err != nil {
		_ = updateCondition(reqCtx.Ctx, r.Client, repo, ConditionTypeStorageClassCreated,
			metav1.ConditionUnknown, ReasonUnknownError, err.Error())
		return fmt.Errorf("failed to collect render parameters: %w", err)
	}
	// TODO: verify parameters
	renderCtx := renderContext{
		Parameters: parameters,
	}
	oldRepo := repo.DeepCopy()

	// create secret for the CSI driver if it's not exist,
	// or update the secret if the template or values are updated
	if provider.Spec.CSIDriverSecretTemplate != "" {
		if repo.Status.GeneratedCSIDriverSecret == nil {
			repo.Status.GeneratedCSIDriverSecret = &corev1.SecretReference{
				Name:      randomNameForDerivedObject(repo, "secret"),
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
			}
		}
		renderCtx.CSIDriverSecretRef = *repo.Status.GeneratedCSIDriverSecret
		// create secret if it's not exist
		if _, err := r.createSecretForCSIDriver(reqCtx, renderCtx, repo, provider); err != nil {
			return err
		}
	}

	// create storage class if it's not exist
	if repo.Status.GeneratedStorageClassName == "" {
		repo.Status.GeneratedStorageClassName = randomNameForDerivedObject(repo, "sc")
	}
	if _, err := r.createStorageClass(reqCtx, renderCtx, repo, provider); err != nil {
		return err
	}

	// update other fields
	if repo.Status.BackupPVCName == "" {
		repo.Status.BackupPVCName = randomNameForDerivedObject(repo, "pvc")
	}
	if repo.Status.ObservedGeneration != repo.Generation {
		repo.Status.ObservedGeneration = repo.Generation
	}
	if !meta.IsStatusConditionTrue(repo.Status.Conditions, ConditionTypeStorageClassCreated) {
		setCondition(repo, ConditionTypeStorageClassCreated,
			metav1.ConditionTrue, ReasonStorageClassCreated, "")
	}

	if !reflect.DeepEqual(oldRepo.Status, repo.Status) {
		err := r.Client.Status().Patch(reqCtx.Ctx, repo, client.MergeFrom(oldRepo))
		if err != nil {
			return fmt.Errorf("failed to patch backup repo: %w", err)
		}
	}
	return nil
}

func (r *BackupRepoReconciler) createSecretForCSIDriver(
	reqCtx intctrlutil.RequestCtx, renderCtx renderContext,
	repo *dpv1alpha1.BackupRepo, provider *storagev1alpha1.StorageProvider) (created bool, err error) {

	secretTemplateMD5 := md5Digest(provider.Spec.CSIDriverSecretTemplate)
	templateValuesMD5 := md5Digest(stableSerializeMap(renderCtx.Parameters))
	condType := ConditionTypeStorageClassCreated
	setSecretContent := func(secret *corev1.Secret) error {
		// render secret template
		content, err := renderTemplate("secret", provider.Spec.CSIDriverSecretTemplate, renderCtx)
		if err != nil {
			_ = updateCondition(reqCtx.Ctx, r.Client, repo, condType,
				metav1.ConditionFalse, ReasonBadSecretTemplate, err.Error())
			return fmt.Errorf("failed to render secret template: %w", err)
		}
		secretStringData := map[string]string{}
		if err = yaml.Unmarshal([]byte(content), &secretStringData); err != nil {
			_ = updateCondition(reqCtx.Ctx, r.Client, repo, condType,
				metav1.ConditionFalse, ReasonBadSecretTemplate, err.Error())
			return fmt.Errorf("failed to unmarshal secret content: %w", err)
		}
		secretData := make(map[string][]byte, len(secretStringData))
		for k, v := range secretStringData {
			secretData[k] = []byte(v)
		}
		secret.Data = secretData
		return nil
	}

	secret := &corev1.Secret{}
	secret.Name = repo.Status.GeneratedCSIDriverSecret.Name
	secret.Namespace = repo.Status.GeneratedCSIDriverSecret.Namespace

	// create the secret object if not exist.
	// this function will retrieve the whole secret object
	// when the object is existing.
	created, err = createObjectIfNotExist(reqCtx.Ctx, r.Client, secret,
		func() error {
			secret.Labels = map[string]string{
				dataProtectionBackupRepoKey: repo.Name,
			}
			secret.Annotations = map[string]string{
				dataProtectionSecretTemplateMD5AnnotationKey: secretTemplateMD5,
				dataProtectionTemplateValuesMD5AnnotationKey: templateValuesMD5,
			}
			if err := setSecretContent(secret); err != nil {
				return err
			}
			if err := controllerutil.SetControllerReference(repo, secret, r.Scheme); err != nil {
				_ = updateCondition(reqCtx.Ctx, r.Client, repo, condType,
					metav1.ConditionUnknown, ReasonUnknownError, err.Error())
				return fmt.Errorf("failed to set controller reference: %w", err)
			}
			return nil
		})
	if err != nil {
		return false, fmt.Errorf("createObjectIfNotExist for secret %s failed: %w",
			client.ObjectKeyFromObject(secret), err)
	}
	if created {
		return true, nil
	}

	// check if the template or config changed, then update the secret
	currSecretTemplateMD5 := secret.Annotations[dataProtectionSecretTemplateMD5AnnotationKey]
	currTemplateValuesMD5 := secret.Annotations[dataProtectionTemplateValuesMD5AnnotationKey]
	if currSecretTemplateMD5 != secretTemplateMD5 || currTemplateValuesMD5 != templateValuesMD5 {
		patch := client.MergeFrom(secret.DeepCopy())
		if err := setSecretContent(secret); err != nil {
			return false, err
		}
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		secret.Annotations[dataProtectionSecretTemplateMD5AnnotationKey] = secretTemplateMD5
		secret.Annotations[dataProtectionTemplateValuesMD5AnnotationKey] = templateValuesMD5
		err := r.Client.Patch(reqCtx.Ctx, secret, patch)
		if err != nil {
			return false, fmt.Errorf("failed to patch secret object %s: %w",
				client.ObjectKeyFromObject(secret), err)
		}
	}
	return false, nil
}

func (r *BackupRepoReconciler) createStorageClass(
	reqCtx intctrlutil.RequestCtx, renderCtx renderContext,
	repo *dpv1alpha1.BackupRepo, provider *storagev1alpha1.StorageProvider) (created bool, err error) {

	storageClass := &storagev1.StorageClass{}
	storageClass.Name = repo.Status.GeneratedStorageClassName
	return createObjectIfNotExist(reqCtx.Ctx, r.Client, storageClass,
		func() error {
			condType := ConditionTypeStorageClassCreated

			// render storage class template
			content, err := renderTemplate("sc", provider.Spec.StorageClassTemplate, renderCtx)
			if err != nil {
				_ = updateCondition(reqCtx.Ctx, r.Client, repo, condType,
					metav1.ConditionFalse, ReasonBadStorageClassTemplate, err.Error())
				return fmt.Errorf("failed to render storage class template: %w", err)
			}
			if err = yaml.Unmarshal([]byte(content), storageClass); err != nil {
				_ = updateCondition(reqCtx.Ctx, r.Client, repo, condType,
					metav1.ConditionFalse, ReasonBadStorageClassTemplate, err.Error())
				return fmt.Errorf("failed to unmarshal storage class: %w", err)
			}

			// create storage class object
			storageClass.Labels = map[string]string{
				dataProtectionBackupRepoKey: repo.Name,
			}
			bindingMode := storagev1.VolumeBindingImmediate
			storageClass.VolumeBindingMode = &bindingMode
			if repo.Spec.PVReclaimPolicy != "" {
				storageClass.ReclaimPolicy = &repo.Spec.PVReclaimPolicy
			}
			if err := controllerutil.SetControllerReference(repo, storageClass, r.Scheme); err != nil {
				_ = updateCondition(reqCtx.Ctx, r.Client, repo, condType,
					metav1.ConditionUnknown, ReasonUnknownError, err.Error())
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			return nil
		})
}

func (r *BackupRepoReconciler) listAssociatedBackups(
	reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo, extraSelector map[string]string) ([]*dpv1alpha1.Backup, error) {
	// list backups associated with the repo
	backupList := &dpv1alpha1.BackupList{}
	selectors := client.MatchingLabels{
		dataProtectionBackupRepoKey: repo.Name,
	}
	for k, v := range extraSelector {
		selectors[k] = v
	}
	err := r.Client.List(reqCtx.Ctx, backupList, selectors)
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

func (r *BackupRepoReconciler) createPVCForAssociatedBackups(
	reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) error {
	backups, err := r.listAssociatedBackups(reqCtx, repo, map[string]string{
		dataProtectionNeedRepoPVCKey: trueVal,
	})
	if err != nil {
		return err
	}
	// return any error to reconcile the repo
	var retErr error
	for _, backup := range backups {
		if err := r.checkOrCreatePVC(reqCtx, repo, backup.Namespace); err != nil {
			reqCtx.Log.Error(err, "failed to check or create PVC", "namespace", backup.Namespace)
			retErr = err
			continue
		}
		if backup.Labels[dataProtectionNeedRepoPVCKey] != "" {
			patch := client.MergeFrom(backup.DeepCopy())
			delete(backup.Labels, dataProtectionNeedRepoPVCKey)
			if err = r.Client.Patch(reqCtx.Ctx, backup, patch); err != nil {
				reqCtx.Log.Error(err, "failed to patch backup",
					"backup", client.ObjectKeyFromObject(backup))
				retErr = err
				continue
			}
		}
	}
	return retErr
}

func (r *BackupRepoReconciler) checkOrCreatePVC(
	reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo, namespace string) error {
	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Name = repo.Status.BackupPVCName
	pvc.Namespace = namespace
	_, err := createObjectIfNotExist(reqCtx.Ctx, r.Client, pvc,
		func() error {
			storageClassName := repo.Status.GeneratedStorageClassName
			volumeMode := corev1.PersistentVolumeFilesystem
			resources := corev1.ResourceRequirements{}
			if !repo.Spec.VolumeCapacity.IsZero() {
				resources.Requests = corev1.ResourceList{
					corev1.ResourceStorage: repo.Spec.VolumeCapacity,
				}
			}
			pvc.Labels = map[string]string{
				dataProtectionBackupRepoKey: repo.Name,
			}
			pvc.Spec = corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteMany,
				},
				Resources:        resources,
				StorageClassName: &storageClassName,
				VolumeMode:       &volumeMode,
			}
			if err := controllerutil.SetControllerReference(repo, pvc, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			return nil
		})

	return err
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
	if backups, err := r.listAssociatedBackups(reqCtx, repo, nil); err != nil {
		return err
	} else if len(backups) > 0 {
		_ = updateCondition(reqCtx.Ctx, r.Client, repo, ConditionTypeDerivedObjectsDeleted,
			metav1.ConditionFalse, ReasonHaveAssociatedBackups,
			"some backups still refer to this repo")
		return fmt.Errorf("some backups still refer to this repo")
	}

	// delete PVCs
	if clear, err := r.deletePVCs(reqCtx, repo); err != nil {
		return err
	} else if !clear {
		_ = updateCondition(reqCtx.Ctx, r.Client, repo, ConditionTypeDerivedObjectsDeleted,
			metav1.ConditionFalse, ReasonHaveResidualPVCs,
			"maybe the derived PVCs are still in use")
		return fmt.Errorf("derived PVCs are still in use")
	}

	// delete derived storage classes
	if err := r.deleteStorageClasses(reqCtx, repo); err != nil {
		return err
	}

	// delete derived secrets
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

func (r *BackupRepoReconciler) deletePVCs(reqCtx intctrlutil.RequestCtx, repo *dpv1alpha1.BackupRepo) (clear bool, err error) {
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
	clear = true
	for _, pvc := range pvcList.Items {
		if !isOwned(repo, &pvc) {
			continue
		}
		err = r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(&pvc), &corev1.PersistentVolumeClaim{})
		if !apierrors.IsNotFound(err) {
			clear = false
			break
		}
	}
	return clear, nil
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

func (r *BackupRepoReconciler) mapBackupToRepo(obj client.Object) []ctrl.Request {
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
	//   1. the Backup needs a PVC which is not present and should be created by the BackupRepo.
	//   2. the Backup is being deleted, because it may block the deletion of the BackupRepo.
	shouldReconcileRepo := backup.Labels[dataProtectionNeedRepoPVCKey] == trueVal ||
		!backup.DeletionTimestamp.IsZero()
	if shouldReconcileRepo {
		return []ctrl.Request{{
			NamespacedName: client.ObjectKey{Name: repoName},
		}}
	}
	return nil
}

func (r *BackupRepoReconciler) mapProviderToRepos(obj client.Object) []ctrl.Request {
	return r.providerRefMapper.mapToRequests(obj)
}

func (r *BackupRepoReconciler) mapSecretToRepos(obj client.Object) []ctrl.Request {
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.BackupRepo{}).
		Watches(&source.Kind{Type: &storagev1alpha1.StorageProvider{}},
			handler.EnqueueRequestsFromMapFunc(r.mapProviderToRepos)).
		Watches(&source.Kind{Type: &dpv1alpha1.Backup{}},
			handler.EnqueueRequestsFromMapFunc(r.mapBackupToRepo)).
		Watches(&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.mapSecretToRepos)).
		Owns(&storagev1.StorageClass{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

// ============================================================================
// helper functions
// ============================================================================

type renderContext struct {
	Parameters         map[string]string
	CSIDriverSecretRef corev1.SecretReference
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

func createObjectIfNotExist(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	mutateFunc func() error) (created bool, err error) {
	key := client.ObjectKeyFromObject(obj)
	err = c.Get(ctx, key, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to check existence of object: %w", err)
	}
	if err == nil {
		// already exists
		return false, nil
	}
	if mutateFunc != nil {
		err := mutateFunc()
		if err != nil {
			return false, err
		}
	}
	err = c.Create(ctx, obj)
	if err != nil {
		return false, fmt.Errorf("failed to create object %s: %w",
			client.ObjectKeyFromObject(obj), err)
	}
	return true, nil
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
