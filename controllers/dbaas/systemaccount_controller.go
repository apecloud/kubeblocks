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

package dbaas

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// SystemAccountReconciler reconciles a SystemAccount object.
type SystemAccountReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	ExpectionManager *systemAccountExpectationsManager
	SecretMapStore   *secretMapStore
}

// backupPolicyChangePredicate implements default create and delete predicate functions on BackupPolicy creation and deletion.
type backupPolicyChangePredicate struct {
	predicate.Funcs
	ExpectionManager *systemAccountExpectationsManager
	Log              logr.Logger
}

// jobCompleditionPredicate implements a default delete predicate function on job deletion.
type jobCompletitionPredicate struct {
	predicate.Funcs
	reconciler *SystemAccountReconciler
	Log        logr.Logger
}

// clusterDeletionPredicate implements a default delete predication function on cluster deletion.
// It is used to clean cached secrets from SystemAccountReconciler.SecretMapStore
type clusterDeletionPredicate struct {
	predicate.Funcs
	reconciler *SystemAccountReconciler
	clusterLog logr.Logger
}

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

// username and password are keys in created secrets for others to refer to.
const (
	accountNameForSecret   = "username"
	accountPasswdForSecret = "password"
)

// compile-time assert that the local data object satisfies the phases data interface.
var _ predicate.Predicate = &backupPolicyChangePredicate{}

// compile-time assert that the local data object satisfies the phases data interface.
var _ predicate.Predicate = &jobCompletitionPredicate{}

// compile-time assert that the local data object satisfies the phases data interface.
var _ predicate.Predicate = &clusterDeletionPredicate{}

var (
	// systemAccountLog is a logger for use during runtime
	systemAccountLog logr.Logger
)

func init() {
	systemAccountLog = log.Log.WithName("systemAccountRuntime")
}

// SystemAccountController does not have a custom resource, but wathes the create/delete/update of resource like cluster,
// clusterdefinition, backuppolicy, jobs, secrets
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusters,verbs=get;list;watch;
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusters/status,verbs=get
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;watch;
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

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
	reqCtx.Log.Info("get cluster", "cluster", req.NamespacedName)

	cluster := &dbaasv1alpha1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	// cluster is under deletion, do nothing
	if !cluster.GetDeletionTimestamp().IsZero() {
		reqCtx.Log.Info("Cluster is under deletion.", "cluster", req.NamespacedName)
		return intctrlutil.Reconciled()
	}

	clusterdefinition := &dbaasv1alpha1.ClusterDefinition{}
	clusterDefNS := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.ClusterDefRef}
	if err := r.Client.Get(reqCtx.Ctx, clusterDefNS, clusterdefinition); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cluster, r.Recorder, err, reqCtx.Log)
	}

	// wait till the cluster is running
	if cluster.Status.Phase != dbaasv1alpha1.RunningPhase && cluster.Status.Phase != dbaasv1alpha1.CreatingPhase {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "Cluster is not ready yet", "cluster", cluster.Name)
	}

	// process accounts per component
	processAccountsForComponent := func(compDef *dbaasv1alpha1.ClusterDefinitionComponent, compDecl *dbaasv1alpha1.ClusterComponent,
		svcEP *corev1.Endpoints, headlessEP *corev1.Endpoints) error {
		// expectations: collect accounts from cluster and cluster definition to be created
		var toCreate dbaasv1alpha1.KBAccountType
		toCreate |= dbaasv1alpha1.KBAccountAdmin
		if compDecl.Monitor {
			toCreate |= dbaasv1alpha1.KBAccountMonitor
		}
		if compDef.Probes != nil {
			toCreate |= dbaasv1alpha1.KBAccountProbe
		}

		// cmdExecutorConfig has a higher priority than built-in engines.
		engineType := getEngineType(clusterdefinition.Spec.Type, *compDef)
		if len(engineType) > 0 {
			// expectations: collect extra accounts, trigger by other recources, to be created
			// TODO: @shanshan. This part should be updated when BackupPolicy API is updated in the future.
			charKey := expectationKey(cluster.Namespace, cluster.Name, engineType)
			charExpect, charExists, err := r.ExpectionManager.getExpectation(charKey)
			if err != nil {
				return err
			}
			if charExists {
				charToCreate := charExpect.getExpectation()
				toCreate |= charToCreate
			}
		}
		// facts: accounts have been created.
		detectedFacts, err := r.getAccountFacts(reqCtx, cluster.Namespace, cluster.Name, clusterdefinition.Spec.Type, clusterdefinition.Name, compDecl.Name)
		if err != nil {
			reqCtx.Log.Error(err, "failed to get secrets")
			return err
		}
		// toCreate = account to create - account exists
		toCreate &= (toCreate ^ detectedFacts)
		if toCreate == 0 {
			return nil
		}

		var engine *customizedEngine
		replaceEnvsValues(cluster.Name, compDef.SystemAccounts)

		for _, account := range compDef.SystemAccounts.Accounts {
			accountID := account.Name.GetAccountID()
			if toCreate&accountID == 0 {
				continue
			}

			switch account.ProvisionPolicy.Type {
			case dbaasv1alpha1.CreateByStmt:
				if engine == nil {
					execConfig := compDef.SystemAccounts.CmdExecutorConfig
					engine = newCustomizedEngine(execConfig, cluster, compDecl.Name)
				}
				if err := r.createByStmt(reqCtx, cluster, clusterdefinition.Spec.Type, clusterdefinition.Name, compDef, compDecl.Name, engine, account, svcEP, headlessEP); err != nil {
					return err
				}
			case dbaasv1alpha1.ReferToExisting:
				if err := r.createByReferingToExisting(reqCtx, cluster, clusterdefinition.Spec.Type, clusterdefinition.Name, compDecl.Name, account); err != nil {
					return err
				}
			}
		}
		return nil
	} // end of processAccountForComponent

	reconcileCounter := 0
	// for each component in the cluster
	for _, compDecl := range cluster.Spec.Components {
		compName := compDecl.Name
		compType := compDecl.Type
		for _, compDef := range clusterdefinition.Spec.Components {
			if compType != compDef.TypeName || compDef.SystemAccounts == nil {
				continue
			}

			isReady, svcEP, headlessEP, err := r.isComponentReady(reqCtx, cluster.Name, compName)
			if err != nil {
				return intctrlutil.RequeueWithErrorAndRecordEvent(cluster, r.Recorder, err, reqCtx.Log)
			}

			// either service or endpoint is not ready, increase counter and continue to process next component
			if !isReady {
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
	r.ExpectionManager = newExpectationsManager()
	r.SecretMapStore = newSecretMapStore()

	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.Cluster{}, builder.WithPredicates(&clusterDeletionPredicate{reconciler: r, clusterLog: systemAccountLog.WithName("clusterDeletionPredicate")})).
		Owns(&corev1.Secret{}).
		Watches(&source.Kind{Type: &dataprotectionv1alpha1.BackupPolicy{}},
			handler.EnqueueRequestsFromMapFunc(r.findClusterForBackupPolicy),
			builder.WithPredicates(&backupPolicyChangePredicate{ExpectionManager: r.ExpectionManager, Log: log.FromContext(context.TODO())})).
		Watches(&source.Kind{Type: &batchv1.Job{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(&jobCompletitionPredicate{reconciler: r, Log: log.FromContext(context.TODO())})).
		Complete(r)
}

func (r *SystemAccountReconciler) createByStmt(reqCtx intctrlutil.RequestCtx,
	cluster *dbaasv1alpha1.Cluster,
	clusterDefType, clusterDefName string,
	compDef *dbaasv1alpha1.ClusterDefinitionComponent,
	compName string,
	engine *customizedEngine,
	account dbaasv1alpha1.SystemAccountConfig,
	svcEP *corev1.Endpoints, headlessEP *corev1.Endpoints) error {
	// render statements
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	policy := account.ProvisionPolicy

	stmts, secret := getCreationStmtForAccount(reqCtx.Req.Namespace, reqCtx.Req.Name, clusterDefType, clusterDefName,
		compName, compDef.SystemAccounts.PasswordConfig, account)

	uprefErr := controllerutil.SetOwnerReference(cluster, secret, scheme)
	if uprefErr != nil {
		return uprefErr
	}

	for _, ep := range retrieveEndpoints(policy.Scope, svcEP, headlessEP) {
		// render a job object
		job := renderJob(engine, reqCtx.Req.Namespace, reqCtx.Req.Name, clusterDefType, clusterDefName, compName, (string)(account.Name), stmts, ep)
		if err := controllerutil.SetOwnerReference(cluster, job, scheme); err != nil {
			return err
		}
		if err := r.Client.Create(reqCtx.Ctx, job); err != nil {
			return err
		}
	}
	// push secret to global SecretMapStore, and secret will be create not until job succeeds.
	key := concatSecretName(reqCtx.Req.Namespace, reqCtx.Req.Name, compName, (string)(account.Name))
	return r.SecretMapStore.addSecret(key, secret)
}

func (r *SystemAccountReconciler) createByReferingToExisting(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster, clusterDefType, clusterDefName, compName string, account dbaasv1alpha1.SystemAccountConfig) error {
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()

	// get secret
	secret := &corev1.Secret{}
	secretRef := account.ProvisionPolicy.SecretRef
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: secretRef.Namespace, Name: secretRef.Name}, secret); err != nil {
		reqCtx.Log.Error(err, "Failed to find secret", "secret", secretRef.Name)
		return err
	}
	// and make a copy of it
	newSecret := renderSecretByCopy(reqCtx.Req.Namespace, reqCtx.Req.Name, clusterDefType, clusterDefName, compName, (string)(account.Name), secret)
	uprefErr := controllerutil.SetOwnerReference(cluster, newSecret, scheme)
	if uprefErr != nil {
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
	// either service or endpoints is not ready.
	if len(svcEP.Subsets) == 0 || len(headlessEP.Subsets) == 0 {
		return false, nil, nil, nil
	}

	// make sure address exists
	if len(svcEP.Subsets[0].Addresses) == 0 || len(headlessEP.Subsets[0].Addresses) == 0 {
		return false, nil, nil, nil
	}

	return true, svcEP, headlessEP, nil
}

// getAccountFacts parse secrets for given cluster as facts, i.e., accounts created
// TODO: @shanshan, should verify accounts on database cluster as well.
func (r *SystemAccountReconciler) getAccountFacts(reqCtx intctrlutil.RequestCtx,
	namespace, clusterName string, clusterDefType, clusterDefName, compName string) (dbaasv1alpha1.KBAccountType, error) {
	// get account facts, i.e., secrets created
	ml := getLabelsForSecretsAndJobs(clusterName, clusterDefType, clusterDefName, compName)

	secrets := &corev1.SecretList{}
	if err := r.Client.List(reqCtx.Ctx, secrets, client.InNamespace(namespace), ml); err != nil {
		return dbaasv1alpha1.KBAccountInvalid, err
	}

	// get all running jobs
	jobs := &batchv1.JobList{}
	if err := r.Client.List(reqCtx.Ctx, jobs, client.InNamespace(namespace), ml); err != nil {
		return dbaasv1alpha1.KBAccountInvalid, err
	}

	detectedFacts := getAccountFacts(secrets, jobs)
	return detectedFacts, nil
}

// findClusterForBackupPolicy is an util function to build a mapping between BackupPolicy and Cluster.
func (r *SystemAccountReconciler) findClusterForBackupPolicy(object client.Object) []reconcile.Request {
	backupPolicy, ok := object.(*dataprotectionv1alpha1.BackupPolicy)
	if !ok {
		return nil
	}
	clusterName, exists := backupPolicy.Spec.Target.LabelsSelector.MatchLabels[intctrlutil.AppInstanceLabelKey]
	if !exists {
		return nil
	}
	cluster := &dbaasv1alpha1.Cluster{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: backupPolicy.GetNamespace(), Name: clusterName}, cluster)
	if err != nil {
		return nil
	}
	requests := []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      cluster.GetName(),
				Namespace: cluster.GetNamespace(),
			},
		},
	}
	return requests
}

// Create implements default CreateEvent filter on backupPolicy creation.
// It will regisiter the accounts to be created in ExpectionManager.
func (r *backupPolicyChangePredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
	}
	backupPolicy, ok := e.Object.(*dataprotectionv1alpha1.BackupPolicy)
	if !ok {
		return false
	}
	// TODO:@shanshan
	// BackupPolicy, for now, does not sepcify clearly which Cluster Component is works for.
	// So we resort to a tricky way, binding BackupPolicy.Spec.Target.DatabaeEngine to Cluster.Componet.CharacterType.
	targetCluster := backupPolicy.Spec.Target
	ml := targetCluster.LabelsSelector.MatchLabels
	databaseEngine := targetCluster.DatabaseEngine

	if clusterName, exists := ml[intctrlutil.AppInstanceLabelKey]; exists {
		key := expectationKey(backupPolicy.Namespace, clusterName, databaseEngine)
		expect, exists, err := r.ExpectionManager.getExpectation(key)
		if err != nil {
			r.Log.Error(err, "failed to get expectation for BackupPolicy by key", "BackupPolicy key", key)
			return false
		}
		if !exists {
			expect, err = r.ExpectionManager.createExpectation(key)
			if err != nil {
				r.Log.Error(err, "failed to create expectation for BackupPolicy by key", "BackupPolicy key", key)
			}
		}
		expect.set(dbaasv1alpha1.KBAccountDataprotection)
		return true
	}
	return false
}

// Delete implements default DeleteEvent filter on backupPolicy deletion.
// It will remove the accounts to be created from ExpectionManager.
func (r *backupPolicyChangePredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		return false
	}
	if backupPolicy, ok := e.Object.(*dataprotectionv1alpha1.BackupPolicy); !ok {
		return false
	} else {
		// BackupPolicy, for the moment, does not sepcify clearly which Cluster Component is works for.
		// So we resort to a tricky way, binding BackupPolicy.Spec.Target.DatabaeEngine to Cluster.Componet.CharacterType.
		// TODO: @shanshan
		targetCluster := backupPolicy.Spec.Target
		ml := targetCluster.LabelsSelector.MatchLabels
		databaseEngine := targetCluster.DatabaseEngine

		if clusterName, exists := ml[intctrlutil.AppInstanceLabelKey]; exists {
			key := expectationKey(backupPolicy.Namespace, clusterName, databaseEngine)
			err := r.ExpectionManager.deleteExpectation(key)
			if err != nil {
				r.Log.Error(err, "failed to delete expectation for BackupPolicy", "BackupPolicy key", key)
			}
		}
		return false
	}
}

// Delete implements default DeleteEvent filter on job deletion.
// If the job for creating account completes successfully, corresponding secret will be created.
func (r *jobCompletitionPredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		return false
	}
	if job, ok := e.Object.(*batchv1.Job); !ok {
		return false
	} else {
		ml := job.ObjectMeta.Labels
		accountName, ok := ml[clusterAccountLabelKey]
		if !ok {
			return false
		}
		clusterName, ok := ml[intctrlutil.AppInstanceLabelKey]
		if !ok {
			return false
		}
		componentName, ok := ml[intctrlutil.AppComponentLabelKey]
		if !ok {
			return false
		}

		for _, jobCond := range job.Status.Conditions {
			if jobCond.Type == batchv1.JobComplete && jobCond.Status == corev1.ConditionTrue {
				// job for cluster-component-account succeeded
				// create secret for this account
				key := concatSecretName(job.Namespace, clusterName, componentName, accountName)
				entry, ok, err := r.reconciler.SecretMapStore.getSecret(key)
				if err != nil || !ok {
					return false
				}
				if err := r.reconciler.Client.Create(context.TODO(), entry.value); err == nil {
					cluster := &dbaasv1alpha1.Cluster{}
					err := r.reconciler.Client.Get(context.TODO(), types.NamespacedName{Namespace: job.Namespace, Name: clusterName}, cluster)
					if err != nil {
						r.Log.Error(err, "failed to create secret", "secret key", key)
						return false
					}
					r.reconciler.Recorder.Eventf(cluster, corev1.EventTypeNormal, SysAcctCreate,
						"Created Accounts for cluster: %s, component: %s, accounts: %s", cluster.Name, componentName, accountName)
					// delete secret from cache store
					if err = r.reconciler.SecretMapStore.deleteSecret(key); err != nil {
						r.Log.Error(err, "failed to delete secret by key", "secret key", key)
					}
					return false
				}
			}
		}
		return false
	}
}

// Delete removes cached entries from SystemAccountReconciler.SecretMapStore
func (r *clusterDeletionPredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		return false
	}
	cluster, ok := e.Object.(*dbaasv1alpha1.Cluster)
	if !ok {
		return false
	}

	// for each component from the cluster, delete cached secrets
	for _, comp := range cluster.Spec.Components {
		for _, accName := range getAllSysAccounts() {
			key := concatSecretName(cluster.Namespace, cluster.Name, comp.Name, string(accName))
			// delete left-over secrets, and ignore errors if it has been removed.
			_, exists, err := r.reconciler.SecretMapStore.getSecret(key)
			if err != nil {
				r.clusterLog.Error(err, "failed to get secrets", "secret key", key)
				continue
			}
			if !exists {
				continue
			}
			err = r.reconciler.SecretMapStore.deleteSecret(key)
			if err != nil {
				r.clusterLog.Error(err, "failed to delete secrets", "secret key", key)
			}
		}
	}
	return false
}
