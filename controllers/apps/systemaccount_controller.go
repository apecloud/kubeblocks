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

package apps

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// SystemAccountReconciler reconciles a SystemAccount object.
type SystemAccountReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// updateStrategy is used to specify the update strategy for a component.
type updateStrategy int8

type accountNameSet map[string]struct{}

type compUniqueKey struct {
	namespace    string
	clusterName  string
	compName     string
	compFullName string
}

const (
	inPlaceUpdate updateStrategy = 1
	reCreate      updateStrategy = 2
)

// SysAccountDeletion and SysAccountCreation are used as event reasons.
const (
	SysAcctDelete      = "SysAcctDelete"
	SysAcctCreate      = "SysAcctCreate"
	SysAcctUnsupported = "SysAcctUnsupported"
)

// Environment names for cmd config connections
const (
	kbAccountStmtEnvName     = "KB_ACCOUNT_STATEMENT"
	kbAccountEndPointEnvName = "KB_ACCOUNT_ENDPOINT"
)

// ENABLE_DEBUG_SYSACCOUNTS is used for debug only.
const (
	systemAccountsDebugMode = "ENABLE_DEBUG_SYSACCOUNTS"
	systemAccountjobPrefix  = "sysacc"
)

var (
	// systemAccountLog is a logger during runtime
	systemAccountLog logr.Logger
)

func init() {
	viper.SetDefault(systemAccountsDebugMode, false)
	systemAccountLog = log.Log.WithName("systemAccountRuntime")
}

// SystemAccountController does not have a custom resource, but watches the create/delete/update of resource like cluster,
// clusterdefinition, backuppolicy, jobs, secrets
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components,verbs=get;list;watch;
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/status,verbs=get
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions,verbs=get;list;watch;
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SystemAccount object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *SystemAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("component", req.NamespacedName),
		Recorder: r.Recorder,
	}
	reqCtx.Log.V(1).Info("reconcile", "component", req.NamespacedName)

	// get component
	comp := &appsv1alpha1.Component{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, comp); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	// get cluster object
	clusterName, err := getClusterName(comp)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	cluster := &appsv1alpha1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: comp.Namespace, Name: clusterName}, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	// component or cluster is under deletion, delete all sysaccount jobs
	if !comp.GetDeletionTimestamp().IsZero() || !cluster.GetDeletionTimestamp().IsZero() {
		reqCtx.Log.V(1).Info("Component is under deletion.", "component", req.NamespacedName)
		// get sysaccount jobs for this cluster and delete them
		jobs := &batchv1.JobList{}
		ml := client.MatchingLabels(constant.GetComponentWellKnownLabels(cluster.Name, getCompNameShort(comp)))
		if err := r.Client.List(reqCtx.Ctx, jobs, ml); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		for _, job := range jobs.Items {
			patch := client.MergeFrom(job.DeepCopy())
			controllerutil.RemoveFinalizer(&job, constant.DBClusterFinalizerName)
			_ = r.Client.Patch(context.Background(), &job, patch)
		}
		return intctrlutil.Reconciled()
	}

	// get componentdefintion
	cmpdName := comp.Spec.CompDef
	if len(cmpdName) == 0 {
		reqCtx.Log.V(1).Info("Component does not have a ComponentDefinition", "component", req.NamespacedName)
		return intctrlutil.Reconciled()
	}

	cmpd := &appsv1alpha1.ComponentDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: cmpdName}, cmpd); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// skip if the component does not support system accounts
	if cmpd.Spec.LifecycleActions.AccountProvision == nil || len(cmpd.Spec.SystemAccounts) == 0 {
		reqCtx.Log.V(1).Info("ComponentDefinition does not support system accounts", "component", req.NamespacedName)
		return intctrlutil.Reconciled()
	}

	if cmpd.Spec.LifecycleActions.AccountProvision.CustomHandler == nil {
		reqCtx.Log.V(1).Info("ComponentDefinition does not have a custom handler for account provision", "component", req.NamespacedName)
		return intctrlutil.Reconciled()
	}
	// wait till the component is running
	if !passPreCondition(reqCtx.Ctx, r.Client, cluster, comp, cmpd) {
		reqCtx.Log.V(1).Info("Component is not ready yet", "component", req.NamespacedName)
		return intctrlutil.Reconciled()
	}

	// request if the cluster is doing operations
	if existsOps := existsOperations(cluster); existsOps {
		return intctrlutil.RequeueAfter(10, reqCtx.Log, "requeue", comp.Name)
	}

	compKey := &compUniqueKey{
		namespace:    comp.Namespace,
		clusterName:  cluster.Name,
		compName:     getCompNameShort(comp),
		compFullName: comp.Name,
	}

	action := cmpd.Spec.LifecycleActions.AccountProvision.CustomHandler

	provisionedAccounts, err := getAccountsProvisioned(reqCtx, r.Client, compKey)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	detectedEngineAccounts := getAccountsInEngine(reqCtx, r.Client, compKey)

	for _, account := range cmpd.Spec.SystemAccounts {
		if provisionedAccounts.Contains(account.Name) {
			continue
		}
		strategy := reCreate
		if detectedEngineAccounts.Contains(account.Name) {
			strategy = inPlaceUpdate
		}

		reqCtx.Log.V(1).Info("create account by stmt", "cluster", req.NamespacedName, "account", account.Name, "strategy", strategy)

		secret, err := getAccountSecretByName(reqCtx.Ctx, r.Client, compKey, account.Name)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		username, passwd := secret.Data[constant.AccountNameForSecret], secret.Data[constant.AccountPasswdForSecret]
		stmts := getCreationStmtForAccount((string)(username), (string)(passwd), account.Statement, strategy)
		pods, err := getTargetPods(reqCtx.Ctx, r.Client, action, cmpd.Spec.Roles, compKey)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		if err := r.createByStmt(reqCtx, cluster, comp, action, account.Name, stmts, pods); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *SystemAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Component{}).
		Owns(&corev1.Secret{}).
		Watches(&batchv1.Job{}, r.jobCompletionHandler()).
		Complete(r)
}

func (r *SystemAccountReconciler) createByStmt(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	action *appsv1alpha1.Action,
	accountName string,
	stmts []string,
	pods *corev1.PodList) error {
	// render a job object, named after account name
	generateJobName := func() string {
		randSuffix := rand.String(5)
		fullJobName := strings.Join([]string{systemAccountjobPrefix, comp.Name, accountName, randSuffix}, "-")
		if len(fullJobName) > 63 {
			return systemAccountjobPrefix + "-" + accountName + "-" + randSuffix
		} else {
			return fullJobName
		}
	}

	compKey := &compUniqueKey{
		namespace:    comp.Namespace,
		clusterName:  cluster.Name,
		compName:     getCompNameShort(comp),
		compFullName: comp.Name,
	}

	for _, pod := range pods.Items {
		job := renderJob(generateJobName(), action, compKey, stmts, &pod)
		controllerutil.AddFinalizer(job, constant.DBClusterFinalizerName)
		// before creating job, we adjust job's attributes, such as labels, tolerations w.r.t cluster info.
		if err := calibrateJobMetaAndSpec(job, cluster, comp, accountName); err != nil {
			return err
		}
		// update owner reference
		if err := controllerutil.SetControllerReference(cluster, job, r.Scheme); err != nil {
			return err
		}
		// create job
		if err := r.Client.Create(reqCtx.Ctx, job); err != nil {
			return err
		}
		reqCtx.Log.V(1).Info("created job", "job", job.Name)
	}

	return nil
}

func (r *SystemAccountReconciler) jobCompletionHandler() *handler.Funcs {
	logger := systemAccountLog.WithName("jobCompletionHandler")

	containsJobCondition := func(job batchv1.Job, jobConditions []batchv1.JobCondition,
		jobCondType batchv1.JobConditionType, jobCondStatus corev1.ConditionStatus) bool {
		for _, jobCond := range job.Status.Conditions {
			if jobCond.Type == jobCondType && jobCond.Status == jobCondStatus {
				return true
			}
		}
		return false
	}

	// check against a job to make sure it
	// 1. works for sysaccount (by checking labels)
	// 2. has completed (either succeeded or failed)
	// 3. is under deletion (either by user or by TTL, where deletionTimestamp is set)
	return &handler.Funcs{
		UpdateFunc: func(ctx context.Context, e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			var (
				jobTerminated = false
				job           *batchv1.Job
				ok            bool
			)

			defer func() {
				// prepare a patch by removing finalizer
				if jobTerminated {
					patch := client.MergeFrom(job.DeepCopy())
					controllerutil.RemoveFinalizer(job, constant.DBClusterFinalizerName)
					_ = r.Client.Patch(context.Background(), job, patch)
				}
			}()

			if e.ObjectNew == nil {
				return
			}

			if job, ok = e.ObjectNew.(*batchv1.Job); !ok || job.Labels == nil {
				return
			}

			accountName := job.Labels[constant.ClusterAccountLabelKey]
			clusterName := job.Labels[constant.AppInstanceLabelKey]
			componentName := job.Labels[constant.KBAppComponentLabelKey]

			compKey := &compUniqueKey{
				namespace:   job.Namespace,
				clusterName: clusterName,
				compName:    componentName,
			}
			// filter out jobs that are not for system account
			if len(accountName) == 0 || len(clusterName) == 0 || len(componentName) == 0 {
				return
			}
			// filter out jobs that have not reached completion (either completed or failed) or have been handled
			if !containsJobCondition(*job, job.Status.Conditions, batchv1.JobFailed, corev1.ConditionTrue) &&
				!containsJobCondition(*job, job.Status.Conditions, batchv1.JobComplete, corev1.ConditionTrue) ||
				!controllerutil.ContainsFinalizer(job, constant.DBClusterFinalizerName) {
				return
			}

			jobTerminated = true

			comp := &appsv1alpha1.Component{}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: job.Namespace, Name: fmt.Sprintf("%s-%s", clusterName, componentName)}, comp); err != nil {
				logger.Error(err, "failed to get component", "key", compKey)
				return
			}

			if containsJobCondition(*job, job.Status.Conditions, batchv1.JobFailed, corev1.ConditionTrue) {
				logger.V(1).Info("job failed", "job", job.Name)
				r.Recorder.Eventf(comp, corev1.EventTypeNormal, SysAcctCreate,
					"Failed to create accounts for cluster: %s, component: %s, accounts: %s", clusterName, componentName, accountName)
				return
			}

			secret, err := getAccountSecretByName(context.TODO(), r.Client, compKey, accountName)
			if err != nil {
				logger.Error(err, "failed to get secret for account", "account", accountName)
				return
			}
			// update secret annotation
			patch := client.MergeFrom(secret.DeepCopy())
			if secret.Annotations == nil {
				secret.Annotations = map[string]string{}
			}
			secret.Annotations[constant.ComponentAccountProvisionKey] = constant.AccountProvisioned
			_ = r.Client.Patch(context.Background(), secret, patch)

			r.Recorder.Eventf(comp, corev1.EventTypeNormal, SysAcctCreate,
				"Created accounts for cluster: %s, component: %s, accounts: %s", clusterName, componentName, accountName)
		},
	}
}
