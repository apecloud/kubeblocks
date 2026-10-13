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

package controllerutil

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	alpha2IndexDigest = "sha256:3f082e22e2e829c986b5d31099353510e3762a233689254483c04bb84c3c6777"
	alpha2AMD64Child  = "sha256:8ac42c16a1ff759707b4454d466ef172b0198bd50baac1c4e2526d104f0f9fd7"
	alpha2ARM64Child  = "sha256:3d1c9bd0c0e452fd87d3959d1ac88f9d7a54bd4dde0a09050af54abb279b81fc"
	alpha1Direct      = "sha256:67924d0a1b54dcb34bb2fc4401c7c77709f4cb53dc7b9cf44c0db19ba62f0585"
	alpha2IndexBase64 = "ewogICJzY2hlbWFWZXJzaW9uIjogMiwKICAibWVkaWFUeXBlIjogImFwcGxpY2F0aW9uL3ZuZC5vY2kuaW1hZ2UuaW5kZXgudjEranNvbiIsCiAgIm1hbmlmZXN0cyI6IFsKICAgIHsKICAgICAgIm1lZGlhVHlwZSI6ICJhcHBsaWNhdGlvbi92bmQub2NpLmltYWdlLm1hbmlmZXN0LnYxK2pzb24iLAogICAgICAiZGlnZXN0IjogInNoYTI1Njo4YWM0MmMxNmExZmY3NTk3MDdiNDQ1NGQ0NjZlZjE3MmIwMTk4YmQ1MGJhYWMxYzRlMjUyNmQxMDRmMGY5ZmQ3IiwKICAgICAgInNpemUiOiAxNjI1LAogICAgICAicGxhdGZvcm0iOiB7CiAgICAgICAgImFyY2hpdGVjdHVyZSI6ICJhbWQ2NCIsCiAgICAgICAgIm9zIjogImxpbnV4IgogICAgICB9CiAgICB9LAogICAgewogICAgICAibWVkaWFUeXBlIjogImFwcGxpY2F0aW9uL3ZuZC5vY2kuaW1hZ2UubWFuaWZlc3QudjEranNvbiIsCiAgICAgICJkaWdlc3QiOiAic2hhMjU2OjNkMWM5YmQwYzBlNDUyZmQ4N2QzOTU5ZDFhYzg4ZjlkN2E1NGJkNGRkZTBhMDkwNTBhZjU0YWJiMjc5YjgxZmMiLAogICAgICAic2l6ZSI6IDE2MjUsCiAgICAgICJwbGF0Zm9ybSI6IHsKICAgICAgICAiYXJjaGl0ZWN0dXJlIjogImFybTY0IiwKICAgICAgICAib3MiOiAibGludXgiCiAgICAgIH0KICAgIH0KICBdCn0="
)

type imageProofReader struct {
	node *corev1.Node
	err  error
	gets int
}

func (r *imageProofReader) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	r.gets++
	if r.err != nil {
		return r.err
	}
	if r.node == nil {
		return fmt.Errorf("node not found")
	}
	r.node.DeepCopyInto(obj.(*corev1.Node))
	return nil
}

func (r *imageProofReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errors.New("unexpected List")
}

func installImageIndexProofs(configs ...imageIndexProofConfig) error {
	proofs, err := compileImageIndexProofs(configs)
	if err != nil {
		return err
	}
	registriesConfigMutex.Lock()
	registriesConfigInstance = &registriesConfig{
		ImageIndexProofs:         configs,
		compiledImageIndexProofs: proofs,
	}
	registriesConfigMutex.Unlock()
	return nil
}

func syntheticImageIndex(descriptors ...ocispec.Descriptor) imageIndexProofConfig {
	raw, err := json.Marshal(ocispec.Index{
		Versioned: ocispec.Index{}.Versioned,
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: descriptors,
	})
	Expect(err).NotTo(HaveOccurred())
	var index map[string]any
	Expect(json.Unmarshal(raw, &index)).To(Succeed())
	index["schemaVersion"] = 2
	raw, err = json.Marshal(index)
	Expect(err).NotTo(HaveOccurred())
	return imageIndexProofConfig{
		IndexDigest: digest.FromBytes(raw).String(),
		IndexBase64: base64.StdEncoding.EncodeToString(raw),
	}
}

func imageDescriptor(child string, operatingSystem string, architecture string) ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.Digest(child),
		Size:      100,
		Platform: &ocispec.Platform{
			OS:           operatingSystem,
			Architecture: architecture,
		},
	}
}

func alpha2Pod(nodeName string, specDigest string, statusDigest string) *corev1.Pod {
	return &corev1.Pod{
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name:  "kbagent",
				Image: "apecloud/kubeblocks-tools@" + specDigest,
			}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "kbagent",
				Image:   "apecloud/kubeblocks-tools:1.2.0-alpha.2",
				ImageID: "apecloud/kubeblocks-tools@" + statusDigest,
			}},
		},
	}
}

var _ = Describe("image index proof", Serial, func() {
	BeforeEach(func() {
		Expect(installImageIndexProofs()).To(Succeed())
	})

	AfterEach(func() {
		Expect(installImageIndexProofs()).To(Succeed())
	})

	It("validates the exact alpha2 raw bytes and matches both digest directions", func() {
		Expect(installImageIndexProofs(imageIndexProofConfig{
			IndexDigest: alpha2IndexDigest,
			IndexBase64: alpha2IndexBase64,
		})).To(Succeed())

		provider := func() (string, string, error) { return "linux", "amd64", nil }
		matched, err := MatchContainerImageInStatusWithPlatform(
			"apecloud/kubeblocks-tools@"+alpha2AMD64Child,
			"apecloud/kubeblocks-tools:1.2.0-alpha.2",
			"apecloud/kubeblocks-tools@"+alpha2IndexDigest,
			provider)
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())

		matched, err = MatchContainerImageInStatusWithPlatform(
			"apecloud/kubeblocks-tools@"+alpha2IndexDigest,
			"apecloud/kubeblocks-tools:1.2.0-alpha.2",
			"apecloud/kubeblocks-tools@"+alpha2AMD64Child,
			provider)
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
	})

	It("accepts a Docker manifest list with Docker image manifests", func() {
		proof := syntheticImageIndex(imageDescriptor(alpha2AMD64Child, "linux", "amd64"))
		raw, err := base64.StdEncoding.DecodeString(proof.IndexBase64)
		Expect(err).NotTo(HaveOccurred())
		var index map[string]any
		Expect(json.Unmarshal(raw, &index)).To(Succeed())
		index["mediaType"] = dockerManifestListMediaType
		manifests := index["manifests"].([]any)
		manifests[0].(map[string]any)["mediaType"] = dockerImageManifestMediaType
		raw, err = json.Marshal(index)
		Expect(err).NotTo(HaveOccurred())
		proof.IndexDigest = digest.FromBytes(raw).String()
		proof.IndexBase64 = base64.StdEncoding.EncodeToString(raw)
		Expect(installImageIndexProofs(proof)).To(Succeed())

		matched, err := MatchContainerImageInStatusWithPlatform(
			"example/tools@"+alpha2AMD64Child, "example/tools:tag", "example/tools@"+proof.IndexDigest,
			func() (string, string, error) { return "linux", "amd64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
	})

	It("keeps direct digest equality independent of proof and platform reads", func() {
		providerCalled := false
		matched, err := MatchContainerImageInStatusWithPlatform(
			"apecloud/kubeblocks-tools@"+alpha1Direct,
			"apecloud/kubeblocks-tools:1.2.0-alpha.1",
			"apecloud/kubeblocks-tools@"+alpha1Direct,
			func() (string, string, error) {
				providerCalled = true
				return "", "", errors.New("must not be called")
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
		Expect(providerCalled).To(BeFalse())
	})

	It("fails closed for wrong platform, parent, child, absent proof, and old-child in-place state", func() {
		Expect(installImageIndexProofs(imageIndexProofConfig{
			IndexDigest: alpha2IndexDigest,
			IndexBase64: alpha2IndexBase64,
		})).To(Succeed())

		cases := []struct {
			name         string
			specDigest   string
			statusDigest string
			os           string
			arch         string
		}{
			{name: "arm64 child on amd64", specDigest: alpha2ARM64Child, statusDigest: alpha2IndexDigest, os: "linux", arch: "amd64"},
			{name: "reverse arm64 child on amd64", specDigest: alpha2IndexDigest, statusDigest: alpha2ARM64Child, os: "linux", arch: "amd64"},
			{name: "wrong parent", specDigest: alpha2AMD64Child, statusDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", os: "linux", arch: "amd64"},
			{name: "wrong child", specDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", statusDigest: alpha2IndexDigest, os: "linux", arch: "amd64"},
			{name: "new child while old parent is still reported", specDigest: alpha2ARM64Child, statusDigest: alpha2IndexDigest, os: "linux", arch: "amd64"},
		}
		for _, tt := range cases {
			By(tt.name)
			matched, err := MatchContainerImageInStatusWithPlatform(
				"example/tools@"+tt.specDigest, "example/tools:tag", "example/tools@"+tt.statusDigest,
				func() (string, string, error) { return tt.os, tt.arch, nil })
			Expect(err).NotTo(HaveOccurred())
			Expect(matched).To(BeFalse())
		}

		Expect(installImageIndexProofs()).To(Succeed())
		matched, err := MatchContainerImageInStatusWithPlatform(
			"example/tools@"+alpha2AMD64Child, "example/tools:tag", "example/tools@"+alpha2IndexDigest,
			func() (string, string, error) { return "linux", "amd64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())
	})

	It("propagates platform errors and treats unknown platform as not matched", func() {
		Expect(installImageIndexProofs(imageIndexProofConfig{
			IndexDigest: alpha2IndexDigest,
			IndexBase64: alpha2IndexBase64,
		})).To(Succeed())
		temporary := errors.New("temporary Node read")
		matched, err := MatchContainerImageInStatusWithPlatform(
			"example/tools@"+alpha2AMD64Child, "example/tools:tag", "example/tools@"+alpha2IndexDigest,
			func() (string, string, error) { return "", "", temporary })
		Expect(matched).To(BeFalse())
		Expect(err).To(MatchError(temporary))

		matched, err = MatchContainerImageInStatusWithPlatform(
			"example/tools@"+alpha2AMD64Child, "example/tools:tag", "example/tools@"+alpha2IndexDigest,
			func() (string, string, error) { return "", "", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())
	})

	It("reads the scheduled Node lazily and rejects missing or terminating Nodes", func() {
		Expect(installImageIndexProofs(imageIndexProofConfig{
			IndexDigest: alpha2IndexDigest,
			IndexBase64: alpha2IndexBase64,
		})).To(Succeed())

		reader := &imageProofReader{node: &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
			Status: corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: "linux",
				Architecture:    "amd64",
			}},
		}}
		matched, err := MatchPodContainerImages(context.Background(), reader,
			alpha2Pod("node-1", alpha2AMD64Child, alpha2IndexDigest))
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
		Expect(reader.gets).To(Equal(1))

		unscheduled := alpha2Pod("", alpha2AMD64Child, alpha2IndexDigest)
		matched, err = MatchPodContainerImages(context.Background(), nil, unscheduled)
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())

		reader = &imageProofReader{err: apierrors.NewNotFound(schema.GroupResource{Resource: "nodes"}, "node-1")}
		matched, err = MatchPodContainerImages(context.Background(), reader,
			alpha2Pod("node-1", alpha2AMD64Child, alpha2IndexDigest))
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())

		terminatingAt := metav1.Now()
		reader = &imageProofReader{node: &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1", DeletionTimestamp: &terminatingAt},
			Status: corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: "linux",
				Architecture:    "amd64",
			}},
		}}
		matched, err = MatchPodContainerImages(context.Background(), reader,
			alpha2Pod("node-1", alpha2AMD64Child, alpha2IndexDigest))
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())
	})

	It("propagates transient Node read errors but avoids reads for impossible pairs", func() {
		Expect(installImageIndexProofs(imageIndexProofConfig{
			IndexDigest: alpha2IndexDigest,
			IndexBase64: alpha2IndexBase64,
		})).To(Succeed())

		reader := &imageProofReader{err: errors.New("temporary")}
		matched, err := MatchPodContainerImages(context.Background(), reader,
			alpha2Pod("node-1", alpha2AMD64Child, alpha2IndexDigest))
		Expect(matched).To(BeFalse())
		Expect(err).To(MatchError(ContainSubstring("temporary")))
		Expect(reader.gets).To(Equal(1))

		reader = &imageProofReader{err: errors.New("must not be called")}
		matched, err = MatchPodContainerImages(context.Background(), reader,
			alpha2Pod("node-1", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", alpha2IndexDigest))
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())
		Expect(reader.gets).To(BeZero())
	})

	It("rejects changed raw bytes, invalid JSON, unsupported media type, and duplicate parents", func() {
		_, err := compileImageIndexProofs([]imageIndexProofConfig{{
			IndexDigest: alpha2IndexDigest,
			IndexBase64: "not-base64",
		}})
		Expect(err).To(MatchError(ContainSubstring("decode indexBase64")))

		raw, err := base64.StdEncoding.DecodeString(alpha2IndexBase64)
		Expect(err).NotTo(HaveOccurred())
		var compacted any
		Expect(json.Unmarshal(raw, &compacted)).To(Succeed())
		reformatted, err := json.Marshal(compacted)
		Expect(err).NotTo(HaveOccurred())
		_, err = compileImageIndexProofs([]imageIndexProofConfig{{
			IndexDigest: alpha2IndexDigest,
			IndexBase64: base64.StdEncoding.EncodeToString(reformatted),
		}})
		Expect(err).To(MatchError(ContainSubstring("does not match")))

		invalidJSON := []byte("not-json")
		_, err = compileImageIndexProofs([]imageIndexProofConfig{{
			IndexDigest: digest.FromBytes(invalidJSON).String(),
			IndexBase64: base64.StdEncoding.EncodeToString(invalidJSON),
		}})
		Expect(err).To(MatchError(ContainSubstring("decode raw index JSON")))

		wrongMedia := syntheticImageIndex(imageDescriptor(alpha2AMD64Child, "linux", "amd64"))
		wrongRaw, err := base64.StdEncoding.DecodeString(wrongMedia.IndexBase64)
		Expect(err).NotTo(HaveOccurred())
		var wrongIndex map[string]any
		Expect(json.Unmarshal(wrongRaw, &wrongIndex)).To(Succeed())
		wrongIndex["mediaType"] = "application/json"
		wrongRaw, err = json.Marshal(wrongIndex)
		Expect(err).NotTo(HaveOccurred())
		wrongMedia.IndexDigest = digest.FromBytes(wrongRaw).String()
		wrongMedia.IndexBase64 = base64.StdEncoding.EncodeToString(wrongRaw)
		_, err = compileImageIndexProofs([]imageIndexProofConfig{wrongMedia})
		Expect(err).To(MatchError(ContainSubstring("unsupported index mediaType")))

		_, err = compileImageIndexProofs([]imageIndexProofConfig{
			{IndexDigest: alpha2IndexDigest, IndexBase64: alpha2IndexBase64},
			{IndexDigest: alpha2IndexDigest, IndexBase64: alpha2IndexBase64},
		})
		Expect(err).To(MatchError(ContainSubstring("duplicate index digest")))
	})

	It("isolates an ambiguous platform while retaining another unique platform", func() {
		proof := syntheticImageIndex(
			imageDescriptor("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "linux", "amd64"),
			imageDescriptor("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "linux", "amd64"),
			imageDescriptor(alpha2ARM64Child, "linux", "arm64"),
			imageDescriptor("sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "unknown", "unknown"),
		)
		Expect(installImageIndexProofs(proof)).To(Succeed())

		matched, err := MatchContainerImageInStatusWithPlatform(
			"example/tools@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"example/tools:tag", "example/tools@"+proof.IndexDigest,
			func() (string, string, error) { return "linux", "amd64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())

		matched, err = MatchContainerImageInStatusWithPlatform(
			"example/tools@"+proof.IndexDigest,
			"example/tools:tag", "example/tools@"+alpha2ARM64Child,
			func() (string, string, error) { return "linux", "arm64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())

		variantAMD64 := imageDescriptor(alpha2AMD64Child, "linux", "amd64")
		variantAMD64.Platform.Variant = "v3"
		variantProof := syntheticImageIndex(
			variantAMD64,
			imageDescriptor(alpha2ARM64Child, "linux", "arm64"),
		)
		Expect(installImageIndexProofs(variantProof)).To(Succeed())
		matched, err = MatchContainerImageInStatusWithPlatform(
			"example/tools@"+alpha2AMD64Child,
			"example/tools:tag", "example/tools@"+variantProof.IndexDigest,
			func() (string, string, error) { return "linux", "amd64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())
		matched, err = MatchContainerImageInStatusWithPlatform(
			"example/tools@"+variantProof.IndexDigest,
			"example/tools:tag", "example/tools@"+alpha2ARM64Child,
			func() (string, string, error) { return "linux", "arm64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())

		allAmbiguous := syntheticImageIndex(
			imageDescriptor("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "linux", "amd64"),
			imageDescriptor("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "linux", "amd64"),
		)
		Expect(installImageIndexProofs(allAmbiguous)).To(MatchError(ContainSubstring("no uniquely usable")))
	})

	It("preserves the previous complete snapshot after an invalid hot reload", func() {
		oldRegistries := viper.Get(constant.CfgRegistries)
		defer func() {
			viper.Set(constant.CfgRegistries, oldRegistries)
			Expect(LoadRegistryConfig()).To(Succeed())
		}()

		viper.Set(constant.CfgRegistries, map[string]any{
			"imageIndexProofs": []map[string]any{{
				"indexDigest": alpha2IndexDigest,
				"indexBase64": alpha2IndexBase64,
			}},
		})
		Expect(LoadRegistryConfig()).To(Succeed())

		newProof := syntheticImageIndex(imageDescriptor(
			"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "linux", "amd64"))
		newParent := newProof.IndexDigest
		newProof.IndexBase64 = base64.StdEncoding.EncodeToString([]byte("invalid"))
		viper.Set(constant.CfgRegistries, map[string]any{
			"imageIndexProofs": []map[string]any{{
				"indexDigest": newProof.IndexDigest,
				"indexBase64": newProof.IndexBase64,
			}},
		})
		Expect(LoadRegistryConfig()).To(MatchError(ContainSubstring("does not match")))

		matched, err := MatchContainerImageInStatusWithPlatform(
			"example/tools@"+alpha2AMD64Child, "example/tools:tag", "example/tools@"+alpha2IndexDigest,
			func() (string, string, error) { return "linux", "amd64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())

		matched, err = MatchContainerImageInStatusWithPlatform(
			"example/tools@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			"example/tools:tag", "example/tools@"+newParent,
			func() (string, string, error) { return "linux", "amd64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())
	})

	It("validates the newly loaded registry mappings before replacing the snapshot", func() {
		oldRegistries := viper.Get(constant.CfgRegistries)
		defer func() {
			viper.Set(constant.CfgRegistries, oldRegistries)
			Expect(LoadRegistryConfig()).To(Succeed())
		}()

		viper.Set(constant.CfgRegistries, map[string]any{
			"imageIndexProofs": []map[string]any{{
				"indexDigest": alpha2IndexDigest,
				"indexBase64": alpha2IndexBase64,
			}},
		})
		Expect(LoadRegistryConfig()).To(Succeed())

		viper.Set(constant.CfgRegistries, map[string]any{
			"registryConfig": []map[string]any{{"from": "", "to": "mirror.example"}},
		})
		Expect(LoadRegistryConfig()).To(MatchError(ContainSubstring("from can't be empty")))

		matched, err := MatchContainerImageInStatusWithPlatform(
			"example/tools@"+alpha2AMD64Child, "example/tools:tag", "example/tools@"+alpha2IndexDigest,
			func() (string, string, error) { return "linux", "amd64", nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
	})
})
