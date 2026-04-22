package plan

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

func TestGetRestoreObjectMetaShortensName(t *testing.T) {
	manager := &RestoreManager{
		Cluster: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.Repeat("restore-cluster-", 4),
				Namespace: "default",
				UID:       types.UID("12345678-1234-1234-1234-1234567890ab"),
			},
		},
	}
	synthesized := &component.SynthesizedComponent{
		Name: strings.Repeat("component-", 4),
	}

	meta := manager.GetRestoreObjectMeta(synthesized, dpv1alpha1.PrepareData, "template")
	if len(meta.Name) > constant.KubeNameMaxLength {
		t.Fatalf("restore name length = %d, want <= %d: %s", len(meta.Name), constant.KubeNameMaxLength, meta.Name)
	}
}
