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

	// update default value from viper config if necessary
	patch := client.MergeFrom(backupPolicy.DeepCopy())
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
	backupPolicy.SetLabels(backupPolicy.Spec.Target.LabelsSelector.MatchLabels)
	if err = r.Client.Patch(reqCtx.Ctx, backupPolicy, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// patch cronjob if backup policy spec patched
	if err := r.patchCronJob(reqCtx, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// if backup policy is available, try to remove expired or oldest backups
	if backupPolicy.Status.Phase == dataprotectionv1alpha1.ConfigAvailable {
		if err := r.RemoveExpiredBackups(reqCtx); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if err := r.RemoveOldestBackups(reqCtx, backupPolicy); err != nil {
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
	if err != nil {
		// ignore already exists.
		if !errors.IsAlreadyExists(err) {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}

	// update status phase
	backupPolicy.Status.Phase = dataprotectionv1alpha1.ConfigAvailable
	if err := r.Client.Status().Update(reqCtx.Ctx, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

type BackupPolicyOptions struct {
	Name       string           `json:"name"`
	Namespace  string           `json:"namespace"`
	Cluster    string           `json:"cluster"`
	Schedule   string           `json:"schedule"`
	BackupType string           `json:"backupType"`
	TTL        *metav1.Duration `json:"ttl"`
}

func (r *BackupPolicyReconciler) buildCronJob(backupPolicy *dataprotectionv1alpha1.BackupPolicy) (*batchv1.CronJob, error) {
	tplFile := "cronjob.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
	if err != nil {
		return nil, err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	options := BackupPolicyOptions{
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

	cronjob.SetLabels(backupPolicy.Labels)

	return &cronjob, nil
}

func (r *BackupPolicyReconciler) RemoveExpiredBackups(reqCtx intctrlutil.RequestCtx) error {
	backups := dataprotectionv1alpha1.BackupList{}
	if err := r.Client.List(reqCtx.Ctx, &backups,
		client.InNamespace(reqCtx.Req.Namespace)); err != nil {
		return err
	}
	now := metav1.Now()
	for _, item := range backups.Items {
		if item.Status.Expiration.Before(&now) {
			if err := DeleteObjectBackground(r.Client, reqCtx.Ctx, &item); err != nil {
				// failed delete backups, return error info.
				return err
			}
		}
	}
	return nil
}

func buildBackupSetLabels(backupPolicy *dataprotectionv1alpha1.BackupPolicy) map[string]string {
	return map[string]string{
		intctrlutil.AppInstanceLabelKey:  backupPolicy.Labels[intctrlutil.AppInstanceLabelKey],
		dataProtectionLabelAutoBackupKey: "true",
	}
}

func (r *BackupPolicyReconciler) RemoveOldestBackups(reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.BackupsHistoryLimit == 0 {
		return nil
	}

	backups := dataprotectionv1alpha1.BackupList{}
	if err := r.Client.List(reqCtx.Ctx, &backups,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(buildBackupSetLabels(backupPolicy))); err != nil {
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
		if errors.IsNotFound(err) {
			return nil
		}
		return err
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
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	patch := client.MergeFrom(cronJob.DeepCopy())
	cronJob.Spec.Schedule = backupPolicy.Spec.Schedule
	cronJob.Spec.JobTemplate.Spec.BackoffLimit = &backupPolicy.Spec.OnFailAttempted
	return r.Client.Patch(reqCtx.Ctx, cronJob, patch)
}
