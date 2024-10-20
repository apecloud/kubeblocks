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

package trace

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type PlanGenerator interface {
	generatePlan(desiredRoot *kbappsv1.Cluster) (*tracev1.DryRunResult, error)
}

type objectLoader func() (map[model.GVKNObjKey]client.Object, error)
type descriptionFormatter func(client.Object, client.Object, tracev1.ObjectChangeType, *schema.GroupVersionKind) (string, *string)

type planGenerator struct {
	ctx       context.Context
	cli       client.Client
	scheme    *runtime.Scheme
	loader    objectLoader
	formatter descriptionFormatter
}

func (g *planGenerator) generatePlan(desiredRoot *kbappsv1.Cluster) (*tracev1.DryRunResult, error) {
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

	// get current root
	currentRoot := &kbappsv1.Cluster{}
	if err = mClient.Get(g.ctx, client.ObjectKeyFromObject(desiredRoot), currentRoot); err != nil {
		return nil, err
	}
	// build spec diff
	var specDiff string
	if specDiff, err = buildSpecDiff(currentRoot, desiredRoot); err != nil {
		return nil, err
	}
	if err = mClient.Update(g.ctx, desiredRoot); err != nil {
		return nil, err
	}

	// generate plan with timeout
	startTime := time.Now()
	timeout := false
	var reconcileErr error
	previousCount := len(store.GetChanges())
	for {
		if time.Since(startTime) > 1000*time.Second {
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
	dryRunResult := &tracev1.DryRunResult{}
	dryRunResult.ObservedTargetGeneration = desiredRoot.Generation
	dryRunResult.SpecDiff = specDiff

	// update phase
	switch {
	case reconcileErr != nil:
		dryRunResult.Phase = tracev1.DryRunFailedPhase
		dryRunResult.Reason = "ReconcileError"
		dryRunResult.Message = reconcileErr.Error()
	case timeout:
		dryRunResult.Phase = tracev1.DryRunFailedPhase
		dryRunResult.Reason = "Timeout"
		dryRunResult.Message = "Can't generate the plan within one second"
	default:
		dryRunResult.Phase = tracev1.DryRunSucceedPhase
	}

	// update plan
	desiredTree, err := getObjectTreeFromCache(g.ctx, mClient, desiredRoot, getKBOwnershipRules())
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

func buildSpecDiff(current, desired client.Object) (string, error) {
	// Extract the current spec
	currentSpec, err := getFieldAsStruct(current, specFieldName)
	if err != nil {
		return "", err
	}
	desiredSpec, err := getFieldAsStruct(desired, specFieldName)
	if err != nil {
		return "", err
	}
	// build the spec change
	specChange := cmp.Diff(currentSpec, desiredSpec)
	return specChange, nil
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
