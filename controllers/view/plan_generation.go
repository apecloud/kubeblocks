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
	"hash/fnv"
	"reflect"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
	v, _ := tree.GetRoot().(*viewv1.ReconciliationView)
	if isDesiredSpecChanged(v) {
		return kubebuilderx.ConditionSatisfied
	}
	return kubebuilderx.ConditionUnsatisfied
}

func isDesiredSpecChanged(v *viewv1.ReconciliationView) bool {
	if v.Spec.DryRun == nil && v.Status.DryRunResult == nil {
		return false
	}
	if v.Spec.DryRun == nil || v.Status.DryRunResult == nil {
		return true
	}
	revision := getDesiredSpecRevision(v.Spec.DryRun.DesiredSpec)
	return revision != v.Status.DryRunResult.DesiredSpecRevision
}

func getDesiredSpecRevision(desiredSpec string) string {
	hf := fnv.New32()
	_, _ = hf.Write([]byte(desiredSpec))
	return rand.SafeEncodeString(fmt.Sprint(hf.Sum32()))
}

func (g *planGenerator) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	view, _ := tree.GetRoot().(*viewv1.ReconciliationView)
	objs := tree.List(&corev1.ConfigMap{})
	var i18nResource *corev1.ConfigMap
	if len(objs) > 0 {
		i18nResource, _ = objs[0].(*corev1.ConfigMap)
	}

	root := &appsv1alpha1.Cluster{}
	objectKey := client.ObjectKeyFromObject(view)
	if view.Spec.TargetObject != nil {
		objectKey = client.ObjectKey{
			Namespace: view.Spec.TargetObject.Namespace,
			Name:      view.Spec.TargetObject.Name,
		}
	}
	if err := g.cli.Get(g.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}

	// create mock client and mock event recorder
	// kbagent client is running in dry-run mode by setting context key-value pair: dry-run=true
	store := newChangeCaptureStore(g.cli.Scheme(), i18nResource)
	mClient, err := newMockClient(g.cli, store, KBOwnershipRules)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	mEventRecorder := newMockEventRecorder(store)

	// build reconciler tree based on ownership rules:
	// 1. each gvk has a corresponding reconciler
	// 2. mock K8s native object reconciler
	// 3. encapsulate KB controller as reconciler
	reconcilerTree, err := newReconcilerTree(g.ctx, mClient, mEventRecorder, KBOwnershipRules)
	if err != nil {
		return kubebuilderx.Commit, err
	}

	// load object store
	if err = loadCurrentObjectTree(g.ctx, g.cli, root, KBOwnershipRules, store); err != nil {
		return kubebuilderx.Commit, err
	}
	initialObjectMap := store.GetAll()

	// apply plan.spec.desiredSpec to root object
	var specChange string
	if specChange, err = applyDesiredSpec(view.Spec.DryRun.DesiredSpec, root); err != nil {
		return kubebuilderx.Commit, err
	}
	if err = mClient.Update(g.ctx, root); err != nil {
		return kubebuilderx.Commit, err
	}

	// start plan generation loop
	expr := defaultStateEvaluationExpression
	if view.Spec.StateEvaluationExpression != nil {
		expr = *view.Spec.StateEvaluationExpression
	}
	for {
		changeCount := len(store.GetChanges())

		// run reconciler tree
		if err = reconcilerTree.Run(); err != nil {
			return kubebuilderx.Commit, err
		}

		// state evaluation
		if err = mClient.Get(g.ctx, objectKey, root); err != nil {
			return kubebuilderx.Commit, err
		}
		state, err := doStateEvaluation(root, expr)
		if err != nil {
			return kubebuilderx.Commit, err
		}

		// final state with no more changes happen, the reconciliation cycle is done.
		if state && changeCount == len(store.GetChanges()) {
			break
		}
	}

	// update dry-run result
	dryRunResult := view.Status.DryRunResult
	if dryRunResult == nil {
		dryRunResult = &viewv1.DryRunResult{}
	}
	dryRunResult.DesiredSpecRevision = getDesiredSpecRevision(view.Spec.DryRun.DesiredSpec)
	dryRunResult.ObservedTargetGeneration = root.Generation
	dryRunResult.Phase = viewv1.DryRunSucceedPhase
	if err = g.cli.Get(g.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}
	desiredRoot := &appsv1alpha1.Cluster{}
	if err = mClient.Get(g.ctx, objectKey, desiredRoot); err != nil {
		return kubebuilderx.Commit, err
	}
	desiredTree, err := getObjectTreeFromCache(g.ctx, mClient, desiredRoot, KBOwnershipRules)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	dryRunResult.FinalObjectTree = *desiredTree
	dryRunResult.Plan = store.GetChanges()
	dryRunResult.PlanSummary.SpecChange = specChange
	newObjectMap := store.GetAll()
	dryRunResult.PlanSummary.ObjectSummaries = buildObjectSummaries(sets.KeySet(initialObjectMap), sets.KeySet(newObjectMap), initialObjectMap, newObjectMap)

	// TODO(free6om): put the plan generation loop into a timeout goroutine

	return kubebuilderx.Continue, nil
}

// getSpecFieldAsStruct extracts the Spec field from a client.Object and returns it as an interface{}.
func getSpecFieldAsStruct(obj client.Object) (interface{}, error) {
	// Get the value of the object
	objValue := reflect.ValueOf(obj)

	// Check if the object is a pointer to a struct
	if objValue.Kind() != reflect.Ptr || objValue.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("obj must be a pointer to a struct")
	}

	// Get the Spec field
	specField := objValue.Elem().FieldByName("Spec")
	if !specField.IsValid() {
		return nil, fmt.Errorf("spec field not found")
	}

	// Return the Spec field as an interface{}
	return specField.Interface(), nil
}

func applyDesiredSpec(desiredSpec string, obj client.Object) (string, error) {
	// Convert the desiredSpec YAML string to a map
	specMap := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(desiredSpec), &specMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal desiredSpec: %w", err)
	}

	// Extract the current spec and apply the patch
	currentSpec, err := getSpecFieldAsStruct(obj)
	if err != nil {
		return "", fmt.Errorf("failed to get current spec: %w", err)
	}

	// Create a strategic merge patch
	patch, err := strategicpatch.CreateTwoWayMergePatch(
		specMapToJSON(currentSpec),
		specMapToJSON(specMap),
		currentSpec,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create merge patch: %w", err)
	}

	// Apply the patch to the current spec
	modifiedSpec, err := strategicpatch.StrategicMergePatch(
		specMapToJSON(currentSpec),
		patch,
		currentSpec,
	)
	if err != nil {
		return "", fmt.Errorf("failed to apply merge patch: %w", err)
	}
	modifiedSpecMap := make(map[string]interface{})
	if err = json.Unmarshal(modifiedSpec, &modifiedSpecMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal final spec: %w", err)
	}

	// Convert the object to an unstructured map
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return "", fmt.Errorf("failed to convert object to unstructured: %w", err)
	}

	// Extract the initial spec
	initialSpec, _, err := unstructured.NestedMap(objMap, "spec")
	if err != nil {
		return "", fmt.Errorf("failed to get current spec: %w", err)
	}

	// Update the spec in the object map
	if err := unstructured.SetNestedField(objMap, modifiedSpecMap, "spec"); err != nil {
		return "", fmt.Errorf("failed to set modified spec: %w", err)
	}

	// Convert the modified map back to the original object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(objMap, obj); err != nil {
		return "", fmt.Errorf("failed to convert back to object: %w", err)
	}

	// build the spec change
	specChange := cmp.Diff(initialSpec, modifiedSpecMap)

	return specChange, nil
}

func loadCurrentObjectTree(ctx context.Context, cli client.Client, root *appsv1alpha1.Cluster, ownershipRules []OwnershipRule, store ChangeCaptureStore) error {
	_, objectMap, err := getObjectsFromCache(ctx, cli, root, ownershipRules)
	if err != nil {
		return err
	}
	for _, object := range objectMap {
		if err := store.Load(object); err != nil {
			return err
		}
	}
	return nil
}

func planGeneration(ctx context.Context, cli client.Client) kubebuilderx.Reconciler {
	return &planGenerator{
		ctx: context.WithValue(ctx, constant.DryRunContextKey, true),
		cli: cli,
	}
}

var _ kubebuilderx.Reconciler = &planGenerator{}
