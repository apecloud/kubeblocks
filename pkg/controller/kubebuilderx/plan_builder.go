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

package kubebuilderx

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type transformContext struct {
	ctx      context.Context
	cli      client.Reader
	recorder record.EventRecorder
	logger   logr.Logger
}

type PlanBuilder struct {
	transCtx    *transformContext
	cli         client.Client
	currentTree *ObjectTree
	desiredTree *ObjectTree
}

type Plan struct {
	vertices []*model.ObjectVertex
	walkFunc graph.WalkFunc
}

var _ graph.TransformContext = &transformContext{}
var _ graph.PlanBuilder = &PlanBuilder{}
var _ graph.Plan = &Plan{}

func init() {
	model.AddScheme(workloads.AddToScheme)
}

func (t *transformContext) GetContext() context.Context {
	return t.ctx
}

func (t *transformContext) GetClient() client.Reader {
	return t.cli
}

func (t *transformContext) GetRecorder() record.EventRecorder {
	return t.recorder
}

func (t *transformContext) GetLogger() logr.Logger {
	return t.logger
}

// PlanBuilder implementation

func (b *PlanBuilder) Init() error {
	return nil
}

func (b *PlanBuilder) AddTransformer(_ ...graph.Transformer) graph.PlanBuilder {
	return b
}

func (b *PlanBuilder) Build() (graph.Plan, error) {
	vertices := buildOrderedVertices(b.transCtx.GetContext(), b.currentTree, b.desiredTree)
	plan := &Plan{
		walkFunc: b.defaultWalkFunc,
		vertices: vertices,
	}
	return plan, nil
}

func buildOrderedVertices(ctx context.Context, currentTree *ObjectTree, desiredTree *ObjectTree) []*model.ObjectVertex {
	getStatusField := func(obj client.Object) interface{} {
		objValue := reflect.ValueOf(obj)
		if objValue.Kind() != reflect.Ptr || objValue.Elem().Kind() != reflect.Struct {
			return nil
		}
		field := objValue.Elem().FieldByName("Status")
		if !field.IsValid() {
			return nil
		}
		return field.Interface()
	}

	var vertices []*model.ObjectVertex

	// handle root object
	if desiredTree.GetRoot() == nil {
		root := model.NewObjectVertex(currentTree.GetRoot(), currentTree.GetRoot(), model.ActionDeletePtr())
		vertices = append(vertices, root)
	} else {
		currentStatus := getStatusField(currentTree.GetRoot())
		desiredStatus := getStatusField(desiredTree.GetRoot())
		if !reflect.DeepEqual(currentStatus, desiredStatus) {
			root := model.NewObjectVertex(currentTree.GetRoot(), desiredTree.GetRoot(), model.ActionStatusPtr())
			vertices = append(vertices, root)
		}
		// if annotations, labels or finalizers updated, do both meta patch and status update.
		if !reflect.DeepEqual(currentTree.GetRoot().GetAnnotations(), desiredTree.GetRoot().GetAnnotations()) ||
			!reflect.DeepEqual(currentTree.GetRoot().GetLabels(), desiredTree.GetRoot().GetLabels()) ||
			!reflect.DeepEqual(currentTree.GetRoot().GetFinalizers(), desiredTree.GetRoot().GetFinalizers()) {
			currentRoot, _ := currentTree.GetRoot().DeepCopyObject().(client.Object)
			desiredRoot, _ := desiredTree.GetRoot().DeepCopyObject().(client.Object)
			patchRoot := model.NewObjectVertex(currentRoot, desiredRoot, model.ActionPatchPtr())
			vertices = append(vertices, patchRoot)
		}
	}

	// handle secondary objects
	oldSnapshot := currentTree.GetSecondaryObjects()
	newSnapshot := desiredTree.GetSecondaryObjects()

	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newSnapshot)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)

	var (
		assistantVertices []*model.ObjectVertex
		workloadVertices  []*model.ObjectVertex
	)
	findAndAppend := func(vertex *model.ObjectVertex) {
		switch vertex.Obj.(type) {
		case *corev1.Service, *corev1.ConfigMap, *corev1.Secret, *corev1.PersistentVolumeClaim:
			assistantVertices = append(assistantVertices, vertex)
		default:
			workloadVertices = append(workloadVertices, vertex)
		}
	}
	createNewObjects := func() {
		for name := range createSet {
			v := model.NewObjectVertex(nil, assign(ctx, newSnapshot[name]), model.ActionCreatePtr(), inDataContext4G())
			findAndAppend(v)
		}
	}
	updateObjects := func() {
		for name := range updateSet {
			oldObj := oldSnapshot[name]
			newObj := newSnapshot[name]
			if !reflect.DeepEqual(oldObj, newObj) {
				v := model.NewObjectVertex(oldObj, newObj, model.ActionUpdatePtr(), inDataContext4G())
				findAndAppend(v)
			}
		}
	}
	deleteOrphanObjects := func() {
		for name := range deleteSet {
			object := oldSnapshot[name]
			v := model.NewObjectVertex(nil, object, model.ActionDeletePtr(), inDataContext4G())
			findAndAppend(v)
		}
	}
	handleDependencies := func() {
		vertices = append(vertices, workloadVertices...)
		vertices = append(vertices, assistantVertices...)
	}

	// objects to be created
	createNewObjects()
	// objects to be updated
	updateObjects()
	// objects to be deleted
	deleteOrphanObjects()
	// handle object dependencies
	handleDependencies()
	return vertices
}

// Plan implementation

func (p *Plan) Execute() error {
	var err error
	for i := len(p.vertices) - 1; i >= 0; i-- {
		if err = p.walkFunc(p.vertices[i]); err != nil {
			return err
		}
	}
	return nil
}

// Do the real works

func (b *PlanBuilder) defaultWalkFunc(v graph.Vertex) error {
	vertex, ok := v.(*model.ObjectVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", v)
	}
	if vertex.Action == nil {
		return errors.New("vertex action can't be nil")
	}
	ctx := b.transCtx.ctx
	switch *vertex.Action {
	case model.CREATE:
		return b.createObject(ctx, vertex)
	case model.UPDATE:
		return b.updateObject(ctx, vertex)
	case model.PATCH:
		return b.patchObject(ctx, vertex)
	case model.DELETE:
		return b.deleteObject(ctx, vertex)
	case model.STATUS:
		return b.statusObject(ctx, vertex)
	}
	return nil
}

func (b *PlanBuilder) createObject(ctx context.Context, vertex *model.ObjectVertex) error {
	err := b.cli.Create(ctx, vertex.Obj, clientOption(vertex))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	b.emitEvent(vertex.Obj, "SuccessfulCreate", model.CREATE)
	return nil
}

func (b *PlanBuilder) updateObject(ctx context.Context, vertex *model.ObjectVertex) error {
	err := b.cli.Update(ctx, vertex.Obj, clientOption(vertex))
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	b.emitEvent(vertex.Obj, "SuccessfulUpdate", model.UPDATE)
	return nil
}

func (b *PlanBuilder) patchObject(ctx context.Context, vertex *model.ObjectVertex) error {
	patch := client.MergeFrom(vertex.OriObj)
	err := b.cli.Patch(ctx, vertex.Obj, patch, clientOption(vertex))
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	b.emitEvent(vertex.Obj, "SuccessfulUpdate", model.UPDATE)
	return nil
}

func (b *PlanBuilder) deleteObject(ctx context.Context, vertex *model.ObjectVertex) error {
	var finalizer string
	if b.currentTree != nil {
		finalizer = b.currentTree.GetFinalizer()
	}
	if len(finalizer) > 0 && controllerutil.RemoveFinalizer(vertex.Obj, finalizer) {
		err := b.cli.Update(ctx, vertex.Obj, clientOption(vertex))
		if err != nil && !apierrors.IsNotFound(err) {
			b.transCtx.logger.Error(err, fmt.Sprintf("delete %T error: %s", vertex.Obj, vertex.Obj.GetName()))
			return err
		}
	}
	if !model.IsObjectDeleting(vertex.Obj) {
		err := b.cli.Delete(ctx, vertex.Obj, clientOption(vertex))
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		b.emitEvent(vertex.Obj, "SuccessfulDelete", model.DELETE)
	}
	return nil
}

func (b *PlanBuilder) statusObject(ctx context.Context, vertex *model.ObjectVertex) error {
	if err := b.cli.Status().Update(ctx, vertex.Obj, clientOption(vertex)); err != nil {
		return err
	}
	return nil
}

func (b *PlanBuilder) emitEvent(obj client.Object, reason string, action model.Action) {
	if b.currentTree == nil {
		return
	}
	root := b.currentTree.GetRoot()
	b.currentTree.EventRecorder.Eventf(root, corev1.EventTypeNormal, reason,
		"%s %s %s in %s %s successful",
		strings.ToLower(string(action)), getTypeName(obj), obj.GetName(), getTypeName(root), root.GetName())
}

func getTypeName(i any) string {
	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

// NewPlanBuilder returns a PlanBuilder
func NewPlanBuilder(ctx context.Context, cli client.Client, currentTree, desiredTree *ObjectTree, recorder record.EventRecorder, logger logr.Logger) graph.PlanBuilder {
	return &PlanBuilder{
		transCtx: &transformContext{
			ctx:      ctx,
			cli:      model.NewGraphClient(cli),
			recorder: recorder,
			logger:   logger,
		},
		cli:         cli,
		currentTree: currentTree,
		desiredTree: desiredTree,
	}
}
