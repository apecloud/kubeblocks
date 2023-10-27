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

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

const (
	msg = "not implemented for test"
)

var _ = Describe("Lorry HTTP Client", func() {
	var pod *corev1.Pod

	BeforeEach(func() {
		podName := "pod-for-lorry-test"
		pod = testapps.NewPodFactory("default", podName).
			AddContainer(corev1.Container{
				Name:    testapps.DefaultNginxContainerName,
				Command: []string{"lorry", "--port", strconv.Itoa(lorryHTTPPort)},
				Image:   testapps.NginxImage}).
			GetObject()
		pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{
				ContainerPort: int32(lorryHTTPPort),
				Name:          constant.LorryHTTPPortName,
				Protocol:      "TCP",
			},
			{
				ContainerPort: int32(50001),
				Name:          constant.LorryGRPCPortName,
				Protocol:      "TCP",
			},
		}
		pod.Status.PodIP = "127.0.0.1"
	})

	Context("new HTTPClient", func() {
		It("without lorry service, return nil", func() {
			podWithoutLorry := pod.DeepCopy()
			podWithoutLorry.Spec.Containers[0].Ports = nil
			lorryClient, err := NewHTTPClientWithPod(podWithoutLorry)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(lorryClient).Should(BeNil())
		})

		It("without pod ip, failed", func() {
			podWithoutPodIP := pod.DeepCopy()
			podWithoutPodIP.Status.PodIP = ""
			_, err := NewHTTPClientWithPod(podWithoutPodIP)
			Expect(err).Should(HaveOccurred())
		})

		It("success", func() {
			lorryClient, err := NewHTTPClientWithPod(pod)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(lorryClient).ShouldNot(BeNil())
		})
	})

	Context("request with timeout", func() {
		var httpServer *httptest.Server
		var port int
		var lorryClient *HTTPClient

		BeforeEach(func() {
			pod1 := pod.DeepCopy()
			body := []byte("{\"role\": \"leader\"}")
			httpServer, port = newHTTPServer(body)
			pod1.Spec.Containers[0].Ports[0].ContainerPort = int32(port)
			lorryClient, _ = NewHTTPClientWithPod(pod1)
			Expect(lorryClient).ShouldNot(BeNil())
		})

		AfterEach(func() {
			httpServer.Close()
		})

		It("response in time", func() {
			lorryClient.ReconcileTimeout = 1 * time.Second
			_, err := lorryClient.GetRole(context.TODO())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(lorryClient.cache).Should(BeEmpty())
		})

		It("response timeout", func() {
			lorryClient.ReconcileTimeout = 50 * time.Millisecond
			_, err := lorryClient.GetRole(context.TODO())
			Expect(err).Should(HaveOccurred())
			// wait client to get response and cache it
			time.Sleep(200 * time.Millisecond)
			Expect(lorryClient.cache).Should(HaveLen(1))
		})

		It("response by cache", func() {
			lorryClient.ReconcileTimeout = 50 * time.Millisecond
			// get response from server, and timeout
			_, err := lorryClient.GetRole(context.TODO())
			Expect(err).Should(HaveOccurred())
			// wait client to get response and cache it
			time.Sleep(200 * time.Millisecond)
			// get response from cache
			_, err = lorryClient.GetRole(context.TODO())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(lorryClient.cache).Should(BeEmpty())
		})
	})

	Context("get replica role", func() {
		var lorryClient *HTTPClient

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
		})

		It("success", func() {
			role := "leader"
			mockDBManager.EXPECT().GetReplicaRole(gomock.Any(), gomock.Any()).Return(role, nil)
			Expect(lorryClient.GetRole(context.TODO())).Should(Equal(role))
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().GetReplicaRole(gomock.Any(), gomock.Any()).Return(string(""), fmt.Errorf(msg))
			role, err := lorryClient.GetRole(context.TODO())
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
			Expect(role).Should(BeEmpty())
		})
	})

	Context("list system accounts", func() {
		var lorryClient *HTTPClient
		var systemAccounts []models.UserInfo

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
			systemAccounts = []models.UserInfo{
				{
					UserName: "kb-admin1",
				},
				{
					UserName: "kb-admin2",
				},
			}
		})

		It("success", func() {
			mockDBManager.EXPECT().ListSystemAccounts(gomock.Any()).Return(systemAccounts, nil)
			Expect(lorryClient.ListSystemAccounts(context.TODO())).Should(HaveLen(2))
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().ListSystemAccounts(gomock.Any()).Return(nil, fmt.Errorf(msg))
			accounts, err := lorryClient.ListSystemAccounts(context.TODO())
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
			Expect(accounts).Should(BeEmpty())
		})
	})

	Context("create user", func() {
		var lorryClient *HTTPClient

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
		})

		It("success", func() {
			mockDBManager.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			Expect(lorryClient.CreateUser(context.TODO(), "user-test", "password-test")).Should(Succeed())
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf(msg))
			err := lorryClient.CreateUser(context.TODO(), "user-test", "password-test")
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
		})
	})

	Context("delete user", func() {
		var lorryClient *HTTPClient

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
		})

		It("success", func() {
			mockDBManager.EXPECT().DeleteUser(gomock.Any(), gomock.Any()).Return(nil)
			Expect(lorryClient.DeleteUser(context.TODO(), "user-test")).Should(Succeed())
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().DeleteUser(gomock.Any(), gomock.Any()).Return(fmt.Errorf(msg))
			err := lorryClient.DeleteUser(context.TODO(), "user-test")
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
		})
	})

	Context("describe user", func() {
		var lorryClient *HTTPClient
		var userInfo *models.UserInfo

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
			userInfo = &models.UserInfo{
				UserName: "kb-admin1",
			}
		})

		It("success", func() {
			mockDBManager.EXPECT().DescribeUser(gomock.Any(), gomock.Any()).Return(userInfo, nil)
			user, err := lorryClient.DescribeUser(context.TODO(), "user-test")
			Expect(err).Should(Succeed())
			Expect(user).ShouldNot(BeZero())
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().DescribeUser(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf(msg))
			_, err := lorryClient.DescribeUser(context.TODO(), "user-test")
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
		})
	})

	Context("grant user role", func() {
		var lorryClient *HTTPClient

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
		})

		It("success", func() {
			mockDBManager.EXPECT().GrantUserRole(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			Expect(lorryClient.GrantUserRole(context.TODO(), "user-test", "readwrite")).Should(Succeed())
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().GrantUserRole(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf(msg))
			err := lorryClient.GrantUserRole(context.TODO(), "user-test", "readwrite")
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
		})
	})

	Context("revoke user role", func() {
		var lorryClient *HTTPClient

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
		})

		It("success", func() {
			mockDBManager.EXPECT().RevokeUserRole(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			Expect(lorryClient.RevokeUserRole(context.TODO(), "user-test", "readwrite")).Should(Succeed())
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().RevokeUserRole(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf(msg))
			err := lorryClient.RevokeUserRole(context.TODO(), "user-test", "readwrite")
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
		})
	})

	Context("list users", func() {
		var lorryClient *HTTPClient
		var users []models.UserInfo

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
			users = []models.UserInfo{
				{
					UserName: "user1",
				},
				{
					UserName: "user2",
				},
			}
		})

		It("success", func() {
			mockDBManager.EXPECT().ListUsers(gomock.Any()).Return(users, nil)
			Expect(lorryClient.ListUsers(context.TODO())).Should(HaveLen(2))
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().ListUsers(gomock.Any()).Return(nil, fmt.Errorf(msg))
			users, err := lorryClient.ListUsers(context.TODO())
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
			Expect(users).Should(BeEmpty())
		})
	})

	Context("join member", func() {
		var lorryClient *HTTPClient
		var cluster *dcs.Cluster

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
			cluster = &dcs.Cluster{}
		})

		It("success if join once", func() {
			mockDBManager.EXPECT().JoinCurrentMemberToCluster(gomock.Any(), gomock.Any()).Return(nil)
			mockDCSStore.EXPECT().GetCluster().Return(cluster, nil)
			Expect(lorryClient.JoinMember(context.TODO())).Should(Succeed())
		})

		It("success if join twice", func() {
			mockDBManager.EXPECT().JoinCurrentMemberToCluster(gomock.Any(), gomock.Any()).Return(nil).Times(2)
			mockDCSStore.EXPECT().GetCluster().Return(cluster, nil).Times(2)
			// first join
			Expect(lorryClient.JoinMember(context.TODO())).Should(Succeed())
			// second join
			Expect(lorryClient.JoinMember(context.TODO())).Should(Succeed())
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().JoinCurrentMemberToCluster(gomock.Any(), gomock.Any()).Return(fmt.Errorf(msg))
			mockDCSStore.EXPECT().GetCluster().Return(cluster, nil)
			err := lorryClient.JoinMember(context.TODO())
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
		})
	})

	Context("leave member", func() {
		var lorryClient *HTTPClient
		var cluster *dcs.Cluster
		var podName string

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
			podName = "pod-test"

			cluster = &dcs.Cluster{
				HaConfig: &dcs.HaConfig{DeleteMembers: make(map[string]dcs.MemberToDelete)},
				Members:  []dcs.Member{{Name: podName}},
			}
		})

		It("success if leave once", func() {
			mockDBManager.EXPECT().GetCurrentMemberName().Return("pod-test").Times(2)
			mockDBManager.EXPECT().LeaveMemberFromCluster(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			mockDCSStore.EXPECT().GetCluster().Return(cluster, nil)
			mockDCSStore.EXPECT().UpdateHaConfig().Return(nil)
			Expect(lorryClient.LeaveMember(context.TODO())).Should(Succeed())
			Expect(cluster.HaConfig.DeleteMembers).Should(HaveLen(1))
		})

		It("success if leave twice", func() {
			mockDBManager.EXPECT().GetCurrentMemberName().Return("pod-test").Times(4)
			mockDBManager.EXPECT().LeaveMemberFromCluster(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			mockDCSStore.EXPECT().GetCluster().Return(cluster, nil).Times(2)
			mockDCSStore.EXPECT().UpdateHaConfig().Return(nil)
			// first leave
			Expect(lorryClient.LeaveMember(context.TODO())).Should(Succeed())
			Expect(cluster.HaConfig.DeleteMembers).Should(HaveLen(1))
			// second leave
			Expect(lorryClient.LeaveMember(context.TODO())).Should(Succeed())
			Expect(cluster.HaConfig.DeleteMembers).Should(HaveLen(1))
		})

		It("not implemented", func() {
			mockDBManager.EXPECT().GetCurrentMemberName().Return("pod-test").Times(2)
			mockDBManager.EXPECT().LeaveMemberFromCluster(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf(msg))
			mockDCSStore.EXPECT().GetCluster().Return(cluster, nil)
			mockDCSStore.EXPECT().UpdateHaConfig().Return(nil)
			err := lorryClient.LeaveMember(context.TODO())
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
		})
	})
})

func newHTTPServer(resp []byte) (*httptest.Server, int) {
	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writer.WriteHeader(200)
		_, _ = writer.Write(resp)
	}))
	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]
	port, _ := strconv.Atoi(portStr)
	return s, port
}
