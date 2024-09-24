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

package view

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type planGenerator struct {
	ctx context.Context
	cli client.Client
}

func (g *planGenerator) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	p, _ := tree.GetRoot().(*viewv1.ReconciliationPlan)
	if p.Generation == p.Status.ObservedPlanGeneration {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (g *planGenerator) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	plan, _ := tree.GetRoot().(*viewv1.ReconciliationPlan)
	viewDef, _ := tree.List(&viewv1.ReconciliationViewDefinition{})[0].(*viewv1.ReconciliationViewDefinition)
	objs := tree.List(&corev1.ConfigMap{})
	var i18nResource *corev1.ConfigMap
	if len(objs) > 0 {
		i18nResource, _ = objs[0].(*corev1.ConfigMap)
	}

	root := &appsv1alpha1.Cluster{}
	objectKey := client.ObjectKeyFromObject(plan)
	if plan.Spec.TargetObject != nil {
		objectKey = client.ObjectKey{
			Namespace: plan.Spec.TargetObject.Namespace,
			Name:      plan.Spec.TargetObject.Name,
		}
	}
	if err := g.cli.Get(g.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}

	// create mock client, mock event recorder, mock kbagent client
	store := make(map[model.GVKNObjKey]client.Object)
	mClient := newMockClient(g.cli, store)
	mEventRecorder := newMockEventRecorder(store)
	// build reconciler tree based on ownership rules:
	// 1. each gvk has a corresponding reconciler
	// 2. mock K8s native object reconciler
	// 3. encapsulate KB controller as reconciler
	reconcilerTree, err := newReconcilerTree(viewDef.Spec.OwnershipRules, mClient, mEventRecorder)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	// load object store
	if err = loadCurrentObjectTree(g.ctx, g.cli, root, viewDef.Spec.OwnershipRules, store); err != nil {
		return kubebuilderx.Commit, err
	}
	// apply plan.spec.desiredSpec to root object
	if err = applyDesiredSpec(plan.Spec.DesiredSpec, root); err != nil {
		return kubebuilderx.Commit, err
	}

	// start plan generation loop
	for {
		// deepcopy object store into old store
		oldStore := deepCopyStore(store)
		// run reconciler tree
		if err = reconcilerTree.Run(); err != nil {
			return kubebuilderx.Commit, err
		}
		// compare object store(new store) to old store
		oldSet := sets.KeySet(oldStore)
		newSet := sets.KeySet(store)
		createSet := newSet.Difference(oldSet)
		updateSet := newSet.Intersection(oldSet)
		deleteSet := oldSet.Difference(newSet)
		// append changes to status.plan
		var planSlice []viewv1.ObjectChange
		changes := buildChanges(createSet, oldStore, store, viewv1.ObjectCreationType, i18nResource, viewDef.Spec.Locale, plan.Spec.Locale)
		planSlice = append(planSlice, changes...)
		changes = buildChanges(updateSet, oldStore, store, viewv1.ObjectUpdateType, i18nResource, viewDef.Spec.Locale, plan.Spec.Locale)
		planSlice = append(planSlice, changes...)
		changes = buildChanges(deleteSet, oldStore, store, viewv1.ObjectDeletionType, i18nResource, viewDef.Spec.Locale, plan.Spec.Locale)
		planSlice = append(planSlice, changes...)
		//
		// state evaluation
		// if true, break
		// else continue plan generation loop
		expr := viewDef.Spec.StateEvaluationExpression
		if plan.Spec.StateEvaluationExpression != nil {
			expr = *plan.Spec.StateEvaluationExpression
		}
		state, err := doStateEvaluation(root, expr)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		if state {
			break
		}
	}
	//
	// TODO(free6om): put the plan generation loop into a timeout goroutine

	return kubebuilderx.Continue, nil
}

func deepCopyStore(store map[model.GVKNObjKey]client.Object) map[model.GVKNObjKey]client.Object {
	newStore := make(map[model.GVKNObjKey]client.Object, len(store))
	for key, object := range store {
		newStore[key] = object.DeepCopyObject().(client.Object)
	}
	return newStore
}

func applyDesiredSpec(desiredSpec string, obj client.Object) error {
	// Convert the desiredSpec YAML string to a map
	specMap := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(desiredSpec), &specMap); err != nil {
		return fmt.Errorf("failed to unmarshal desiredSpec: %w", err)
	}

	// Convert the object to an unstructured map
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return fmt.Errorf("failed to convert object to unstructured: %w", err)
	}

	// Extract the current spec and apply the patch
	currentSpec, _, err := unstructured.NestedMap(objMap, "spec")
	if err != nil {
		return fmt.Errorf("failed to get current spec: %w", err)
	}

	specMapToJSON := func(spec interface{}) []byte {
		// Convert the spec map to JSON for the patch functions
		specJSON, _ := json.Marshal(spec)
		return specJSON
	}

	// Create a strategic merge patch
	patch, err := strategicpatch.CreateTwoWayMergePatch(
		specMapToJSON(currentSpec),
		specMapToJSON(specMap),
		currentSpec,
	)
	if err != nil {
		return fmt.Errorf("failed to create merge patch: %w", err)
	}

	// Apply the patch to the current spec
	modifiedSpec, err := strategicpatch.StrategicMergePatch(
		specMapToJSON(currentSpec),
		patch,
		currentSpec,
	)
	if err != nil {
		return fmt.Errorf("failed to apply merge patch: %w", err)
	}

	// Update the spec in the object map
	if err := unstructured.SetNestedField(objMap, modifiedSpec, "spec"); err != nil {
		return fmt.Errorf("failed to set modified spec: %w", err)
	}

	// Convert the modified map back to the original object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(objMap, obj); err != nil {
		return fmt.Errorf("failed to convert back to object: %w", err)
	}

	return nil
}

func loadCurrentObjectTree(ctx context.Context, cli client.Client, root *appsv1alpha1.Cluster, ownershipRules []viewv1.OwnershipRule, store map[model.GVKNObjKey]client.Object) error {
	_, objectMap, err := getObjectsFromCache(ctx, cli, root, ownershipRules)
	if err != nil {
		return err
	}
	for key, object := range objectMap {
		store[key] = object
	}
	return nil
}

func planGeneration(cli client.Client) kubebuilderx.Reconciler {
	return &planGenerator{cli: cli}
}

var _ kubebuilderx.Reconciler = &planGenerator{}
