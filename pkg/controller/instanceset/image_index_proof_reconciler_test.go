/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instanceset

import (
	"context"
	"encoding/base64"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	digest "github.com/opencontainers/go-digest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const instancesetProofChild = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type instancesetProofReader struct {
	node *corev1.Node
	err  error
}

func (r *instancesetProofReader) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if r.err != nil {
		return r.err
	}
	r.node.DeepCopyInto(obj.(*corev1.Node))
	return nil
}

func (r *instancesetProofReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errors.New("unexpected List")
}

func instancesetProofConfig() (string, map[string]any) {
	raw := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"` + instancesetProofChild + `","size":100,"platform":{"architecture":"amd64","os":"linux"}}]}`)
	parent := digest.FromBytes(raw).String()
	return parent, map[string]any{
		"imageIndexProofs": []map[string]any{{
			"indexDigest": parent,
			"indexBase64": base64.StdEncoding.EncodeToString(raw),
		}},
	}
}

func instancesetProofPod(parent string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Containers: []corev1.Container{{
				Name:  "kbagent",
				Image: "example/tools@" + instancesetProofChild,
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "kbagent",
				Image:   "example/tools:tag",
				ImageID: "example/tools@" + parent,
			}},
		},
	}
}

var _ = Describe("image index proof reconciler propagation", Serial, func() {
	var oldRegistries any
	var parent string

	BeforeEach(func() {
		oldRegistries = viper.Get(constant.CfgRegistries)
		var config map[string]any
		parent, config = instancesetProofConfig()
		viper.Set(constant.CfgRegistries, config)
		Expect(intctrlutil.LoadRegistryConfig()).To(Succeed())
	})

	AfterEach(func() {
		viper.Set(constant.CfgRegistries, oldRegistries)
		Expect(intctrlutil.LoadRegistryConfig()).To(Succeed())
	})

	It("returns a transient Node error from the status reconciler", func() {
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}
		pod := instancesetProofPod(parent)
		tree := kubebuilderx.NewObjectTree()
		tree.Context = context.Background()
		tree.SetRoot(its)
		Expect(tree.Add(pod)).To(Succeed())

		_, err := NewStatusReconciler(&instancesetProofReader{err: errors.New("temporary Node read")}).Reconcile(tree)
		Expect(err).To(MatchError(ContainSubstring("temporary Node read")))
	})

	It("returns a transient Node error from the update gate", func() {
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}
		pod := instancesetProofPod(parent)
		tree := kubebuilderx.NewObjectTree()
		tree.Context = context.Background()
		tree.SetRoot(its)

		matched, retry, err := (&updateReconciler{reader: &instancesetProofReader{
			err: errors.New("temporary Node read"),
		}}).isPodCanBeUpdated(tree, its, pod)
		Expect(matched).To(BeFalse())
		Expect(retry).To(BeFalse())
		Expect(err).To(MatchError(ContainSubstring("temporary Node read")))
	})

	It("allows the update gate only after the exact Node platform match", func() {
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}
		pod := instancesetProofPod(parent)
		tree := kubebuilderx.NewObjectTree()
		tree.Context = context.Background()
		tree.SetRoot(its)
		reader := &instancesetProofReader{node: &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
			Status: corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: "linux",
				Architecture:    "amd64",
			}},
		}}

		matched, retry, err := (&updateReconciler{reader: reader}).isPodCanBeUpdated(tree, its, pod)
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
		Expect(retry).To(BeFalse())
	})
})
