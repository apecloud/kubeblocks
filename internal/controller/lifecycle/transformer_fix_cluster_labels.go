package lifecycle

import (
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// FixClusterLabelsTransformer patches the label first to prevent the label from being modified by the user.
type FixClusterLabelsTransformer struct{}

func (f *FixClusterLabelsTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	labels := cluster.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	cdLabelName := labels[clusterDefLabelKey]
	cvLabelName := labels[clusterVersionLabelKey]
	cdName, cvName := cluster.Spec.ClusterDefRef, cluster.Spec.ClusterVersionRef
	if cdLabelName == cdName && cvLabelName == cvName {
		return nil
	}
	labels[clusterDefLabelKey] = cdName
	labels[clusterVersionLabelKey] = cvName
	cluster.Labels = labels
	return nil
}

var _ graph.Transformer = &FixClusterLabelsTransformer{}
