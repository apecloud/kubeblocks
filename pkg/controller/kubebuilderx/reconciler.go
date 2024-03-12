package kubebuilderx

import (
	"context"
	"errors"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectTree struct {
	// TODO(free6om): should find a better place to hold these two params?
	record.EventRecorder
	logr.Logger

	Root     client.Object
	Children model.ObjectSnapshot
}

type TreeReader interface {
	Read(context.Context, client.Reader, ctrl.Request, record.EventRecorder, logr.Logger) (*ObjectTree, error)
}

type CheckResult struct {
	Satisfied bool
	Err       error
}

var ResultUnsatisfied = &CheckResult{}
var ResultSatisfied = &CheckResult{Satisfied: true}

var ErrDeepCopyFailed = errors.New("DeepCopyFailed")

type Reconciler interface {
	PreCondition(*ObjectTree) *CheckResult
	Reconcile(tree *ObjectTree) (*ObjectTree, error)
}

func CheckResultWithError(err error) *CheckResult {
	return &CheckResult{Satisfied: false, Err: err}
}

func (t *ObjectTree) DeepCopy() (*ObjectTree, error) {
	out := new(ObjectTree)
	root, ok := t.Root.DeepCopyObject().(client.Object)
	if !ok {
		return nil, ErrDeepCopyFailed
	}
	out.Root = root
	var children model.ObjectSnapshot
	for key, child := range t.Children {
		childCopied, ok := child.DeepCopyObject().(client.Object)
		if !ok {
			return nil, ErrDeepCopyFailed
		}
		children[key] = childCopied
	}
	out.Children = children
	return out, nil
}

func (t *ObjectTree) List(obj client.Object) []client.Object {
	assignableTo := func(src, dst reflect.Type) bool {
		if dst == nil {
			return src == nil
		}
		return src.AssignableTo(dst)
	}
	objType := reflect.TypeOf(obj)
	objects := make([]client.Object, 0)
	for _, child := range t.Children {
		vertexType := reflect.TypeOf(child)
		if assignableTo(vertexType, objType) {
			objects = append(objects, child)
		}
	}
	return objects
}

func (t *ObjectTree) Add(objects ...client.Object) error {
	return t.replace(objects...)
}

func (t *ObjectTree) Delete(objects ...client.Object) error {
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return err
		}
		delete(t.Children, *name)
	}
	return nil
}

func (t *ObjectTree) Update(objects ...client.Object) error {
	return t.replace(objects...)
}

func (t *ObjectTree) replace(objects ...client.Object) error {
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return err
		}
		t.Children[*name] = object
	}
	return nil
}
