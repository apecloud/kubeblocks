package lifecycle

import (
	"context"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type shardAdd struct {
	namespace   string
	clusterName string
	compName    string
	action      *appsv1.Action
}

var _ lifecycleAction = &shardAdd{}

func (a *shardAdd) name() string {
	return "shardAdd"
}

func (a *shardAdd) parameters(context.Context, client.Reader) (map[string]string, error) {
	return nil, nil
}

type shardRemove struct {
	namespace   string
	clusterName string
	compName    string
	action      *appsv1.Action
}

var _ lifecycleAction = &shardRemove{}

func (a *shardRemove) name() string {
	return "shardRemove"
}

func (a *shardRemove) parameters(context.Context, client.Reader) (map[string]string, error) {
	return nil, nil
}
