package lifecycle

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// fixClusterLabelsTransformer should patch the label first to prevent the label from being modified by the user.
type fixClusterLabelsTransformer struct {
	cc  compoundCluster
	cli client.Client
	ctx intctrlutil.RequestCtx
}

func (f *fixClusterLabelsTransformer) Transform(dag *graph.DAG) error {
	labels := f.cc.cluster.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	cdLabelName := labels[clusterDefLabelKey]
	cvLabelName := labels[clusterVersionLabelKey]
	cdName, cvName := f.cc.cluster.Spec.ClusterDefRef, f.cc.cluster.Spec.ClusterVersionRef
	if cdLabelName == cdName && cvLabelName == cvName {
		return nil
	}
	labels[clusterDefLabelKey] = cdName
	labels[clusterVersionLabelKey] = cvName
	f.cc.cluster.Labels = labels
	return nil
}
