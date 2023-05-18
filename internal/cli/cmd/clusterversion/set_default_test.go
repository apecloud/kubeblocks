/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package clusterversion

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("set-default", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	BeforeEach(func() {
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		_ = metav1.AddMetaToScheme(scheme.Scheme)
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	getFakeClusterVersion := func(dynamic dynamic.Interface, clusterVersionName string) (*appsv1alpha1.ClusterVersion, error) {
		var cv appsv1alpha1.ClusterVersion
		u, err := dynamic.Resource(ClusterVersionGVR).Get(context.Background(), clusterVersionName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &cv)
		if err != nil {
			return nil, err
		}
		return &cv, nil
	}

	It("test getIsDefault", func() {
		cv := &appsv1alpha1.ClusterVersion{}
		Expect(getIsDefault(cv)).Should(Equal(annotationFalseValue))
		cv.SetAnnotations(map[string]string{
			constant.IsDefaultClusterVersionAnnotationKey: annotationFalseValue,
		})
		Expect(getIsDefault(cv)).Should(Equal(annotationFalseValue))
		cv.Annotations[constant.IsDefaultClusterVersionAnnotationKey] = annotationTrueValue
		Expect(getIsDefault(cv)).Should(Equal(annotationTrueValue))
	})

	It("set-default cmd", func() {
		cmd := NewSetDefaultCMD(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("unset-default cmd", func() {
		cmd := NewUnSetDefaultCMD(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("set-default empty args", func() {
		o := NewSetOrUnsetDefaultOptions(tf, streams, true)
		Expect(o.validate([]string{})).Should(HaveOccurred())
	})

	It("unset-default empty args", func() {
		o := NewSetOrUnsetDefaultOptions(tf, streams, false)
		Expect(o.validate([]string{})).Should(HaveOccurred())
	})

	It("set-default and unset-default", func() {
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterVersion())
		cmd := NewSetDefaultCMD(tf, streams)
		// before set-default
		cv, err := getFakeClusterVersion(tf.FakeDynamicClient, testing.ClusterVersionName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv)).Should(Equal(annotationFalseValue))

		// set-default
		cmd.Run(cmd, []string{testing.ClusterVersionName})
		cv, err = getFakeClusterVersion(tf.FakeDynamicClient, testing.ClusterVersionName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv)).Should(Equal(annotationTrueValue))
		// unset-default
		cmd = NewUnSetDefaultCMD(tf, streams)
		cmd.Run(cmd, []string{testing.ClusterVersionName})
		cv, err = getFakeClusterVersion(tf.FakeDynamicClient, testing.ClusterVersionName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv)).Should(Equal(annotationFalseValue))
	})

	It("the clusterDef already have a default cv when set-default", func() {
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterVersion())
		o := NewSetOrUnsetDefaultOptions(tf, streams, true)
		err := o.run([]string{testing.ClusterVersionName})
		Expect(err).ShouldNot(HaveOccurred())
		cv, err := getFakeClusterVersion(tf.FakeDynamicClient, testing.ClusterVersionName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv)).Should(Equal(annotationTrueValue))
		err = o.run([]string{testing.ClusterVersionName})
		Expect(err).Should(HaveOccurred())
	})

	It("set-default args belong the same cd", func() {
		cv1 := testing.FakeClusterVersion()
		cv2 := testing.FakeClusterVersion()
		cv2.Name += "-other"
		tf.FakeDynamicClient = testing.FakeDynamicClient(cv1, cv2)
		o := NewSetOrUnsetDefaultOptions(tf, streams, true)
		err := o.run([]string{cv1.Name, cv2.Name})
		Expect(err).Should(HaveOccurred())
	})

	It("set-default and unset-default more than one args", func() {
		cv1 := testing.FakeClusterVersion()
		cv2 := testing.FakeClusterVersion()
		cv2.Name += "-other"
		cv2.SetLabels(map[string]string{
			constant.ClusterDefLabelKey:   testing.ClusterDefName + "-other",
			constant.AppManagedByLabelKey: constant.AppName,
		})
		tf.FakeDynamicClient = testing.FakeDynamicClient(cv1, cv2)
		cmd := NewSetDefaultCMD(tf, streams)
		cv1Res, err := getFakeClusterVersion(tf.FakeDynamicClient, cv1.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv1Res)).Should(Equal(annotationFalseValue))
		cv2Res, err := getFakeClusterVersion(tf.FakeDynamicClient, cv2.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv2Res)).Should(Equal(annotationFalseValue))
		// set-default
		cmd.Run(cmd, []string{cv1.Name, cv2.Name})
		cv1Res, err = getFakeClusterVersion(tf.FakeDynamicClient, cv1.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv1Res)).Should(Equal(annotationTrueValue))
		cv2Res, err = getFakeClusterVersion(tf.FakeDynamicClient, cv2.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv2Res)).Should(Equal(annotationTrueValue))
		// unset-defautl
		cmd = NewUnSetDefaultCMD(tf, streams)
		cmd.Run(cmd, []string{cv1.Name, cv2.Name})
		cv1Res, err = getFakeClusterVersion(tf.FakeDynamicClient, cv1.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv1Res)).Should(Equal(annotationFalseValue))
		cv2Res, err = getFakeClusterVersion(tf.FakeDynamicClient, cv2.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getIsDefault(cv2Res)).Should(Equal(annotationFalseValue))
	})
})
