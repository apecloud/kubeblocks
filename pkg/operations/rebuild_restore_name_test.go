package operations

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestBuildRestoreMetaObjectShortensName(t *testing.T) {
	helper := &inplaceRebuildHelper{}
	opsRequest := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rebuild-ops",
			Namespace: "default",
		},
	}

	meta := helper.buildRestoreMetaObject(opsRequest, strings.Repeat("rebuild-restore-", 5))
	if len(meta.Name) > constant.KubeNameMaxLength {
		t.Fatalf("restore name length = %d, want <= %d: %s", len(meta.Name), constant.KubeNameMaxLength, meta.Name)
	}
}
