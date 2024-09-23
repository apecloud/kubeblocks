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
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type stateEvaluation struct {
	ctx    context.Context
	reader client.Reader
	store  ObjectStore
	scheme *runtime.Scheme
}

func (s *stateEvaluation) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (s *stateEvaluation) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	view, _ := tree.GetRoot().(*viewv1.ReconciliationView)
	viewDef, _ := tree.List(&viewv1.ReconciliationViewDefinition{})[0].(*viewv1.ReconciliationViewDefinition)

	// build new object set from cache
	root := &appsv1alpha1.Cluster{}
	objectKey := client.ObjectKeyFromObject(view)
	if view.Spec.TargetObject != nil {
		objectKey = client.ObjectKey{
			Namespace: view.Spec.TargetObject.Namespace,
			Name:      view.Spec.TargetObject.Name,
		}
	}
	if err := s.reader.Get(s.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}

	// keep only the latest reconciliation cycle.
	// the basic idea is:
	// 1. traverse the view progress list from tail to head,
	// 2. read the corresponding version of the Cluster object from object store by objectChange.resourceVersion,
	// 3. evaluation its state,
	// if we find the first-false-then-true pattern, means a new reconciliation cycle starts.
	firstFalseStateFound := false
	clusterType := viewv1.ObjectType{
		APIVersion: appsv1alpha1.APIVersion,
		Kind:       appsv1alpha1.ClusterKind,
	}
	latestReconciliationCycleStart := 0
	for i := len(view.Status.View) - 1; i >= 0; i-- {
		change := view.Status.View[i]
		objType := objectRefToType(&change.ObjectReference)
		if *objType != clusterType {
			continue
		}
		objectRef, err := objectReferenceToRef(&change.ObjectReference)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		obj, err := s.store.Get(objectRef, change.Revision)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		if obj == nil {
			return kubebuilderx.Commit, fmt.Errorf("object %s not found", change.ObjectReference)
		}
		expr := viewDef.Spec.StateEvaluationExpression
		if view.Spec.StateEvaluationExpression != nil {
			expr = *view.Spec.StateEvaluationExpression
		}
		state, err := doStateEvaluation(obj, expr)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		if !state && !firstFalseStateFound {
			firstFalseStateFound = true
		}
		if state && firstFalseStateFound {
			latestReconciliationCycleStart = i
			break
		}
	}
	if latestReconciliationCycleStart <= 0 {
		if view.Status.InitialObjectTree == nil {
			reference, err := getObjectReference(root, s.scheme)
			if err != nil {
				return kubebuilderx.Commit, err
			}
			view.Status.InitialObjectTree = &viewv1.ObjectTreeNode{
				Primary: *reference,
			}
		}
		return kubebuilderx.Continue, nil
	}

	// build new InitialObjectTree
	var err error
	view.Status.InitialObjectTree, err = getObjectTreeWithRevision(root, viewDef.Spec.OwnershipRules, s.store, view.Status.View[latestReconciliationCycleStart].Revision, s.scheme)
	if err != nil {
		return kubebuilderx.Commit, err
	}

	// delete unused object revisions
	for i := 0; i < latestReconciliationCycleStart; i++ {
		change := view.Status.View[i]
		objectRef, err := objectReferenceToRef(&change.ObjectReference)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		s.store.Delete(objectRef, view, change.Revision)
	}

	// truncate view
	view.Status.View = view.Status.View[latestReconciliationCycleStart:]

	return kubebuilderx.Continue, nil
}

// TODO(free6om): similar as getSecondaryObjectsOf, refactor and merge them
func getObjectTreeWithRevision(primary client.Object, ownershipRules []viewv1.OwnershipRule, store ObjectStore, revision int64, scheme *runtime.Scheme) (*viewv1.ObjectTreeNode, error) {
	// find matched rules
	var matchedRules []*viewv1.OwnershipRule
	for i := range ownershipRules {
		rule := &ownershipRules[i]
		gvk, err := objectTypeToGVK(&rule.Primary)
		if err != nil {
			return nil, err
		}
		primaryGVK, err := apiutil.GVKForObject(primary, scheme)
		if err != nil {
			return nil, err
		}
		if *gvk == primaryGVK {
			matchedRules = append(matchedRules, rule)
		}
	}

	reference, err := getObjectReference(primary, scheme)
	if err != nil {
		return nil, err
	}
	tree := &viewv1.ObjectTreeNode{
		Primary: *reference,
	}
	// traverse rules, build subtree
	var secondaries []client.Object
	for _, rule := range matchedRules {
		for _, ownedResource := range rule.OwnedResources {
			gvk, err := objectTypeToGVK(&ownedResource.Secondary)
			if err != nil {
				return nil, err
			}
			ml, err := parseMatchingLabels(primary, &ownedResource.Criteria)
			if err != nil {
				return nil, err
			}
			objects, err := getObjectsByRevision(gvk, store, ml, revision)
			if err != nil {
				return nil, err
			}
			secondaries = append(secondaries, objects...)
		}
	}
	for _, secondary := range secondaries {
		subTree, err := getObjectTreeWithRevision(secondary, ownershipRules, store, revision, scheme)
		if err != nil {
			return nil, err
		}
		tree.Secondaries = append(tree.Secondaries, subTree)
	}

	return tree, nil
}

func getObjectsByRevision(gvk *schema.GroupVersionKind, store ObjectStore, ml client.MatchingLabels, revision int64) ([]client.Object, error) {
	objectMap := store.List(gvk)

	var matchedObjects []client.Object
	opts := &client.ListOptions{}
	ml.ApplyToList(opts)
	for _, revisionMap := range objectMap {
		rev := int64(-1)
		for r, object := range revisionMap {
			if !opts.LabelSelector.Matches(labels.Set(object.GetLabels())) {
				continue
			}
			if rev < r && r <= revision {
				rev = r
			}
		}
		if rev > -1 {
			matchedObjects = append(matchedObjects, revisionMap[rev])
		}
	}
	return matchedObjects, nil
}

func getObjectRef(object client.Object, scheme *runtime.Scheme) (*model.GVKNObjKey, error) {
	gvk, err := apiutil.GVKForObject(object, scheme)
	if err != nil {
		return nil, err
	}
	return &model.GVKNObjKey{
		GroupVersionKind: gvk,
		ObjectKey:        client.ObjectKeyFromObject(object),
	}, nil
}

func doStateEvaluation(object client.Object, expression viewv1.StateEvaluationExpression) (bool, error) {
	if expression.CELExpression == nil {
		return false, fmt.Errorf("CEL expression can't be empty")
	}

	// Convert the object to an unstructured map
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return false, fmt.Errorf("failed to convert object to unstructured: %w", err)
	}

	// Create a CEL environment with the object fields available
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("object", decls.NewMapType(decls.String, decls.Dyn)),
		),
	)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Compile the CEL expression
	ast, issues := env.Compile(expression.CELExpression.Expression)
	if issues.Err() != nil {
		return false, fmt.Errorf("failed to compile expression: %w", issues.Err())
	}

	// Create a program
	prg, err := env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL program: %w", err)
	}

	// Evaluate the expression with the object map
	objValue, err := structpb.NewStruct(unstructuredMap)
	if err != nil {
		return false, fmt.Errorf("failed to create structpb value: %w", err)
	}

	out, _, err := prg.Eval(map[string]ref.Val{
		"object": types.NewDynamicMap(types.DefaultTypeAdapter, objValue),
	})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	// Convert the output to a boolean
	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression did not return a boolean")
	}

	return result, nil
}

func viewStateEvaluation(ctx context.Context, reader client.Reader, store ObjectStore, scheme *runtime.Scheme) kubebuilderx.Reconciler {
	return &stateEvaluation{
		ctx:    ctx,
		reader: reader,
		store:  store,
		scheme: scheme,
	}
}

var _ kubebuilderx.Reconciler = &stateEvaluation{}
