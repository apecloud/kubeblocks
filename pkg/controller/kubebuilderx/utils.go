package kubebuilderx

import (
	"context"
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReadObjectTree reads all objects owned by the root object which is type of 'T' with key in 'req'.
func ReadObjectTree[T client.Object](ctx context.Context, reader client.Reader, req ctrl.Request, labelKeys []string, kinds ...client.ObjectList) (*ObjectTree, error) {
	root := *new(T)
	if err := reader.Get(ctx, req.NamespacedName, root); err != nil {
		return nil, err
	}

	var children []client.Object
	ml := getMatchLabels(root, labelKeys)
	inNS := client.InNamespace(req.Namespace)
	for _, list := range kinds {
		if err := reader.List(ctx, list, inNS, ml); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			children = append(children, object)
		}
	}

	tree := &ObjectTree{
		Root:     root,
		Children: children,
	}

	return tree, nil
}

func getMatchLabels(root client.Object, labelKeys []string) client.MatchingLabels {
	labels := make(map[string]string, len(labelKeys))
	for _, key := range labelKeys {
		labels[key] = root.GetLabels()[key]
	}
	return labels
}
