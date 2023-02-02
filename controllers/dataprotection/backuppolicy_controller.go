/*
Copyright ApeCloud, Inc.

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

package dataprotection

import (
	"context"
	"embed"
	"encoding/json"
	"sort"
	"time"

	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// BackupPolicyReconciler reconciles a BackupPolicy object
type BackupPolicyReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

type backupPolicyOptions struct {
	Name       string           `json:"name"`
	Namespace  string           `json:"namespace"`
	Cluster    string           `json:"cluster"`
	Schedule   string           `json:"schedule"`
	BackupType string           `json:"backupType"`
	TTL        *metav1.Duration `json:"ttl"`
}

var (
	//go:embed cue/*
	cueTemplates embed.FS
)

//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/finalizers,verbs=update

// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=cronjobs/finalizers,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BackupPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *BackupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// NOTES:
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backupPolicy", req.NamespacedName),
		Recorder: r.Recorder,
	}

	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupPolicy, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backupPolicy)
	})
	if res != nil {
		return *res, err
	}

	switch backupPolicy.Status.Phase {
	case "", dataprotectionv1alpha1.ConfigNew:
		return r.doNewPhaseAction(reqCtx, backupPolicy)
	case dataprotectionv1alpha1.ConfigInProgress:
		return r.doInProgressPhaseAction(reqCtx, backupPolicy)
	case dataprotectionv1alpha1.ConfigAvailable:
		return r.doAvailablePhaseAction(reqCtx, backupPolicy)
	default:
		return intctrlutil.Reconciled()
	}
}

func (r *BackupPolicyReconciler) doNewPhaseAction(
	reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) (ctrl.Result, error) {
	// update status phase
	backupPolicy.Status.Phase = dataprotectionv1alpha1.ConfigInProgress
	if err := r.Client.Status().Update(reqCtx.Ctx, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
}

func (r *BackupPolicyReconciler) doInProgressPhaseAction(
	reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) (ctrl.Result, error) {
	// update default value from viper config if necessary
	patch := client.MergeFrom(backupPolicy.DeepCopy())

	// merge backup policy template spec
	if err := r.mergeBackupPolicyTemplate(reqCtx, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if len(backupPolicy.Spec.Schedule) == 0 {
		schedule := viper.GetString("DP_BACKUP_SCHEDULE")
		if len(schedule) > 0 {
			backupPolicy.Spec.Schedule = schedule
		}
	}
	if backupPolicy.Spec.TTL == nil {
		ttlString := viper.GetString("DP_BACKUP_TTL")
		if len(ttlString) > 0 {
			ttl, err := time.ParseDuration(ttlString)
			if err == nil {
				backupPolicy.Spec.TTL = &metav1.Duration{Duration: ttl}
			}
		}
	}
	for k, v := range backupPolicy.Spec.Target.LabelsSelector.MatchLabels {
		if backupPolicy.Labels == nil {
			backupPolicy.SetLabels(map[string]string{})
		}
		backupPolicy.Labels[k] = v
	}

	if err := r.Client.Patch(reqCtx.Ctx, backupPolicy, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// if backup policy is available, try to remove expired or oldest backups
	if backupPolicy.Status.Phase == dataprotectionv1alpha1.ConfigAvailable {
		if err := r.removeExpiredBackups(reqCtx); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if err := r.removeOldestBackups(reqCtx, backupPolicy); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	// create cronjob from cue template.
	cronjob, err := r.buildCronJob(backupPolicy)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	err = r.Client.Create(reqCtx.Ctx, cronjob)
	// ignore already exists.
	if err != nil && !errors.IsAlreadyExists(err) {
		r.Recorder.Eventf(backupPolicy, corev1.EventTypeWarning, "CreatingBackupPolicy",
			"Failed to create cronjob %s.", err.Error())
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// update status phase
	backupPolicy.Status.Phase = dataprotectionv1alpha1.ConfigAvailable
	if err := r.Client.Status().Update(reqCtx.Ctx, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
}

func (r *BackupPolicyReconciler) doAvailablePhaseAction(
	reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) (ctrl.Result, error) {
	// patch cronjob if backup policy spec patched
	if err := r.patchCronJob(reqCtx, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// try to remove expired or oldest backups, triggered by cronjob controller
	if err := r.removeExpiredBackups(reqCtx); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if err := r.removeOldestBackups(reqCtx, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupPolicyReconciler) mergeBackupPolicyTemplate(
	reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.BackupPolicyTemplateName == "" {
		return nil
	}
	template := &dataprotectionv1alpha1.BackupPolicyTemplate{}
	key := types.NamespacedName{Namespace: backupPolicy.Namespace, Name: backupPolicy.Spec.BackupPolicyTemplateName}
	if err := r.Client.Get(reqCtx.Ctx, key, template); err != nil {
		r.Recorder.Eventf(backupPolicy, corev1.EventTypeWarning, "BackupPolicyTemplateFailed",
			"Failed to get backupPolicyTemplateName: %s, reason: %s", key.Name, err.Error())
		return err
	}

	if backupPolicy.Spec.BackupToolName == "" {
		backupPolicy.Spec.BackupToolName = template.Spec.BackupToolName
	}
	if backupPolicy.Spec.Target.Secret.UserKeyword == "" {
		backupPolicy.Spec.BackupToolName = template.Spec.CredentialKeyword.UserKeyword
	}
	if backupPolicy.Spec.Target.Secret.PasswordKeyword == "" {
		backupPolicy.Spec.BackupToolName = template.Spec.CredentialKeyword.PasswordKeyword
	}
	if backupPolicy.Spec.TTL == nil {
		backupPolicy.Spec.TTL = template.Spec.TTL
	}
	if backupPolicy.Spec.Schedule == "" {
		backupPolicy.Spec.Schedule = template.Spec.Schedule
	}
	if backupPolicy.Spec.Hooks == nil {
		backupPolicy.Spec.Hooks = template.Spec.Hooks
	}
	if backupPolicy.Spec.OnFailAttempted == 0 {
		backupPolicy.Spec.OnFailAttempted = template.Spec.OnFailAttempted
	}
	return nil
}

func (r *BackupPolicyReconciler) buildCronJob(backupPolicy *dataprotectionv1alpha1.BackupPolicy) (*batchv1.CronJob, error) {
	tplFile := "cronjob.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
	if err != nil {
		return nil, err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	options := backupPolicyOptions{
		Name:       backupPolicy.Name,
		Namespace:  backupPolicy.Namespace,
		Cluster:    backupPolicy.Spec.Target.LabelsSelector.MatchLabels[intctrlutil.AppInstanceLabelKey],
		Schedule:   backupPolicy.Spec.Schedule,
		TTL:        backupPolicy.Spec.TTL,
		BackupType: backupPolicy.Spec.BackupType,
	}
	backupPolicyOptionsByte, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("options", backupPolicyOptionsByte); err != nil {
		return nil, err
	}

	cronjobByte, err := cueValue.Lookup("cronjob")
	if err != nil {
		return nil, err
	}

	cronjob := batchv1.CronJob{}
	if err = json.Unmarshal(cronjobByte, &cronjob); err != nil {
		return nil, err
	}

	controllerutil.AddFinalizer(&cronjob, dataProtectionFinalizerName)

	scheme, _ := dataprotectionv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(backupPolicy, &cronjob, scheme); err != nil {
		return nil, err
	}

	// set labels
	for k, v := range backupPolicy.Labels {
		if cronjob.Labels == nil {
			cronjob.SetLabels(map[string]string{})
		}
		cronjob.Labels[k] = v
	}
	return &cronjob, nil
}

func (r *BackupPolicyReconciler) removeExpiredBackups(reqCtx intctrlutil.RequestCtx) error {
	backups := dataprotectionv1alpha1.BackupList{}
	if err := r.Client.List(reqCtx.Ctx, &backups,
		client.InNamespace(reqCtx.Req.Namespace)); err != nil {
		return err
	}
	now := metav1.Now()
	for _, item := range backups.Items {
		// ignore retained backup.
		if item.GetLabels()[intctrlutil.BackupProtectionLabelKey] == intctrlutil.BackupRetain {
			return nil
		}
		if item.Status.Expiration != nil && item.Status.Expiration.Before(&now) {
			if err := DeleteObjectBackground(r.Client, reqCtx.Ctx, &item); err != nil {
				// failed delete backups, return error info.
				return err
			}
		}
	}
	return nil
}

func buildBackupLabelsForRemove(backupPolicy *dataprotectionv1alpha1.BackupPolicy) map[string]string {
	return map[string]string{
		intctrlutil.AppInstanceLabelKey:  backupPolicy.Labels[intctrlutil.AppInstanceLabelKey],
		dataProtectionLabelAutoBackupKey: "true",
	}
}

func (r *BackupPolicyReconciler) removeOldestBackups(reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.BackupsHistoryLimit == 0 {
		return nil
	}

	backups := dataprotectionv1alpha1.BackupList{}
	if err := r.Client.List(reqCtx.Ctx, &backups,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(buildBackupLabelsForRemove(backupPolicy))); err != nil {
		return err
	}
	numToDelete := len(backups.Items) - int(backupPolicy.Spec.BackupsHistoryLimit)
	if numToDelete <= 0 {
		return nil
	}
	backupItems := backups.Items
	sort.Sort(byBackupStartTime(backupItems))
	for i := 0; i < numToDelete; i++ {
		if err := DeleteObjectBackground(r.Client, reqCtx.Ctx, &backupItems[i]); err != nil {
			// failed delete backups, return error info.
			return err
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.BackupPolicy{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurDataProtectionReconKey),
		}).
		Complete(r)
}

func (r *BackupPolicyReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	// delete cronjob resource
	cronjob := &batchv1.CronJob{}

	key := types.NamespacedName{
		Namespace: backupPolicy.Namespace,
		Name:      backupPolicy.Name,
	}
	if err := r.Client.Get(reqCtx.Ctx, key, cronjob); err != nil {
		return client.IgnoreNotFound(err)
	}
	if controllerutil.ContainsFinalizer(cronjob, dataProtectionFinalizerName) {
		patch := client.MergeFrom(cronjob.DeepCopy())
		controllerutil.RemoveFinalizer(cronjob, dataProtectionFinalizerName)
		if err := r.Patch(reqCtx.Ctx, cronjob, patch); err != nil {
			return err
		}
	}
	if err := DeleteObjectBackground(r.Client, reqCtx.Ctx, cronjob); err != nil {
		// failed delete k8s job, return error info.
		return err
	}

	return nil
}

// patchCronJob patch cronjob spec if backup policy patched
func (r *BackupPolicyReconciler) patchCronJob(
	reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {

	cronJob := &batchv1.CronJob{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	patch := client.MergeFrom(cronJob.DeepCopy())
	cronJob.Spec.Schedule = backupPolicy.Spec.Schedule
	cronJob.Spec.JobTemplate.Spec.BackoffLimit = &backupPolicy.Spec.OnFailAttempted
	return r.Client.Patch(reqCtx.Ctx, cronJob, patch)
}
