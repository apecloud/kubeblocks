package rsm2

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func getFinalizer(obj client.Object) string {
	if _, ok := obj.(*workloads.ReplicatedStateMachine); ok {
		return rsm.RSMFinalizerName
	}
	if viper.GetBool(rsm.FeatureGateRSMCompatibilityMode) {
		return constant.DBClusterFinalizerName
	}
	return rsm.RSMFinalizerName
}
