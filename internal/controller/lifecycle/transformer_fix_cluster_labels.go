package lifecycle

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// fixClusterLabelsTransformer should patch the label first to prevent the label from being modified by the user.
type fixClusterLabelsTransformer struct{}

func (f *fixClusterLabelsTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)
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
