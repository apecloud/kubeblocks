package instancetemplate

import (
	"errors"
	"reflect"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// NewPodNameBuilder returns a PodNameBuilder based on the InstanceSet's PodNamingRule.
// When the PodNamingRule is Combined, it should be a instanceset returned by kubernetes (i.e. with status field included)
func NewPodNameBuilder(itsExt *InstanceSetExt) (PodNameBuilder, error) {
	switch itsExt.InstanceSet.Spec.PodNamingRule {
	case workloads.PodNamingRuleCombined:
		// validate status is not empty
		if reflect.ValueOf(itsExt.InstanceSet.Status).IsZero() {
			return nil, errors.New("instanceset status is empty")
		}
		return &combinedPodNameBuilder{
			itsExt: itsExt,
		}, nil
	// default to seperated naming rule, since it's the old behavior
	default:
		return &seperatedPodNameBuilder{
			itsExt: itsExt,
		}, nil
	}
}
