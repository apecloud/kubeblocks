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

package apps

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
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
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/sqlchannel"
)

// SystemAccountReconciler reconciles a SystemAccount object.
type SystemAccountReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// componentUniqueKey is used internally to uniquely identify a component, by namespace-clusterName-componentName.
type componentUniqueKey struct {
	namespace     string
	clusterName   string
	componentName string
	characterType string
}

// updateStrategy is used to specify the update strategy for a component.
type updateStrategy int8

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
	systemAccountsDebugMode       string = "ENABLE_DEBUG_SYSACCOUNTS"
	systemAccountPasswdAnnotation string = "passwd"
	systemAccountjobPrefix               = "sysacc"
)

var (
	// systemAccountLog is a logger during runtime
	systemAccountLog logr.Logger
)

func init() {
	viper.SetDefault(systemAccountsDebugMode, false)
	systemAccountLog = log.Log.WithName("systemAccountRuntime")
}

// SystemAccountController does not have a custom resource, but wathes the create/delete/update of resource like cluster,
// clusterdefinition, backuppolicy, jobs, secrets
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;watch;
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
		Log:      log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
		Recorder: r.Recorder,
	}
	reqCtx.Log.V(1).Info("reconcile", "cluster", req.NamespacedName)

	cluster := &appsv1alpha1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	// cluster is under deletion, do nothing
	if !cluster.GetDeletionTimestamp().IsZero() {
		reqCtx.Log.V(1).Info("Cluster is under deletion.", "cluster", req.NamespacedName)
		return intctrlutil.Reconciled()
	}

	// wait till the cluster is running
	if cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
		reqCtx.Log.V(1).Info("Cluster is not ready yet", "cluster", req.NamespacedName)
		return intctrlutil.Reconciled()
	}

	clusterdefinition := &appsv1alpha1.ClusterDefinition{}
	clusterDefNS := types.NamespacedName{Name: cluster.Spec.ClusterDefRef}
	if err := r.Client.Get(reqCtx.Ctx, clusterDefNS, clusterdefinition); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cluster, r.Recorder, err, reqCtx.Log)
	}

	clusterVersion := &appsv1alpha1.ClusterVersion{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, clusterVersion); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cluster, r.Recorder, err, reqCtx.Log)
	}

	componentVersions := clusterVersion.Spec.GetDefNameMappingComponents()

	// process accounts for each component
	processAccountsForComponent := func(compDef *appsv1alpha1.ClusterComponentDefinition, compDecl *appsv1alpha1.ClusterComponentSpec,
		svcEP *corev1.Endpoints, headlessEP *corev1.Endpoints) error {
		var (
			err                 error
			toCreate            appsv1alpha1.KBAccountType
			detectedK8SFacts    appsv1alpha1.KBAccountType
			detectedEngineFacts appsv1alpha1.KBAccountType
			engine              *customizedEngine
			compKey             = componentUniqueKey{
				namespace:     cluster.Namespace,
				clusterName:   cluster.Name,
				componentName: compDecl.Name,
				characterType: compDef.CharacterType,
			}
		)

		// expectations: collect accounts from default setting, cluster and cluster definition.
		toCreate = getDefaultAccounts()
		reqCtx.Log.V(1).Info("accounts to create", "cluster", req.NamespacedName, "accounts", toCreate)

		// facts: accounts have been created, in form of k8s secrets.
		if detectedK8SFacts, err = r.getAccountFacts(reqCtx, compKey); err != nil {
			reqCtx.Log.Error(err, "failed to get secrets")
			return err
		}
		reqCtx.Log.V(1).Info("detected k8s facts", "cluster", req.NamespacedName, "accounts", detectedK8SFacts)

		// toCreate = account to create - account exists
		// (toCreate \intersect detectedEngineFacts) means the set of account exists in engine but not in k8s, and should be updated or altered, not re-created.
		toCreate &= toCreate ^ detectedK8SFacts
		if toCreate == 0 {
			return nil
		}

		// facts: accounts have been created in engine.
		if detectedEngineFacts, err = r.getEngineFacts(reqCtx, compKey); err != nil {
			reqCtx.Log.Error(err, "failed to get accounts", "cluster", cluster.Name, "component", compDecl.Name)
			// we don't return error here, because we can still create accounts in k8s and will give it a try.
		}
		reqCtx.Log.V(1).Info("detected database facts", "cluster", req.NamespacedName, "accounts", detectedEngineFacts)

		// replace KubeBlocks ENVs.
		replaceEnvsValues(cluster.Name, compDef.SystemAccounts)

		for _, account := range compDef.SystemAccounts.Accounts {
			accountID := account.Name.GetAccountID()
			if toCreate&accountID == 0 {
				continue
			}

			strategy := reCreate
			if detectedEngineFacts&accountID != 0 {
				strategy = inPlaceUpdate
			}

			switch account.ProvisionPolicy.Type {
			case appsv1alpha1.CreateByStmt:
				if engine == nil {
					execConfig := compDef.SystemAccounts.CmdExecutorConfig
					// complete execConfig with settings from component version
					completeExecConfig(execConfig, componentVersions[compDef.Name])
					engine = newCustomizedEngine(execConfig, cluster, compDecl.Name)
				}
				reqCtx.Log.V(1).Info("create account by stmt", "cluster", req.NamespacedName, "account", account.Name, "strategy", strategy)
				if err := r.createByStmt(reqCtx, cluster, compDef, compKey, engine, account, svcEP, headlessEP, strategy); err != nil {
					return err
				}
			case appsv1alpha1.ReferToExisting:
				if err := r.createByReferringToExisting(reqCtx, cluster, compKey, account); err != nil {
					return err
				}
			}
		}
		return nil
	} // end of processAccountForComponent

	reconcileCounter := 0
	existsOps := existsOperations(cluster)
	// for each component in the cluster
	for _, compDecl := range cluster.Spec.ComponentSpecs {
		compName := compDecl.Name
		compType := compDecl.ComponentDefRef
		for _, compDef := range clusterdefinition.Spec.ComponentDefs {
			if compType != compDef.Name || compDef.SystemAccounts == nil {
				continue
			}

			isReady, svcEP, headlessEP, err := r.isComponentReady(reqCtx, cluster.Name, compName)
			if err != nil {
				return intctrlutil.RequeueAfter(requeueDuration, reqCtx.Log, "failed to get service")
			}

			// either service or endpoint is not ready, increase counter and continue to process next component
			if !isReady || existsOps {
				reconcileCounter++
				continue
			}

			if err := processAccountsForComponent(&compDef, &compDecl, svcEP, headlessEP); err != nil {
				reconcileCounter++
				continue
			}
		}
	}

	if reconcileCounter > 0 {
		return intctrlutil.Requeue(reqCtx.Log, "Not all components have been reconciled. Requeue request.")
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SystemAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Cluster{}).
		Owns(&corev1.Secret{}).
		Watches(&source.Kind{Type: &batchv1.Job{}}, r.jobCompletionHandler()).
		Complete(r)
}

func (r *SystemAccountReconciler) createByStmt(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compKey componentUniqueKey,
	engine *customizedEngine,
	account appsv1alpha1.SystemAccountConfig,
	svcEP *corev1.Endpoints, headlessEP *corev1.Endpoints, strategy updateStrategy) error {
	policy := account.ProvisionPolicy

	generateJobName := func() string {
		// render a job object, named after account name
		randSuffix := rand.String(5)
		fullJobName := strings.Join([]string{systemAccountjobPrefix, compKey.clusterName, compKey.componentName, string(account.Name), randSuffix}, "-")
		if len(fullJobName) > 63 {
			return systemAccountjobPrefix + "-" + string(account.Name) + "-" + randSuffix
		} else {
			return fullJobName
		}
	}

	stmts, passwd := getCreationStmtForAccount(compKey, compDef.SystemAccounts.PasswordConfig, account, strategy)

	for _, ep := range retrieveEndpoints(policy.Scope, svcEP, headlessEP) {
		job := renderJob(generateJobName(), engine, compKey, stmts, ep)
		controllerutil.AddFinalizer(job, constant.DBClusterFinalizerName)
		if job.Annotations == nil {
			job.Annotations = map[string]string{}
		}
		job.Annotations[systemAccountPasswdAnnotation] = passwd

		// before creating job, we adjust job's attributes, such as labels, tolerations w.r.t cluster info.
		if err := calibrateJobMetaAndSpec(job, cluster, compKey, account.Name); err != nil {
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
		reqCtx.Log.V(1).Info("created job", "job", job.Name, "passwd", passwd)
	}
	return nil
}

func (r *SystemAccountReconciler) createByReferingToExisting(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster, key componentUniqueKey, account appsv1alpha1.SystemAccountConfig) error {
	// get secret
	secret := &corev1.Secret{}
	secretRef := account.ProvisionPolicy.SecretRef
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: secretRef.Namespace, Name: secretRef.Name}, secret); err != nil {
		reqCtx.Log.Error(err, "Failed to find secret", "secret", secretRef.Name)
		return err
	}
	// and make a copy of it
	newSecret := renderSecretByCopy(key, (string)(account.Name), secret)
	if uprefErr := controllerutil.SetControllerReference(cluster, newSecret, r.Scheme); uprefErr != nil {
		return uprefErr
	}

	if err := r.Client.Create(reqCtx.Ctx, newSecret); err != nil {
		reqCtx.Log.Error(err, "Failed to find secret", "secret", newSecret.Name)
		return err
	}
	return nil
}

func (r *SystemAccountReconciler) isComponentReady(reqCtx intctrlutil.RequestCtx, clusterName string, compName string) (bool, *corev1.Endpoints, *corev1.Endpoints, error) {
	svcEP := &corev1.Endpoints{}
	serviceName := clusterName + "-" + compName

	headlessEP := &corev1.Endpoints{}
	headlessSvcName := serviceName + "-headless"

	svcErr := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: serviceName}, svcEP)
	if svcErr != nil {
		return false, nil, nil, svcErr
	}

	headlessSvcErr := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: headlessSvcName}, headlessEP)
	if headlessSvcErr != nil {
		return false, nil, nil, headlessSvcErr
	}
	// Neither service nor endpoints is ready.
	if len(svcEP.Subsets) == 0 || len(headlessEP.Subsets) == 0 {
		return false, nil, nil, nil
	}

	// make sure address exists
	if len(svcEP.Subsets[0].Addresses) == 0 || len(headlessEP.Subsets[0].Addresses) == 0 {
		return false, nil, nil, nil
	}

	return true, svcEP, headlessEP, nil
}

// getAccountFacts parses secrets for given cluster as facts, i.e., accounts created
// TODO: @shanshan, should verify accounts on database cluster as well.
func (r *SystemAccountReconciler) getAccountFacts(reqCtx intctrlutil.RequestCtx, key componentUniqueKey) (appsv1alpha1.KBAccountType, error) {
	// get account facts, i.e., secrets created
	ml := getLabelsForSecretsAndJobs(key)

	secrets := &corev1.SecretList{}
	if err := r.Client.List(reqCtx.Ctx, secrets, client.InNamespace(key.namespace), ml); err != nil {
		return appsv1alpha1.KBAccountInvalid, err
	}

	// get all running jobs
	jobs := &batchv1.JobList{}
	if err := r.Client.List(reqCtx.Ctx, jobs, client.InNamespace(key.namespace), ml); err != nil {
		return appsv1alpha1.KBAccountInvalid, err
	}

	detectedFacts := getAccountFacts(secrets, jobs)
	reqCtx.Log.V(1).Info("Detected account facts", "facts", detectedFacts)
	return detectedFacts, nil
}

func (r *SystemAccountReconciler) getEngineFacts(reqCtx intctrlutil.RequestCtx, key componentUniqueKey) (appsv1alpha1.KBAccountType, error) {
	// get pods for this cluster-component, by label
	ml := getLabelsForSecretsAndJobs(key)
	pods := &corev1.PodList{}
	if err := r.Client.List(reqCtx.Ctx, pods, client.InNamespace(key.namespace), ml); err != nil {
		return appsv1alpha1.KBAccountInvalid, err
	}
	if len(pods.Items) == 0 {
		return appsv1alpha1.KBAccountInvalid, fmt.Errorf("no pods available for cluster: %s, component %s", key.clusterName, key.componentName)
	}
	// find the first running pod
	var target *corev1.Pod
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			target = &pod
		}
	}
	if target == nil {
		return appsv1alpha1.KBAccountInvalid, fmt.Errorf("no pod is running for cluster: %s, component %s", key.clusterName, key.componentName)
	}

	sqlChanClient, err := sqlchannel.NewClientWithPod(target, key.characterType)
	if err != nil {
		return appsv1alpha1.KBAccountInvalid, err
	}
	accounts, err := sqlChanClient.GetSystemAccounts()
	if err != nil {
		return appsv1alpha1.KBAccountInvalid, err
	}
	accountsID := appsv1alpha1.KBAccountInvalid
	for _, acc := range accounts {
		updateFacts(appsv1alpha1.AccountName(acc), &accountsID)
	}
	return accountsID, nil
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
	// 2. has completed (either successed or failed)
	// 3. is under deletion (either by user or by TTL, where deletionTimestamp is set)
	return &handler.Funcs{
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
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

			if job, ok = e.ObjectNew.(*batchv1.Job); !ok {
				return
			}

			if job.DeletionTimestamp != nil || job.Annotations == nil || job.Labels == nil {
				return
			}

			accountName := job.ObjectMeta.Labels[constant.ClusterAccountLabelKey]
			clusterName := job.ObjectMeta.Labels[constant.AppInstanceLabelKey]
			componentName := job.ObjectMeta.Labels[constant.KBAppComponentLabelKey]

			if len(accountName) == 0 || len(clusterName) == 0 || len(componentName) == 0 {
				return
			}

			if containsJobCondition(*job, job.Status.Conditions, batchv1.JobFailed, corev1.ConditionTrue) {
				logger.V(1).Info("job failed", "job", job.Name)
				jobTerminated = true
				return
			}

			if !containsJobCondition(*job, job.Status.Conditions, batchv1.JobComplete, corev1.ConditionTrue) {
				return
			}

			logger.V(1).Info("job succeeded", "job", job.Name)
			jobTerminated = true
			clusterKey := types.NamespacedName{Namespace: job.Namespace, Name: clusterName}
			cluster := &appsv1alpha1.Cluster{}
			if err := r.Client.Get(context.TODO(), clusterKey, cluster); err != nil {
				logger.Error(err, "failed to get cluster", "cluster key", clusterKey)
				return
			}

			compKey := componentUniqueKey{
				namespace:     job.Namespace,
				clusterName:   clusterName,
				componentName: componentName,
			}
			// get password from job
			passwd := job.Annotations[systemAccountPasswdAnnotation]
			secret := renderSecretWithPwd(compKey, accountName, passwd)
			if err := controllerutil.SetControllerReference(cluster, secret, r.Scheme); err != nil {
				logger.Error(err, "failed to set ownere reference for secret", secret.Name)
				return
			}

			if err := r.Client.Create(context.TODO(), secret); err != nil {
				logger.Error(err, "failed to create secret", secret.Name)
				return
			}
			r.Recorder.Eventf(cluster, corev1.EventTypeNormal, SysAcctCreate,
				"Created Accounts for cluster: %s, component: %s, accounts: %s", cluster.Name, componentName, accountName)
		},
	}
}

// existsOperations checks if the cluster is doing operations
func existsOperations(cluster *appsv1alpha1.Cluster) bool {
	opsRequestMap, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	_, isRestoring := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	return len(opsRequestMap) > 0 || isRestoring
}
