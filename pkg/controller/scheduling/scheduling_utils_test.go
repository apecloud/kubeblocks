/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package scheduling

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Scheduling util test", func() {
	Context("mergeAffinity", func() {
		It("merge all configs", func() {
			affinity1 := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "node-role.kubernetes.io/worker",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
								MatchFields: nil,
							},
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "topology.kubernetes.io/zone",
										Operator: corev1.NodeSelectorOpIn,
										Values: []string{
											"east1",
										},
									},
								},
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: nil,
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"myapp"},
									},
								},
							},
							Namespaces:  nil,
							TopologyKey: "",
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels:      nil,
								MatchExpressions: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: nil,
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: nil,
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"myapp"},
										},
									},
								},
								TopologyKey: "kubernetes.io/hostname",
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels:      nil,
									MatchExpressions: nil,
								},
							},
						},
					},
				},
			}
			affinity2 := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "disktype",
										Operator: corev1.NodeSelectorOpIn,
										Values: []string{
											"hdd",
										},
									},
								},
								MatchFields: nil,
							},
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "topology.kubernetes.io/zone",
										Operator: corev1.NodeSelectorOpIn,
										Values: []string{
											"west1",
										},
									},
								},
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: nil,
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"myapp"},
									},
								},
							},
							Namespaces:  nil,
							TopologyKey: "",
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels:      nil,
								MatchExpressions: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: nil,
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: nil,
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"myapp"},
										},
									},
								},
								TopologyKey: "kubernetes.io/hostname",
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels:      nil,
									MatchExpressions: nil,
								},
							},
						},
					},
				},
			}

			rtn := MergeAffinity(affinity1, affinity2)

			expectMergedAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "disktype",
										Operator: corev1.NodeSelectorOpIn,
										Values: []string{
											"hdd",
										},
									},
								},
								MatchFields: nil,
							},
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "topology.kubernetes.io/zone",
										Operator: corev1.NodeSelectorOpIn,
										Values: []string{
											"west1",
										},
									},
								},
							},
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "node-role.kubernetes.io/worker",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
								MatchFields: nil,
							},
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "topology.kubernetes.io/zone",
										Operator: corev1.NodeSelectorOpIn,
										Values: []string{
											"east1",
										},
									},
								},
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: nil,
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"myapp"},
									},
								},
							},
							Namespaces:  nil,
							TopologyKey: "",
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels:      nil,
								MatchExpressions: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: nil,
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: nil,
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"myapp"},
										},
									},
								},
								TopologyKey: "kubernetes.io/hostname",
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels:      nil,
									MatchExpressions: nil,
								},
							},
						},
					},
				},
			}
			Expect(rtn).Should(Equal(expectMergedAffinity))
		})
		It("merge with nil src", func() {
			var affinity1 *corev1.Affinity
			affinity2 := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "node-role.kubernetes.io/worker",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
								MatchFields: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: nil,
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"myapp"},
									},
								},
							},
							Namespaces:  nil,
							TopologyKey: "",
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels:      nil,
								MatchExpressions: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: nil,
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: nil,
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"myapp"},
										},
									},
								},
								TopologyKey: "kubernetes.io/hostname",
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels:      nil,
									MatchExpressions: nil,
								},
							},
						},
					},
				},
			}

			rtn := MergeAffinity(affinity1, affinity2)

			expectMergedAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "node-role.kubernetes.io/worker",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
								MatchFields: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: nil,
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"myapp"},
									},
								},
							},
							Namespaces:  nil,
							TopologyKey: "",
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels:      nil,
								MatchExpressions: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: nil,
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: nil,
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"myapp"},
										},
									},
								},
								TopologyKey: "kubernetes.io/hostname",
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels:      nil,
									MatchExpressions: nil,
								},
							},
						},
					},
				},
			}
			Expect(rtn).Should(Equal(expectMergedAffinity))
		})
		It("merge with nil dst", func() {
			affinity1 := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "node-role.kubernetes.io/worker",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
								MatchFields: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: nil,
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"myapp"},
									},
								},
							},
							Namespaces:  nil,
							TopologyKey: "",
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels:      nil,
								MatchExpressions: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: nil,
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: nil,
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"myapp"},
										},
									},
								},
								TopologyKey: "kubernetes.io/hostname",
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels:      nil,
									MatchExpressions: nil,
								},
							},
						},
					},
				},
			}
			var affinity2 *corev1.Affinity = nil

			rtn := MergeAffinity(affinity1, affinity2)

			expectMergedAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "node-role.kubernetes.io/worker",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
								MatchFields: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: nil,
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"myapp"},
									},
								},
							},
							Namespaces:  nil,
							TopologyKey: "",
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels:      nil,
								MatchExpressions: nil,
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: nil,
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: nil,
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"myapp"},
										},
									},
								},
								TopologyKey: "kubernetes.io/hostname",
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels:      nil,
									MatchExpressions: nil,
								},
							},
						},
					},
				},
			}
			Expect(rtn).Should(Equal(expectMergedAffinity))
		})
	})
})
