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

	const (
		cluterversionName       = testing.ClusterVersionName
		otherClusterversionName = testing.ClusterName + "-other"
	)
	BeforeEach(func() {
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		_ = metav1.AddMetaToScheme(scheme.Scheme)
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterVersion())
	})

	beginWithTwoClusterversion := func() {
		tf.FakeDynamicClient = testing.FakeDynamicClient([]runtime.Object{
			&appsv1alpha1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name: cluterversionName,
					Labels: map[string]string{
						constant.ClusterDefLabelKey: testing.ClusterDefName,
					},
				},
			},
			&appsv1alpha1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name: otherClusterversionName,
					Labels: map[string]string{
						constant.ClusterDefLabelKey: testing.ClusterDefName + "-other",
					},
				},
			},
		}...)
	}

	getFakeClusterVersion := func(dynamic dynamic.Interface, clusterVersionName string) (*appsv1alpha1.ClusterVersion, error) {
		var cv appsv1alpha1.ClusterVersion
		u, err := dynamic.Resource(clusterVersionGVR).Get(context.Background(), clusterVersionName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &cv)
		if err != nil {
			return nil, err
		}
		return &cv, nil
	}

	var validateSetOrUnsetResult func(needToChecks []string, value []string)
	validateSetOrUnsetResult = func(needToChecks []string, value []string) {
		if len(needToChecks) == 1 {
			cv, err := getFakeClusterVersion(tf.FakeDynamicClient, needToChecks[0])
			Expect(err).Should(Succeed())
			Expect(isDefault(cv)).Should(Equal(value[0]))
			return
		}
		validateSetOrUnsetResult(needToChecks[1:], value[1:])
	}

	It("test isDefault Func", func() {
		cv := testing.FakeClusterVersion()
		Expect(isDefault(cv)).Should(Equal(annotationFalseValue))
		cv.SetAnnotations(map[string]string{
			constant.DefaultClusterVersionAnnotationKey: annotationFalseValue,
		})
		Expect(isDefault(cv)).Should(Equal(annotationFalseValue))
		cv.Annotations[constant.DefaultClusterVersionAnnotationKey] = annotationTrueValue
		Expect(isDefault(cv)).Should(Equal(annotationTrueValue))
	})

	It("set-default cmd", func() {
		cmd := newSetDefaultCMD(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("unset-default cmd", func() {
		cmd := newUnSetDefaultCMD(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("set-default empty args", func() {
		o := newSetOrUnsetDefaultOptions(tf, streams, true)
		Expect(o.validate([]string{})).Should(HaveOccurred())
	})

	It("unset-default empty args", func() {
		o := newSetOrUnsetDefaultOptions(tf, streams, false)
		Expect(o.validate([]string{})).Should(HaveOccurred())
	})

	It("set-default and unset-default", func() {
		// before set-default
		validateSetOrUnsetResult([]string{cluterversionName}, []string{annotationFalseValue})
		// set-default
		cmd := newSetDefaultCMD(tf, streams)
		cmd.Run(cmd, []string{cluterversionName})
		validateSetOrUnsetResult([]string{cluterversionName}, []string{annotationTrueValue})
		// unset-default
		cmd = newUnSetDefaultCMD(tf, streams)
		cmd.Run(cmd, []string{cluterversionName})
		validateSetOrUnsetResult([]string{cluterversionName}, []string{annotationFalseValue})
	})

	It("the clusterDef already have a default cv when set-default", func() {
		cmd := newSetDefaultCMD(tf, streams)
		cmd.Run(cmd, []string{cluterversionName})
		validateSetOrUnsetResult([]string{cluterversionName}, []string{annotationTrueValue})
		o := newSetOrUnsetDefaultOptions(tf, streams, true)
		err := o.run([]string{cluterversionName})
		Expect(err).Should(HaveOccurred())
	})

	It("set-default args belong the same cd", func() {
		o := newSetOrUnsetDefaultOptions(tf, streams, true)
		err := o.run([]string{cluterversionName, cluterversionName})
		Expect(err).Should(HaveOccurred())
	})

	It("set-default and unset-default more than one args", func() {
		beginWithTwoClusterversion()
		cmd := newSetDefaultCMD(tf, streams)
		validateSetOrUnsetResult([]string{cluterversionName, otherClusterversionName}, []string{annotationFalseValue, annotationFalseValue})
		// set-default
		cmd.Run(cmd, []string{cluterversionName, otherClusterversionName})
		validateSetOrUnsetResult([]string{cluterversionName, otherClusterversionName}, []string{annotationTrueValue, annotationTrueValue})
		// unset-defautl
		cmd = newUnSetDefaultCMD(tf, streams)
		cmd.Run(cmd, []string{cluterversionName, otherClusterversionName})
		validateSetOrUnsetResult([]string{cluterversionName, otherClusterversionName}, []string{annotationFalseValue, annotationFalseValue})
	})
})
