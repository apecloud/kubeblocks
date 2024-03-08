package rsm2

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type treeReader struct {}

func (r *treeReader) Read(ctx context.Context, reader client.Reader, req ctrl.Request) (*kubebuilderx.ObjectTree, error) {
	keys := getMatchLabelKeys()
	kinds := ownedKinds()
	return kubebuilderx.ReadObjectTree[*workloads.ReplicatedStateMachine](ctx, reader, req, keys, kinds...)
}

func getMatchLabelKeys() []string {
	if viper.GetBool(rsm.FeatureGateRSMCompatibilityMode) {
		return []string{
			constant.AppManagedByLabelKey,
			constant.AppNameLabelKey,
			constant.AppComponentLabelKey,
			constant.AppInstanceLabelKey,
			constant.KBAppComponentLabelKey,
		}
	}
	return []string{
		rsm.WorkloadsManagedByLabelKey,
		rsm.WorkloadsInstanceLabelKey,
	}
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&corev1.ServiceList{},
		&corev1.ConfigMapList{},
		&corev1.PodList{},
		&corev1.PersistentVolumeClaimList{},
		&batchv1.JobList{},
	}
}

func NewTreeReader() kubebuilderx.TreeReader {
	return &treeReader{}
}

var _ kubebuilderx.TreeReader = &treeReader{}
