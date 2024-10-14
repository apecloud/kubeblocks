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
	"reflect"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type PlanGenerator interface {
	generatePlan(root *kbappsv1.Cluster, patch client.Patch) (*viewv1.DryRunResult, error)
}

type objectLoader func() (map[model.GVKNObjKey]client.Object, error)
type descriptionFormatter func(client.Object, client.Object, viewv1.ObjectChangeType, *schema.GroupVersionKind) (string, *string)

type planGenerator struct {
	ctx       context.Context
	cli       client.Client
	scheme    *runtime.Scheme
	loader    objectLoader
	formatter descriptionFormatter
}

func (g *planGenerator) generatePlan(root *kbappsv1.Cluster, patch client.Patch) (*viewv1.DryRunResult, error) {
	// create mock client and mock event recorder
	// kbagent client is running in dry-run mode by setting context key-value pair: dry-run=true
	store := newChangeCaptureStore(g.scheme, g.formatter)
	mClient, err := newMockClient(g.cli, store, getKBOwnershipRules())
	if err != nil {
		return nil, err
	}
	mEventRecorder := newMockEventRecorder(store)

	// build reconciler tree based on ownership rules:
	// 1. each gvk has a corresponding reconciler
	// 2. mock K8s native object reconciler
	// 3. encapsulate KB controller as reconciler
	reconcilerTree, err := newReconcilerTree(g.ctx, mClient, mEventRecorder, getKBOwnershipRules())
	if err != nil {
		return nil, err
	}

	// load current object tree into store
	if err = loadCurrentObjectTree(g.loader, store); err != nil {
		return nil, err
	}
	initialObjectMap := store.GetAll()

	// apply dryRun.desiredSpec to target cluster object
	var specDiff string
	if specDiff, err = applyPatch(root, patch); err != nil {
		return nil, err
	}
	if err = mClient.Update(g.ctx, root); err != nil {
		return nil, err
	}

	// generate plan with timeout
	startTime := time.Now()
	timeout := false
	var reconcileErr error
	previousCount := len(store.GetChanges())
	for {
		if time.Since(startTime) > time.Second {
			timeout = true
			break
		}

		// run reconciler tree
		if reconcileErr = reconcilerTree.Run(); reconcileErr != nil {
			break
		}

		// no change means reconciliation cycle is done
		currentCount := len(store.GetChanges())
		if currentCount == previousCount {
			break
		}
		previousCount = currentCount
	}

	// update dry-run result
	// update spec info
	dryRunResult := &viewv1.DryRunResult{}
	dryRunResult.ObservedTargetGeneration = root.Generation
	dryRunResult.SpecDiff = specDiff

	// update phase
	switch {
	case reconcileErr != nil:
		dryRunResult.Phase = viewv1.DryRunFailedPhase
		dryRunResult.Reason = "ReconcileError"
		dryRunResult.Message = reconcileErr.Error()
	case timeout:
		dryRunResult.Phase = viewv1.DryRunFailedPhase
		dryRunResult.Reason = "Timeout"
		dryRunResult.Message = "Can't generate the plan within one second"
	default:
		dryRunResult.Phase = viewv1.DryRunSucceedPhase
	}

	// update plan
	desiredTree, err := getObjectTreeFromCache(g.ctx, mClient, root, getKBOwnershipRules())
	if err != nil {
		return nil, err
	}
	dryRunResult.Plan.ObjectTree = desiredTree
	dryRunResult.Plan.Changes = store.GetChanges()
	newObjectMap := store.GetAll()
	dryRunResult.Plan.Summary.ObjectSummaries = buildObjectSummaries(initialObjectMap, newObjectMap)

	return dryRunResult, nil
}

func newPlanGenerator(ctx context.Context, cli client.Client, scheme *runtime.Scheme, loader objectLoader, formatter descriptionFormatter) PlanGenerator {
	return &planGenerator{
		ctx:       ctx,
		cli:       cli,
		scheme:    scheme,
		loader:    loader,
		formatter: formatter,
	}
}

func applyPatch(obj client.Object, patch client.Patch) (string, error) {
	// Extract the current spec and apply the patch
	currentSpec, err := getSpecFieldAsStruct(obj)
	if err != nil {
		return "", err
	}

	patchData, err := patch.Data(obj)
	if err != nil {
		return "", err
	}
	// Apply the patch to the current spec
	var modifiedSpec []byte
	if patch.Type() == types.StrategicMergePatchType {
		modifiedSpec, err = strategicpatch.StrategicMergePatch(
			specMapToJSON(currentSpec),
			patchData,
			currentSpec,
		)
		if err != nil {
			return "", err
		}
	} else if patch.Type() == types.MergePatchType {
		modifiedSpec, err = jsonpatch.MergePatch(specMapToJSON(currentSpec), patchData)
		if err != nil {
			return "", err
		}
	}
	modifiedSpecMap := make(map[string]interface{})
	if err = json.Unmarshal(modifiedSpec, &modifiedSpecMap); err != nil {
		return "", err
	}

	// Convert the object to an unstructured map
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return "", err
	}

	// Extract the initial spec
	initialSpec, _, err := unstructured.NestedMap(objMap, "spec")
	if err != nil {
		return "", err
	}

	// Update the spec in the object map
	if err := unstructured.SetNestedField(objMap, modifiedSpecMap, "spec"); err != nil {
		return "", err
	}

	// Convert the modified map back to the original object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(objMap, obj); err != nil {
		return "", err
	}

	// build the spec change
	specChange := cmp.Diff(initialSpec, modifiedSpecMap)

	return specChange, nil
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

func loadCurrentObjectTree(loader objectLoader, store ChangeCaptureStore) error {
	objectMap, err := loader()
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

var _ PlanGenerator = &planGenerator{}
