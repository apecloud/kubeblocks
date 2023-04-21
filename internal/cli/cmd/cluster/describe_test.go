/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package cluster

import (
	"bytes"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Expose", func() {
	const (
		namespace   = "test"
		clusterName = "test"
	)

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
		cluster = testing.FakeCluster(clusterName, namespace)
		pods    = testing.FakePods(3, namespace, clusterName)
	)
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				mapping := map[string]*http.Response{
					"/api/v1/nodes/" + testing.NodeName:   httpResp(testing.FakeNode()),
					urlPrefix + "/services":               httpResp(&corev1.ServiceList{}),
					urlPrefix + "/events":                 httpResp(&corev1.EventList{}),
					urlPrefix + "/persistentvolumeclaims": httpResp(&corev1.PersistentVolumeClaimList{}),
					urlPrefix + "/pods":                   httpResp(pods),
				}
				return mapping[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, testing.FakeClusterDef(), testing.FakeClusterVersion())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("describe", func() {
		cmd := NewDescribeCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("complete", func() {
		o := newOptions(tf, streams)
		Expect(o.complete(nil)).Should(HaveOccurred())
		Expect(o.complete([]string{clusterName})).Should(Succeed())
		Expect(o.names).Should(Equal([]string{clusterName}))
		Expect(o.client).ShouldNot(BeNil())
		Expect(o.dynamic).ShouldNot(BeNil())
		Expect(o.namespace).Should(Equal(namespace))
	})

	It("run", func() {
		o := newOptions(tf, streams)
		Expect(o.complete([]string{clusterName})).Should(Succeed())
		Expect(o.run()).Should(Succeed())
	})

	It("showEvents", func() {
		out := &bytes.Buffer{}
		showEvents(testing.FakeEvents(), "test-cluster", namespace, out)
		strs := strings.Split(out.String(), "\n")

		// sorted
		firstEvent := strs[3]
		secondEvent := strs[4]
		Expect(strings.Compare(firstEvent, secondEvent) < 0).Should(BeTrue())
	})

	It("showDataProtections", func() {
		out := &bytes.Buffer{}
		fakeBackupPolicies := []dpv1alpha1.BackupPolicy{
			*testing.FakeBackupPolicy("backup-policy-test", "test-cluster"),
		}
		fakeBackups := []dpv1alpha1.Backup{
			*testing.FakeBackup("backup-test"),
			*testing.FakeBackup("backup-test2"),
		}
		now := metav1.Now()
		fakeBackups[0].Status = dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupCompleted,
			Manifests: &dpv1alpha1.ManifestsStatus{
				BackupLog: &dpv1alpha1.BackupLogStatus{
					StartTime: &now,
					StopTime:  &now,
				},
			},
		}
		after := metav1.Time{Time: now.Add(time.Hour)}
		fakeBackups[1].Status = dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupCompleted,
			Manifests: &dpv1alpha1.ManifestsStatus{
				BackupLog: &dpv1alpha1.BackupLogStatus{
					StartTime: &now,
					StopTime:  &after,
				},
			},
		}
		showDataProtection(fakeBackupPolicies, fakeBackups, out)
		strs := strings.Split(out.String(), "\n")

		Expect(strs).ShouldNot(BeEmpty())
	})
})
