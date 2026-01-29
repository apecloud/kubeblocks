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

package component

import (
	"encoding/json"
	"fmt"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// componentRBACTransformer puts the RBAC objects at the beginning of the DAG
type componentRBACTransformer struct{}

var _ graph.Transformer = &componentRBACTransformer{}

const EventReasonRBACManager = "RBACManager"
const EventReasonServiceAccountRollback = "ServiceAccountRollback"

func (t *componentRBACTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	graphCli, _ := transCtx.Client.(model.GraphClient)
	synthesizedComp := transCtx.SynthesizeComponent
	comp := transCtx.Component
	if isCompDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create rbac related objects",
			"component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	var serviceAccountName string
	sa := &corev1.ServiceAccount{}
	// If the user has disabled rbac manager or specified comp.Spec.ServiceAccountName, it is now
	// the user's responsibility to provide appropriate serviceaccount.
	if serviceAccountName = transCtx.Component.Spec.ServiceAccountName; serviceAccountName != "" {
		// if user provided serviceaccount does not exist, raise error
		if err := transCtx.Client.Get(transCtx.Context, types.NamespacedName{Namespace: synthesizedComp.Namespace, Name: serviceAccountName}, sa); err != nil {
			if errors.IsNotFound(err) {
				transCtx.EventRecorder.Event(transCtx.Component, corev1.EventTypeWarning, EventReasonRBACManager,
					fmt.Sprintf("serviceaccount %v not found", serviceAccountName))
			}
			return err
		}
	}
	if !viper.GetBool(constant.EnableRBACManager) {
		transCtx.EventRecorder.Event(transCtx.Component, corev1.EventTypeNormal, EventReasonRBACManager, "RBAC manager is disabled")
		return nil
	}

	// user managed sa
	if serviceAccountName != "" {
		return t.handleRBACNewRule(transCtx, dag, serviceAccountName)
	}

	if err := t.handleRBACNewRule(transCtx, dag, ""); err != nil {
		return err
	}
	newName := constant.GenerateDefaultServiceAccountNameNew(synthesizedComp.FullCompName)
	runningITS := transCtx.RunningWorkload
	// new code path
	if runningITS == nil ||
		(runningITS.Annotations != nil && runningITS.Annotations[constant.ServiceAccountInUseAnnotationKey] == newName) {
		delete(comp.Annotations, constant.ComponentLastServiceAccountNameAnnotationKey)
		delete(comp.Annotations, constant.ComponentLastServiceAccountRuleHashAnnotationKey)

		// to prevent when an instanceset is newly created, its controller doesn't have time
		// to add the annoation for it. Then the component's reconciliation will
		// fall into old code path.
		synthesizedComp.AnnotaionsInjectedToWorkload[constant.ServiceAccountInUseAnnotationKey] = newName
		return nil
	}

	// old code path
	var err error
	lastServiceAccountName := comp.Annotations[constant.ComponentLastServiceAccountNameAnnotationKey]
	lastHash := comp.Annotations[constant.ComponentLastServiceAccountRuleHashAnnotationKey]
	if serviceAccountName == "" {
		rollback, err := needRollbackServiceAccount(transCtx)
		if err != nil {
			return err
		}

		serviceAccountName = constant.GenerateDefaultServiceAccountName(synthesizedComp.CompDefName)
		if serviceAccountName != lastServiceAccountName && rollback {
			transCtx.EventRecorder.Event(comp, corev1.EventTypeNormal, EventReasonServiceAccountRollback, "Change to serviceaccount has been rolled back to prevent pod restart")
			serviceAccountName = lastServiceAccountName
		}
		// if no rolebinding is needed, sa will be created anyway, because other modules may reference it.
		sa, err = createOrUpdateServiceAccount(transCtx, serviceAccountName, graphCli, dag)
		if err != nil {
			return err
		}

		hash, err := computeServiceAccountRuleHash(transCtx)
		if err != nil {
			return err
		}

		if lastServiceAccountName != serviceAccountName || lastHash != hash {
			comp.Annotations[constant.ComponentLastServiceAccountNameAnnotationKey] = serviceAccountName
			comp.Annotations[constant.ComponentLastServiceAccountRuleHashAnnotationKey] = hash
			graphCli.Update(dag, transCtx.ComponentOrig, transCtx.Component)
		}

		synthesizedComp.AnnotaionsInjectedToWorkload[constant.ProposedServiceAccountNameAnnotationKey] = newName
	}

	synthesizedComp.PodSpec.ServiceAccountName = serviceAccountName

	role, err := createOrUpdateRole(transCtx, graphCli, dag)
	if err != nil {
		return err
	}

	rbs, err := createOrUpdateRoleBinding(transCtx, role, serviceAccountName, graphCli, dag)
	if err != nil {
		return err
	}

	objs := []client.Object{role}
	if sa != nil {
		objs = append(objs, sa)
		// serviceAccount should be created before roleBinding and role
		for _, rb := range rbs {
			objs = append(objs, rb)
			graphCli.DependOn(dag, rb, sa, role)
		}
		// serviceAccount should be created before workload
		itsList := graphCli.FindAll(dag, &workloads.InstanceSet{})
		for _, its := range itsList {
			graphCli.DependOn(dag, its, sa)
		}
	}

	t.rbacInstanceAssistantObjects(graphCli, dag, objs)

	return nil
}

func (t *componentRBACTransformer) handleRBACNewRule(transCtx *componentTransformContext, dag *graph.DAG, userDefinedSAName string) error {
	synthesizedComp := transCtx.SynthesizeComponent
	graphCli, _ := transCtx.Client.(model.GraphClient)
	saName := userDefinedSAName
	var sa *corev1.ServiceAccount
	var err error
	if userDefinedSAName == "" {
		saName = constant.GenerateDefaultServiceAccountNameNew(synthesizedComp.FullCompName)
		// if no rolebinding is needed, sa will be created anyway, because other modules may reference it.
		sa, err = createOrUpdateServiceAccount(transCtx, saName, graphCli, dag)
		if err != nil {
			return err
		}
	}
	synthesizedComp.PodSpec.ServiceAccountName = saName
	rbs, err := createOrUpdateRoleBindingNew(transCtx, transCtx.CompDef, saName, graphCli, dag)
	if err != nil {
		return err
	}
	objs := []client.Object{sa}
	if sa != nil {
		// serviceAccount should be created before roleBinding and role
		for _, rb := range rbs {
			objs = append(objs, rb)
			graphCli.DependOn(dag, rb, sa)
		}
		// serviceAccount should be created before workload
		itsList := graphCli.FindAll(dag, &workloads.InstanceSet{})
		for _, its := range itsList {
			graphCli.DependOn(dag, its, sa)
		}
	}

	t.rbacInstanceAssistantObjects(graphCli, dag, objs)
	return nil
}

func createOrUpdateRoleBindingNew(transCtx *componentTransformContext,
	cmpd *appsv1.ComponentDefinition, serviceAccountName string, graphCli model.GraphClient, dag *graph.DAG) ([]*rbacv1.RoleBinding, error) {
	cmpRoleBinding := func(old, new *rbacv1.RoleBinding) bool {
		return labelAndAnnotationEqual(old, new) &&
			equality.Semantic.DeepEqual(old.Subjects, new.Subjects) &&
			equality.Semantic.DeepEqual(old.RoleRef, new.RoleRef)
	}
	res := make([]*rbacv1.RoleBinding, 0)

	if len(cmpd.Spec.PolicyRules) != 0 {
		// cluster role is handled by cmpd controller
		cmpdRoleBinding := factory.BuildRoleBinding(transCtx.SynthesizeComponent, serviceAccountName, &rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     constant.GenerateDefaultRoleName(cmpd.Name),
		}, serviceAccountName)
		if err := intctrlutil.SetOwnership(transCtx.Component, cmpdRoleBinding, model.GetScheme(), ""); err != nil {
			return nil, err
		}
		rb, err := createOrUpdate(transCtx, cmpdRoleBinding, graphCli, dag, cmpRoleBinding)
		if err != nil {
			return nil, err
		}
		res = append(res, rb)
	}

	if isLifecycleActionsEnabled(transCtx.CompDef) {
		clusterPodRoleBinding := factory.BuildRoleBinding(
			transCtx.SynthesizeComponent,
			fmt.Sprintf("%v-pod", serviceAccountName),
			&rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     constant.RBACRoleName,
			},
			serviceAccountName,
		)
		if err := intctrlutil.SetOwnership(transCtx.Component, clusterPodRoleBinding, model.GetScheme(), ""); err != nil {
			return nil, err
		}
		rb, err := createOrUpdate(transCtx, clusterPodRoleBinding, graphCli, dag, cmpRoleBinding)
		if err != nil {
			return nil, err
		}
		res = append(res, rb)
	}

	return res, nil
}

func (t *componentRBACTransformer) rbacInstanceAssistantObjects(graphCli model.GraphClient, dag *graph.DAG, objs []client.Object) {
	itsList := graphCli.FindAll(dag, &workloads.InstanceSet{})
	for _, itsObj := range itsList {
		its := itsObj.(*workloads.InstanceSet)
		component.AddInstanceAssistantObjectsToITS(its, objs...)
	}
}

func computeServiceAccountRuleHash(transCtx *componentTransformContext) (string, error) {
	hash := fnv.New32a()
	data, err := json.Marshal(transCtx.SynthesizeComponent.PolicyRules)
	if err != nil {
		return "", err
	}
	hash.Write(data)
	enabled := transCtx.SynthesizeComponent.LifecycleActions.ComponentLifecycleActions != nil
	fmt.Fprint(hash, enabled)
	return rand.SafeEncodeString(fmt.Sprintf("%d", hash.Sum32())), nil
}

func needRollbackServiceAccount(transCtx *componentTransformContext) (rollback bool, err error) {
	hash, err := computeServiceAccountRuleHash(transCtx)
	if err != nil {
		return false, err
	}

	lastHash, ok := transCtx.Component.Annotations[constant.ComponentLastServiceAccountRuleHashAnnotationKey]
	if !ok {
		return false, nil
	}

	return hash == lastHash, nil
}

func labelAndAnnotationEqual(old, new metav1.Object) bool {
	// exclude component labels, since they are different for each component
	compLabels := constant.GetCompLabels("", "")
	oldLabels := make(map[string]string)
	for k, v := range old.GetLabels() {
		if _, ok := compLabels[k]; !ok {
			oldLabels[k] = v
		}
	}
	newLabels := make(map[string]string)
	for k, v := range new.GetLabels() {
		if _, ok := compLabels[k]; !ok {
			newLabels[k] = v
		}
	}
	return equality.Semantic.DeepEqual(oldLabels, newLabels) &&
		equality.Semantic.DeepEqual(old.GetAnnotations(), new.GetAnnotations())
}

func createOrUpdate[T any, PT generics.PObject[T]](transCtx *componentTransformContext,
	obj PT, graphCli model.GraphClient, dag *graph.DAG, cmpFn func(oldObj, newObj PT) bool) (PT, error) {
	oldObj := PT(new(T))
	if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(obj), oldObj); err != nil {
		if errors.IsNotFound(err) {
			graphCli.Create(dag, obj)
			return obj, nil
		}
		return nil, err
	}
	// adopt any orphaned object
	if !cmpFn(oldObj, obj) || metav1.GetControllerOf(oldObj) == nil {
		transCtx.Logger.V(1).Info("updating rbac resources",
			"name", klog.KObj(obj).String(), "obj", fmt.Sprintf("%#v", obj))
		graphCli.Update(dag, oldObj, obj)
	}
	return obj, nil
}

func createOrUpdateServiceAccount(transCtx *componentTransformContext,
	serviceAccountName string, graphCli model.GraphClient, dag *graph.DAG) (*corev1.ServiceAccount, error) {
	synthesizedComp := transCtx.SynthesizeComponent

	sa := factory.BuildServiceAccount(synthesizedComp, serviceAccountName)
	if err := intctrlutil.SetOwnership(transCtx.Component, sa, model.GetScheme(), ""); err != nil {
		return nil, err
	}

	return createOrUpdate(transCtx, sa, graphCli, dag, func(old, new *corev1.ServiceAccount) bool {
		return labelAndAnnotationEqual(old, new) &&
			equality.Semantic.DeepEqual(old.ImagePullSecrets, new.ImagePullSecrets) &&
			equality.Semantic.DeepEqual(old.AutomountServiceAccountToken, new.AutomountServiceAccountToken)
	})
}

func createOrUpdateRole(transCtx *componentTransformContext, graphCli model.GraphClient, dag *graph.DAG) (*rbacv1.Role, error) {
	role := factory.BuildRole(transCtx.SynthesizeComponent, transCtx.CompDef)
	if role == nil {
		return nil, nil
	}
	if err := intctrlutil.SetOwnership(transCtx.Component, role, model.GetScheme(), ""); err != nil {
		return nil, err
	}
	return createOrUpdate(transCtx, role, graphCli, dag, func(old, new *rbacv1.Role) bool {
		return labelAndAnnotationEqual(old, new) &&
			equality.Semantic.DeepEqual(old.Rules, new.Rules)
	})
}

func createOrUpdateRoleBinding(transCtx *componentTransformContext,
	cmpdRole *rbacv1.Role, serviceAccountName string, graphCli model.GraphClient, dag *graph.DAG) ([]*rbacv1.RoleBinding, error) {
	cmpRoleBinding := func(old, new *rbacv1.RoleBinding) bool {
		return labelAndAnnotationEqual(old, new) &&
			equality.Semantic.DeepEqual(old.Subjects, new.Subjects) &&
			equality.Semantic.DeepEqual(old.RoleRef, new.RoleRef)
	}
	res := make([]*rbacv1.RoleBinding, 0)

	if cmpdRole != nil {
		cmpdRoleBinding := factory.BuildRoleBinding(transCtx.SynthesizeComponent, serviceAccountName, &rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     cmpdRole.Name,
		}, serviceAccountName)
		if err := intctrlutil.SetOwnership(transCtx.Component, cmpdRoleBinding, model.GetScheme(), ""); err != nil {
			return nil, err
		}
		rb, err := createOrUpdate(transCtx, cmpdRoleBinding, graphCli, dag, cmpRoleBinding)
		if err != nil {
			return nil, err
		}
		res = append(res, rb)
	}

	if isLifecycleActionsEnabled(transCtx.CompDef) {
		clusterPodRoleBinding := factory.BuildRoleBinding(
			transCtx.SynthesizeComponent,
			fmt.Sprintf("%v-pod", serviceAccountName),
			&rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     constant.RBACRoleName,
			},
			serviceAccountName,
		)
		if err := intctrlutil.SetOwnership(transCtx.Component, clusterPodRoleBinding, model.GetScheme(), ""); err != nil {
			return nil, err
		}
		rb, err := createOrUpdate(transCtx, clusterPodRoleBinding, graphCli, dag, cmpRoleBinding)
		if err != nil {
			return nil, err
		}
		res = append(res, rb)
	}

	return res, nil
}

func isLifecycleActionsEnabled(compDef *appsv1.ComponentDefinition) bool {
	return compDef.Spec.LifecycleActions != nil
}
