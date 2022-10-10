package dataprotection

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func checkResourceExists(
	ctx context.Context,
	client client.Client,
	key client.ObjectKey,
	obj client.Object) (bool, error) {

	if err := client.Get(ctx, key, obj); err != nil {
		// if err is NOT "not found", that means unknown error.
		if !strings.Contains(err.Error(), "not found") {
			return false, err
		}
		// if not found, return false
		return false, nil
	}
	// if found, return true
	return true, nil
}
