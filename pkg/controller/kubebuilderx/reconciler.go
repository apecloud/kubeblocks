package kubebuilderx

import (
	"context"
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectTree struct {
	Root     client.Object
	Children []client.Object
}

type TreeReader interface {
	Read(context.Context, client.Reader, ctrl.Request) (*ObjectTree, error)
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

func (t *ObjectTree) DeepCopy() (*ObjectTree, error) {
	out := new(ObjectTree)
	root, ok := t.Root.DeepCopyObject().(client.Object)
	if !ok {
		return nil, ErrDeepCopyFailed
	}
	out.Root = root
	var children []client.Object
	for _, child := range t.Children {
		childCopied, ok := child.DeepCopyObject().(client.Object)
		if !ok {
			return nil, ErrDeepCopyFailed
		}
		children = append(children, childCopied)
	}
	out.Children = children
	return out, nil
}
