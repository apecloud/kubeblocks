/*
Copyright 2022.

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
	"regexp"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ConsensusSetReconciler reconciles a ConsensusSet object
type ConsensusSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=consensussets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=consensussets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=consensussets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ConsensusSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *ConsensusSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("consensusset", req.NamespacedName),
	}

	cs := &dbaasv1alpha1.ConsensusSet{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cs); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, cs, consensusSetFinalizerName, func() (*ctrl.Result, error) {
		return r.deleteExternalResources(reqCtx, cs)
	})
	if err != nil {
		return *res, err
	}

	if cs.Status.ObservedGeneration == cs.GetObjectMeta().GetGeneration() {
		err := r.updateStatusBySubResources(reqCtx.Ctx, cs)
		if err != nil {
			return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "updateStatusBySubResourcesError: "+err.Error())
		}

		return intctrlutil.Reconciled()
	}

	// create or update cs
	task, err := buildConsensusSetCreationTasks(cs)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "buildConsensusSetCreationTasksError")
	}
	err = task.Exec(ctx, r.Client)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "buildConsensusSetCreationTaskExecError")
	}

	// update generation
	patch := client.MergeFrom(cs.DeepCopy())
	cs.Status.ObservedGeneration = cs.Generation
	err = r.Client.Status().Patch(ctx, cs, patch)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "updateStatusObservedGenerationError")
	}

	return intctrlutil.Reconciled()
}

// buildConsensusSetCreationTasks build creation or update tasks based on current status and target spec
// TODO need a better name
func buildConsensusSetCreationTasks(
	cs *dbaasv1alpha1.ConsensusSet) (*intctrlutil.Task, error) {
	rootTask := intctrlutil.NewTask()

	// create readWrite service
	// TODO if service not created
	rootTask.SubTasks = prepareServiceCreationTask(rootTask.SubTasks, cs, dbaasv1alpha1.ReadWrite)

	// create readonly service
	// TODO if service not created
	rootTask.SubTasks = prepareServiceCreationTask(rootTask.SubTasks, cs, dbaasv1alpha1.Readonly)

	// create headless service
	rootTask.SubTasks = prepareHeadlessServiceCreationTask(rootTask.SubTasks, cs)

	// create stateful set
	rootTask.SubTasks = prepareStatefulSetCreationTask(rootTask.SubTasks, cs)

	return &rootTask, nil
}

type serviceParams struct {
	setName        types.NamespacedName
	selectorLabels []string
	// port           int
}

func prepareStatefulSetCreationTask(tasks []intctrlutil.Task, cs *dbaasv1alpha1.ConsensusSet) []intctrlutil.Task {
	// TODO finish me
	return tasks
}

func prepareHeadlessServiceCreationTask(tasks []intctrlutil.Task, cs *dbaasv1alpha1.ConsensusSet) []intctrlutil.Task {
	// TODO finish me
	return tasks
}

func prepareServiceCreationTask(tasks []intctrlutil.Task, cs *dbaasv1alpha1.ConsensusSet, accessMode dbaasv1alpha1.AccessMode) []intctrlutil.Task {
	serviceTask := intctrlutil.NewTask()
	serviceTask.ExecFunction = prepareServiceObj
	params := prepareServiceParams(cs, accessMode)
	serviceTask.Context["exec"] = params
	return append(tasks, serviceTask)
}

func prepareServiceParams(cs *dbaasv1alpha1.ConsensusSet, accessMode dbaasv1alpha1.AccessMode) *serviceParams {
	params := &serviceParams{}
	params.setName = types.NamespacedName{
		Namespace: cs.Namespace,
		Name:      cs.Name + "-" + string(accessMode),
	}
	// TODO port mapping to port in PodSpec
	params.selectorLabels = appendLabelsIfMatch(make([]string, 0), accessMode, cs.Spec.Leader, cs.Spec.Learner)

	for _, member := range cs.Spec.Followers {
		params.selectorLabels = appendLabelsIfMatch(params.selectorLabels, accessMode, member)
	}

	return params
}

func appendLabelsIfMatch(labels []string, accessMode dbaasv1alpha1.AccessMode, members ...dbaasv1alpha1.ConsensusMember) []string {
	for _, member := range members {
		if accessMode == member.AccessMode {
			labels = append(labels, consensusSetRoleLabelKey+"="+member.Name)
		}
	}

	return labels
}

func prepareServiceObj(ctx context.Context, cli client.Client, params interface{}) error {
	// TODO finish me

	return nil
}

func (r *ConsensusSetReconciler) updateStatusBySubResources(ctx context.Context, cs *dbaasv1alpha1.ConsensusSet) error {
	// observe statefulset status and update
	ssList := &appsv1.StatefulSetList{}
	err := r.Client.List(
		ctx,
		ssList,
		&client.ListOptions{Namespace: cs.Namespace},
		client.MatchingLabels{consensusSetLabelKey: cs.GetName()})
	if err != nil {
		return err
	}

	patch := client.MergeFrom(cs.DeepCopy())

	// should be one StatefulSet, we don't check it
	ss := &ssList.Items[0]
	cs.Status.Replicas = ss.Status.Replicas
	cs.Status.ReadyReplicas = ss.Status.ReadyReplicas

	// observe consensus roles
	// filter pods owned by this ss: copy from StatefulSet
	podList := &corev1.PodList{}
	err = r.Client.List(
		ctx,
		podList,
		&client.ListOptions{Namespace: cs.Namespace},
		client.MatchingLabelsSelector{Selector: labels.Everything()})
	if err != nil {
		return err
	}
	// read labels and count
	readyLeader, readyFollowers, readyLearners := 0, 0, 0
	for _, pod := range podList.Items {
		if !isMemberOf(ss, &pod) {
			continue
		}
		roleLabel := pod.GetLabels()[consensusSetRoleLabelKey]

		switch {
		case isLeader(cs, roleLabel):
			readyLeader++
		case isFollower(cs, roleLabel):
			readyFollowers++
		case isLearner(cs, roleLabel):
			readyLearners++
		}
	}
	// update cs status.ReadyLeader, ReadyFollowers and ReadyLearners
	cs.Status.ReadyLeader = int32(readyLeader)
	cs.Status.ReadyFollowers = int32(readyFollowers)
	cs.Status.ReadyLearners = int32(readyLearners)

	// observe readwrite service status and update
	// get rw service: label: cs-name-rw
	rwSvc := &corev1.Service{}
	err = r.Client.Get(ctx, getReadWriteServiceName(cs), rwSvc)
	if err != nil {
		return err
	}

	// observe service.status.lb readiness
	// TODO how to check LoadBalancer presence ?
	if rwSvc.Status.LoadBalancer.Ingress != nil {
		cs.Status.IsReadWriteServiceReady = true
	}

	// observe readonly service status and update
	// get rw service: label: cs-name-ro
	roSvc := &corev1.Service{}
	err = r.Client.Get(ctx, getReadOnlyServiceName(cs), roSvc)
	if err != nil {
		return err
	}
	// observe service.status.lb readiness
	if roSvc.Status.LoadBalancer.Ingress != nil {
		cs.Status.IsReadonlyServiceReady = true
	}

	// TODO append Condition based on progress

	err = r.Client.Status().Patch(ctx, cs, patch)
	if err != nil {
		return err
	}

	return nil
}

func getReadOnlyServiceName(cs *dbaasv1alpha1.ConsensusSet) types.NamespacedName {
	return types.NamespacedName{
		Namespace: cs.Namespace,
		Name:      cs.Name + "-ro",
	}
}

func getReadWriteServiceName(cs *dbaasv1alpha1.ConsensusSet) types.NamespacedName {
	return types.NamespacedName{
		Namespace: cs.Namespace,
		Name:      cs.Name + "-rw",
	}
}

func isLearner(cs *dbaasv1alpha1.ConsensusSet, roleLabel string) bool {
	return roleLabel == cs.Spec.Learner.Name
}

func isFollower(cs *dbaasv1alpha1.ConsensusSet, roleLabel string) bool {
	if cs.Spec.Followers == nil {
		return false
	}

	for _, follower := range cs.Spec.Followers {
		if roleLabel == follower.Name {
			return true
		}
	}

	return false
}

func isLeader(cs *dbaasv1alpha1.ConsensusSet, roleLabel string) bool {
	return roleLabel == cs.Spec.Leader.Name
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConsensusSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.ConsensusSet{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Complete(r)
}

func (r *ConsensusSetReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, cs *dbaasv1alpha1.ConsensusSet) (*ctrl.Result, error) {
	// TODO finish me

	return nil, nil
}

// ------- copy from stateful_set_utils.go ----
// statefulPodRegex is a regular expression that extracts the parent StatefulSet and ordinal from the Name of a Pod
var statefulPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")

// getParentNameAndOrdinal gets the name of pod's parent StatefulSet and pod's ordinal as extracted from its Name. If
// the Pod was not created by a StatefulSet, its parent is considered to be empty string, and its ordinal is considered
// to be -1.
func getParentNameAndOrdinal(pod *corev1.Pod) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := statefulPodRegex.FindStringSubmatch(pod.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

// getParentName gets the name of pod's parent StatefulSet. If pod has not parent, the empty string is returned.
func getParentName(pod *corev1.Pod) string {
	parent, _ := getParentNameAndOrdinal(pod)
	return parent
}

// isMemberOf tests if pod is a member of set.
func isMemberOf(set *appsv1.StatefulSet, pod *corev1.Pod) bool {
	return getParentName(pod) == set.Name
}

// getPodRevision gets the revision of Pod by inspecting the StatefulSetRevisionLabel. If pod has no revision the empty
// string is returned.
func getPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[appsv1.StatefulSetRevisionLabel]
}

// ------- end copy from stateful_set_utils.go ----
