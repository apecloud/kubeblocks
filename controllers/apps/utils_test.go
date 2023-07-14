package apps

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var _ = Describe("utils", func() {
	Context("test mergeClusterTemplates", func() {
		It("should succeed", func() {
			cts := []appsv1alpha1.ClusterTemplate{
				{
					Spec: appsv1alpha1.ClusterTemplateSpec{
						ComponentSpecs: []appsv1alpha1.ClusterComponentSpec{
							{
								Name:            "mysql",
								ComponentDefRef: "",
								Replicas:        3,
							},
						},
					},
				},
				{
					Spec: appsv1alpha1.ClusterTemplateSpec{
						ComponentSpecs: []appsv1alpha1.ClusterComponentSpec{
							{
								Name:            "etcd",
								ComponentDefRef: "",
								Replicas:        1,
							},
							{
								Name:            "vtctld",
								ComponentDefRef: "",
								Replicas:        1,
							},
							{
								Name:            "vtconsensus",
								ComponentDefRef: "",
								Replicas:        1,
							},
							{
								Name:            "vtgate",
								ComponentDefRef: "",
								Replicas:        1,
							},
						},
					},
				},
			}
			finalClusterTpl := &appsv1alpha1.ClusterTemplate{
				Spec: appsv1alpha1.ClusterTemplateSpec{
					ComponentSpecs: []appsv1alpha1.ClusterComponentSpec{
						{
							Name:            "mysql",
							ComponentDefRef: "",
							Replicas:        3,
						},
						{
							Name:            "etcd",
							ComponentDefRef: "",
							Replicas:        1,
						},
						{
							Name:            "vtctld",
							ComponentDefRef: "",
							Replicas:        1,
						},
						{
							Name:            "vtconsensus",
							ComponentDefRef: "",
							Replicas:        1,
						},
						{
							Name:            "vtgate",
							ComponentDefRef: "",
							Replicas:        1,
						},
					},
				},
			}
			Expect(mergeClusterTemplates(cts)).Should(BeEquivalentTo(finalClusterTpl))
		})
	})

	Context("test getTemplateNamesFromCF", func() {
		It("should succeed", func() {
			cluster := appsv1alpha1.Cluster{
				Spec: appsv1alpha1.ClusterSpec{
					Mode: "raftGroup",
					Parameters: map[string]string{
						"proxyEnabled": "true",
					},
				},
			}
			cf := appsv1alpha1.ClusterFamily{
				Spec: appsv1alpha1.ClusterFamilySpec{
					ClusterTemplateRefs: []appsv1alpha1.ClusterFamilyTemplateRef{
						{
							Key:         "cluster.spec.mode",
							Value:       "raftGroup",
							TemplateRef: "mysql-raft-template",
						},
						{
							Expression:  "cluster.spec.mode=='raftGroup' && cluster.spec.parameters.proxyEnabled=='true'",
							Value:       "true",
							TemplateRef: "mysql-vitess-template",
						},
						{
							TemplateRef: "mysql-template",
						},
					},
				},
			}
			expectedNames := []string{
				"mysql-raft-template",
				"mysql-vitess-template",
				"mysql-template",
			}
			tplNames, err := getTemplateNamesFromCF(context.TODO(), &cf, &cluster)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(tplNames).Should(BeEquivalentTo(expectedNames))
		})
		It("should not report error if key not exist", func() {
			cluster := appsv1alpha1.Cluster{
				Spec: appsv1alpha1.ClusterSpec{},
			}
			cf := appsv1alpha1.ClusterFamily{
				Spec: appsv1alpha1.ClusterFamilySpec{
					ClusterTemplateRefs: []appsv1alpha1.ClusterFamilyTemplateRef{
						{
							Key:         "cluster.spec.mode",
							Value:       "raftGroup",
							TemplateRef: "mysql-raft-template",
						},
						{
							Expression:  "cluster.spec.mode=='raftGroup' && cluster.spec.parameters.proxyEnabled=='true'",
							Value:       "true",
							TemplateRef: "mysql-vitess-template",
						},
						{
							TemplateRef: "mysql-template",
						},
					},
				},
			}
			expectedNames := []string{
				"mysql-template",
			}
			tplNames, err := getTemplateNamesFromCF(context.TODO(), &cf, &cluster)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(tplNames).Should(BeEquivalentTo(expectedNames))
		})
	})

	Context("test evalCEL", func() {
		It("should succeed", func() {
			cluster := appsv1alpha1.Cluster{
				Spec: appsv1alpha1.ClusterSpec{
					Mode: "raftGroup",
					Parameters: map[string]string{
						"proxyEnabled": "true",
					},
				},
			}
			exp := "cluster.spec.mode"
			res, err := evalCEL(context.TODO(), exp, &cluster)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).Should(Equal("raftGroup"))
			exp = "cluster.spec.mode == 'raftGroup' && cluster.spec.parameters.proxyEnabled == 'true'"
			res, err = evalCEL(context.TODO(), exp, &cluster)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).Should(Equal("true"))
		})
	})
})
