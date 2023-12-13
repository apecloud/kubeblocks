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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Job Utils Test", func() {

	Context("Job Utils Test function", func() {
		const (
			clusterDefName     = "test-clusterdef"
			clusterVersionName = "test-clusterversion"
			clusterName        = "test-cluster"
			mysqlCompDefName   = "replicasets"
			mysqlCompName      = "mysql"
			labelKey           = "test-label"
		)

		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		createJob := func(name string, keys ...string) *batchv1.Job {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: name,
					Namespace:    testCtx.DefaultNamespace,
					Labels:       map[string]string{},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "kubeblocks",
									Image: "busybox",
								},
							},
						},
					},
				},
			}
			for _, key := range keys {
				job.ObjectMeta.Labels[key] = constant.AppName
			}
			return job
		}

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponentVersion(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				GetObject()
		})

		It("should work as expected with various inputs", func() {
			By("create watch obj with label=" + labelKey)
			job := createJob("test-job1", labelKey)
			Expect(k8sClient.Create(ctx, job)).ToNot(HaveOccurred())

			jobList, err := GetJobWithLabels(ctx, k8sClient, cluster, map[string]string{labelKey: constant.AppName})
			Expect(err).Should(Succeed())
			Expect(len(jobList)).To(Equal(1))

			By("test job complete false")
			Expect(CheckJobSucceed(ctx, k8sClient, cluster, job.Name)).Should(HaveOccurred())

			By("mock job is succeed")
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(job), func(job *batchv1.Job) {
				job.Status.Conditions = []batchv1.JobCondition{
					{
						Type: batchv1.JobComplete,
					},
				}
			})()).ShouldNot(HaveOccurred())
			Expect(CheckJobSucceed(ctx, k8sClient, cluster, job.Name)).ShouldNot(HaveOccurred())

			By("delete job by name")
			Expect(CleanJobByName(ctx, k8sClient, cluster, job.Name)).ShouldNot(HaveOccurred())

			By("create watch obj with label=" + labelKey)
			job2 := createJob("test-job2", labelKey)
			Expect(k8sClient.Create(ctx, job2)).ToNot(HaveOccurred())

			By("delete job with label")
			Expect(CleanJobWithLabels(ctx, k8sClient, cluster, map[string]string{labelKey: constant.AppName})).ShouldNot(HaveOccurred())
		})
	})
})
