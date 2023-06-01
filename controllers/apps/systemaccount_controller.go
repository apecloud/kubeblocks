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

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	SecretMapStore *secretMapStore
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
	systemAccountsDebugMode string = "ENABLE_DEBUG_SYSACCOUNTS"
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
			reqCtx.Log.V(1).Error(err, "failed to get secrets")
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
				if err := r.createByStmt(reqCtx, cluster, compDef, compKey, engine, account, svcEP, headlessEP, strategy); err != nil {
					return err
				}
			case appsv1alpha1.ReferToExisting:
				if err := r.createByReferingToExisting(reqCtx, cluster, compKey, account); err != nil {
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
	r.SecretMapStore = newSecretMapStore()
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Cluster{}, r.clusterDeletionHander()).
		Owns(&corev1.Secret{}).
		Watches(&source.Kind{Type: &batchv1.Job{}}, r.jobCompletionHander()).
		Complete(r)
}

func (r *SystemAccountReconciler) createByStmt(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compKey componentUniqueKey,
	engine *customizedEngine,
	account appsv1alpha1.SystemAccountConfig,
	svcEP *corev1.Endpoints, headlessEP *corev1.Endpoints, strategy updateStrategy) error {
	// render statements
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	policy := account.ProvisionPolicy

	stmts, secret := getCreationStmtForAccount(compKey, compDef.SystemAccounts.PasswordConfig, account, strategy)

	uprefErr := controllerutil.SetControllerReference(cluster, secret, scheme)
	if uprefErr != nil {
		return uprefErr
	}

	for _, ep := range retrieveEndpoints(policy.Scope, svcEP, headlessEP) {
		// render a job object
		job := renderJob(engine, compKey, stmts, ep)
		// before creating job, we adjust job's attributes, such as labels, tolerations w.r.t cluster info.
		if err := calibrateJobMetaAndSpec(job, cluster, compKey, account.Name); err != nil {
			return err
		}
		// update owner reference
		if err := controllerutil.SetControllerReference(cluster, job, scheme); err != nil {
			return err
		}
		// create job
		if err := r.Client.Create(reqCtx.Ctx, job); err != nil {
			return err
		}
	}
	// push secret to global SecretMapStore, and secret will not be created until job succeeds.
	key := concatSecretName(compKey, (string)(account.Name))
	return r.SecretMapStore.addSecret(key, secret)
}

func (r *SystemAccountReconciler) createByReferingToExisting(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster, key componentUniqueKey, account appsv1alpha1.SystemAccountConfig) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()

	// get secret
	secret := &corev1.Secret{}
	secretRef := account.ProvisionPolicy.SecretRef
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: secretRef.Namespace, Name: secretRef.Name}, secret); err != nil {
		reqCtx.Log.Error(err, "Failed to find secret", "secret", secretRef.Name)
		return err
	}
	// and make a copy of it
	newSecret := renderSecretByCopy(key, (string)(account.Name), secret)
	if uprefErr := controllerutil.SetControllerReference(cluster, newSecret, scheme); uprefErr != nil {
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

	svcerr := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: serviceName}, svcEP)
	if svcerr != nil {
		return false, nil, nil, svcerr
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

	for _, accName := range getAllSysAccounts() {
		key := concatSecretName(key, string(accName))
		_, exists, _ := r.SecretMapStore.getSecret(key)
		if exists {
			detectedFacts |= accName.GetAccountID()
		}
	}
	reqCtx.Log.V(1).Info("Detected account facts with those from cache", "facts", detectedFacts)
	return detectedFacts, nil
}

func (r *SystemAccountReconciler) getEngineFacts(reqCtx intctrlutil.RequestCtx, key componentUniqueKey) (appsv1alpha1.KBAccountType, error) {
	// get pods for this cluster-component, by lable
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
		updateFacts((appsv1alpha1.AccountName(acc)), &accountsID)
	}
	return accountsID, nil
}

func (r *SystemAccountReconciler) jobCompletionHander() *handler.Funcs {
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

	return &handler.Funcs{
		DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
			if e.Object == nil {
				return
			}

			job, ok := e.Object.(*batchv1.Job)
			if !ok {
				return
			}

			ml := job.ObjectMeta.Labels
			accountName := ml[constant.ClusterAccountLabelKey]
			clusterName := ml[constant.AppInstanceLabelKey]
			componentName := ml[constant.KBAppComponentLabelKey]
			if len(accountName) == 0 || len(clusterName) == 0 || len(componentName) == 0 {
				return
			}

			// job failed, reconcile
			if !containsJobCondition(*job, job.Status.Conditions, batchv1.JobComplete, corev1.ConditionTrue) {
				return
			}

			// job for cluster-component-account succeeded
			// create secret for this account
			compKey := componentUniqueKey{
				namespace:     job.Namespace,
				clusterName:   clusterName,
				componentName: componentName,
			}

			key := concatSecretName(compKey, accountName)
			entry, ok, err := r.SecretMapStore.getSecret(key)
			if err != nil || !ok {
				return
			}

			logger.V(1).Info("job succeeded", "job", job.Name, "account", accountName, "cluster", clusterName, "secret", key)

			if err = r.Client.Create(context.TODO(), entry.value); err != nil {
				logger.Error(err, "failed to create secret, will try later", "secret key", key)
				return
			}

			clusterKey := types.NamespacedName{Namespace: job.Namespace, Name: clusterName}
			cluster := &appsv1alpha1.Cluster{}
			if err := r.Client.Get(context.TODO(), clusterKey, cluster); err != nil {
				logger.Error(err, "failed to get cluster", "cluster key", clusterKey)
			} else {
				r.Recorder.Eventf(cluster, corev1.EventTypeNormal, SysAcctCreate,
					"Created Accounts for cluster: %s, component: %s, accounts: %s", cluster.Name, componentName, accountName)
				// delete secret from cache store
				if err = r.SecretMapStore.deleteSecret(key); err != nil {
					logger.Error(err, "failed to delete secret by key", "secret key", key)
				}
			}
		},
	}
}

// Delete removes cached entries from SystemAccountReconciler.SecretMapStore
func (r *SystemAccountReconciler) clusterDeletionHander() builder.Predicates {
	logger := systemAccountLog.WithName("clusterDeletionHandler")
	predicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object == nil {
				return false
			}
			cluster, ok := e.Object.(*appsv1alpha1.Cluster)
			if !ok {
				return false
			}

			// for each component from the cluster, delete cached secrets
			for _, comp := range cluster.Spec.ComponentSpecs {
				compKey := componentUniqueKey{
					namespace:     cluster.Namespace,
					clusterName:   cluster.Name,
					componentName: comp.Name,
				}
				for _, accName := range getAllSysAccounts() {
					key := concatSecretName(compKey, string(accName))
					// delete left-over secrets, and ignore errors if it has been removed.
					_, exists, err := r.SecretMapStore.getSecret(key)
					if err != nil {
						logger.Error(err, "failed to get secrets", "secret key", key)
						continue
					}
					if !exists {
						continue
					}
					err = r.SecretMapStore.deleteSecret(key)
					if err != nil {
						logger.Error(err, "failed to delete secrets", "secret key", key)
					}
				}
			}
			logger.V(1).Info("cluster deleted", "cluster", cluster.Name, "namespace", cluster.Namespace, "secretMapStore", r.SecretMapStore.ListKeys())
			return false
		},
	}
	return builder.WithPredicates(predicate)
}

// existsOperations checks if the cluster is doing operations
func existsOperations(cluster *appsv1alpha1.Cluster) bool {
	opsRequestMap, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	_, isRestoring := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	return len(opsRequestMap) > 0 || isRestoring
}
