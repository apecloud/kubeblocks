package kubebuilderx

import (
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectTree2DAGTransformer struct {
	current *ObjectTree
	desired *ObjectTree
}

func (t *objectTree2DAGTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// init context
	transCtx, _ := ctx.(*transformContext)
	cli, _ := transCtx.cli.(model.GraphClient)
	// init dag
	cli.Root(dag, t.current.Root, t.desired.Root, model.ActionStatusPtr())

	oldSnapshot := t.current.Children
	newSnapshot := t.desired.Children

	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newSnapshot)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)

	createNewObjects := func() {
		for name := range createSet {
			cli.Create(dag, newSnapshot[name])
		}
	}
	updateObjects := func() {
		for name := range updateSet {
			oldObj := oldSnapshot[name]
			newObj := newSnapshot[name]
			cli.Update(dag, oldObj, newObj)
		}
	}
	deleteOrphanObjects := func() {
		for name := range deleteSet {
			cli.Delete(dag, oldSnapshot[name])
		}
	}
	handleDependencies := func() error {
		svcList := cli.FindAll(dag, &corev1.Service{})
		cmList := cli.FindAll(dag, &corev1.ConfigMap{})
		secretList := cli.FindAll(dag, &corev1.Secret{})
		pvcList := cli.FindAll(dag, &corev1.PersistentVolumeClaim{})
		allObjects := cli.FindAll(dag, nil, &model.HaveDifferentTypeWithOption{})
		dependencyMap := make(model.ObjectSnapshot, len(svcList)+len(cmList)+len(secretList)+len(pvcList))
		buildDependencyMap := func(objects []client.Object) error {
			for _, object := range objects {
				name, err := model.GetGVKName(object)
				if err != nil {
					return err
				}
				dependencyMap[*name] = object
			}
			return nil
		}
		if err := buildDependencyMap(svcList); err != nil {
			return err
		}
		if err := buildDependencyMap(cmList); err != nil {
			return err
		}
		if err := buildDependencyMap(secretList); err != nil {
			return err
		}
		if err := buildDependencyMap(pvcList); err != nil {
			return err
		}
		for _, workload := range allObjects {
			name, err := model.GetGVKName(workload)
			if err != nil {
				return err
			}
			if _, ok := dependencyMap[*name]; ok {
				continue
			}
			for _, dependency := range dependencyMap {
				cli.DependOn(dag, workload, dependency)
			}
		}
		return nil
	}

	// objects to be created
	createNewObjects()
	// objects to be updated
	updateObjects()
	// objects to be deleted
	deleteOrphanObjects()
	// handle object dependencies
	return handleDependencies()
}

func newObjectTree2DAGTransformer(currentTree, desiredTree *ObjectTree) graph.Transformer {
	return &objectTree2DAGTransformer{
		current: currentTree,
		desired: desiredTree,
	}
}

var _ graph.Transformer = &objectTree2DAGTransformer{}
