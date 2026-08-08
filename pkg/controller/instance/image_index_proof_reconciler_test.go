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

package instance

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

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

const instanceProofChild = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type instanceProofReader struct {
	node *corev1.Node
	err  error
}

func (r *instanceProofReader) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if r.err != nil {
		return r.err
	}
	r.node.DeepCopyInto(obj.(*corev1.Node))
	return nil
}

func (r *instanceProofReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errors.New("unexpected List")
}

func instanceProofConfig() (string, map[string]any) {
	raw := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"` + instanceProofChild + `","size":100,"platform":{"architecture":"amd64","os":"linux"}}]}`)
	parent := digest.FromBytes(raw).String()
	return parent, map[string]any{
		"imageIndexProofs": []map[string]any{{
			"indexDigest": parent,
			"indexBase64": base64.StdEncoding.EncodeToString(raw),
		}},
	}
}

func instanceProofPod(parent string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Containers: []corev1.Container{{
				Name:  "kbagent",
				Image: "example/tools@" + instanceProofChild,
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

func TestImageIndexProofReconcilerPropagation(t *testing.T) {
	oldRegistries := viper.Get(constant.CfgRegistries)
	parent, config := instanceProofConfig()
	viper.Set(constant.CfgRegistries, config)
	if err := intctrlutil.LoadRegistryConfig(); err != nil {
		t.Fatalf("LoadRegistryConfig() error = %v", err)
	}
	defer func() {
		viper.Set(constant.CfgRegistries, oldRegistries)
		if err := intctrlutil.LoadRegistryConfig(); err != nil {
			t.Errorf("restore LoadRegistryConfig() error = %v", err)
		}
	}()

	newTree := func() (*kubebuilderx.ObjectTree, *workloads.Instance, *corev1.Pod) {
		inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}
		pod := instanceProofPod(parent)
		tree := kubebuilderx.NewObjectTree()
		tree.Context = context.Background()
		tree.SetRoot(inst)
		if err := tree.Add(pod); err != nil {
			t.Fatalf("tree.Add() error = %v", err)
		}
		return tree, inst, pod
	}

	t.Run("status returns transient Node errors", func(t *testing.T) {
		tree, _, _ := newTree()
		_, err := NewStatusReconciler(&instanceProofReader{err: errors.New("temporary Node read")}).Reconcile(tree)
		if err == nil || !strings.Contains(err.Error(), "temporary Node read") {
			t.Fatalf("status Reconcile() error = %v, want temporary Node read", err)
		}
	})

	t.Run("update gate returns transient Node errors", func(t *testing.T) {
		tree, inst, pod := newTree()
		matched, retry, err := (&updateReconciler{reader: &instanceProofReader{
			err: errors.New("temporary Node read"),
		}}).isPodCanBeUpdated(tree, inst, pod)
		if matched || retry || err == nil || !strings.Contains(err.Error(), "temporary Node read") {
			t.Fatalf("isPodCanBeUpdated() = (%v, %v, %v), want (false, false, temporary error)", matched, retry, err)
		}
	})

	t.Run("update gate accepts the exact Node platform relation", func(t *testing.T) {
		tree, inst, pod := newTree()
		reader := &instanceProofReader{node: &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
			Status: corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: "linux",
				Architecture:    "amd64",
			}},
		}}
		matched, retry, err := (&updateReconciler{reader: reader}).isPodCanBeUpdated(tree, inst, pod)
		if err != nil || !matched || retry {
			t.Fatalf("isPodCanBeUpdated() = (%v, %v, %v), want (true, false, nil)", matched, retry, err)
		}
	})
}
