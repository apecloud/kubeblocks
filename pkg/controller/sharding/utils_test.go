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

package sharding

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("sharding", func() {
	const (
		seed = 1670750000
	)

	var (
		// first 10 ids
		ids = []string{"bvj", "g7c", "gpz", "w8b", "dng", "rhk", "rzn", "ql8", "929", "99n"}
	)

	Context("build sharding comp specs", func() {
		const (
			clusterName       = "test-cluster"
			shardingName      = "sharding"
			shardTemplateName = "shard-template"
		)

		BeforeEach(func() {
			rand.Seed(seed)
		})

		It("precheck - shards", func() {
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				ShardTemplates: []appsv1.ShardTemplate{
					{
						Name:     fmt.Sprintf("%s-0", shardTemplateName),
						Shards:   ptr.To[int32](2),
						Replicas: ptr.To[int32](5),
					},
					{
						Name:     fmt.Sprintf("%s-1", shardTemplateName),
						Shards:   ptr.To[int32](2),
						Replicas: ptr.To[int32](5),
					},
				},
			}

			_, err := buildShardingCompSpecs(clusterName, sharding, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("the sum of shards in shard templates is greater than the total shards"))
		})

		It("precheck - shard ids", func() {
			runningComp1 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				ShardTemplates: []appsv1.ShardTemplate{
					{
						Name:     fmt.Sprintf("%s-0", shardTemplateName),
						Shards:   ptr.To[int32](1),
						ShardIDs: []string{ids[9]},
						Replicas: ptr.To[int32](5),
					},
					{
						Name:     fmt.Sprintf("%s-1", shardTemplateName),
						Shards:   ptr.To[int32](1),
						ShardIDs: []string{ids[9]},
						Replicas: ptr.To[int32](5),
					},
				},
			}

			_, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp1})
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring(fmt.Sprintf("shard id %s is duplicated", ids[9])))
		})

		It("provision", func() {
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, nil)
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(1))
			Expect(specs).Should(HaveKey(defaultShardTemplateName))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(2))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[0]))
			Expect(specs[defaultShardTemplateName][0].Replicas).Should(Equal(int32(3)))
			Expect(specs[defaultShardTemplateName][1].Name).Should(HaveSuffix(ids[1]))
			Expect(specs[defaultShardTemplateName][1].Replicas).Should(Equal(int32(3)))
		})

		It("provision with template", func() {
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				ShardTemplates: []appsv1.ShardTemplate{
					{
						Name:     shardTemplateName,
						Shards:   ptr.To[int32](1),
						Replicas: ptr.To[int32](5),
					},
				},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, nil)
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(2))
			Expect(specs).Should(And(HaveKey(defaultShardTemplateName), HaveKey(shardTemplateName)))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[0]))
			Expect(specs[defaultShardTemplateName][0].Replicas).Should(Equal(int32(3)))
			Expect(specs[shardTemplateName]).Should(HaveLen(1))
			Expect(specs[shardTemplateName][0].Name).Should(HaveSuffix(ids[1]))
			Expect(specs[shardTemplateName][0].Replicas).Should(Equal(int32(5)))
		})

		It("provision with offline", func() {
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				ShardTemplates: []appsv1.ShardTemplate{
					{
						Name:     shardTemplateName,
						Shards:   ptr.To[int32](1),
						Replicas: ptr.To[int32](5),
					},
				},
				Offline: []string{fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[0])},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, nil)
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(2))
			Expect(specs).Should(And(HaveKey(defaultShardTemplateName), HaveKey(shardTemplateName)))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[1])) // skip offline shard of ids[0]
			Expect(specs[defaultShardTemplateName][0].Replicas).Should(Equal(int32(3)))
			Expect(specs[shardTemplateName]).Should(HaveLen(1))
			Expect(specs[shardTemplateName][0].Name).Should(HaveSuffix(ids[2]))
			Expect(specs[shardTemplateName][0].Replicas).Should(Equal(int32(5)))
		})

		PIt("merge with shard template", func() {
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				ShardTemplates: []appsv1.ShardTemplate{
					{
						Name:     shardTemplateName,
						Shards:   ptr.To[int32](1),
						Replicas: ptr.To[int32](5),
					},
				},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, nil)
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(2))
			Expect(specs).Should(HaveKey(defaultShardTemplateName))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Replicas).Should(Equal(int32(3)))
			Expect(specs).Should(HaveKey(shardTemplateName))
			Expect(specs[shardTemplateName]).Should(HaveLen(1))
			Expect(specs[shardTemplateName][0].Replicas).Should(Equal(int32(5)))
		})

		It("scale out", func() {
			runningComp := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp})
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(1))
			Expect(specs).Should(HaveKey(defaultShardTemplateName))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(2))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[9]))
			Expect(specs[defaultShardTemplateName][1].Name).Should(HaveSuffix(ids[0]))
		})

		It("scale out - shard template", func() {
			runningComp := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				ShardTemplates: []appsv1.ShardTemplate{
					{
						Name:     shardTemplateName,
						Shards:   ptr.To[int32](1),
						Replicas: ptr.To[int32](5),
					},
				},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp})
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(2))
			Expect(specs).Should(And(HaveKey(defaultShardTemplateName), HaveKey(shardTemplateName)))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[9]))
			Expect(specs[defaultShardTemplateName][0].Replicas).Should(Equal(int32(3)))
			Expect(specs[shardTemplateName]).Should(HaveLen(1))
			Expect(specs[shardTemplateName][0].Name).Should(HaveSuffix(ids[0]))
			Expect(specs[shardTemplateName][0].Replicas).Should(Equal(int32(5)))
		})

		It("scale in", func() {
			runningComp1 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[8]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			runningComp2 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 1,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp1, runningComp2})
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(1))
			Expect(specs).Should(HaveKey(defaultShardTemplateName))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[8])) // runningComp1.Name < runningComp2.Name
		})

		It("scale in - shard template", func() {
			runningComp1 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[8]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: shardTemplateName, // shard template
					},
				},
			}
			runningComp2 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 1,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				// the shard template is removed
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp1, runningComp2})
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(1))
			Expect(specs).Should(HaveKey(defaultShardTemplateName))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[9])) // runningComp1 belongs to the shard template
		})

		It("scale in - offline", func() {
			runningComp1 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[8]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			runningComp2 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 1,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				Offline: []string{runningComp1.Name},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp1, runningComp2})
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(1))
			Expect(specs).Should(HaveKey(defaultShardTemplateName))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[9])) // runningComp1 has been offline
		})

		It("scale in & out", func() {
			runningComp1 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[8]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			runningComp2 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2, // still 2 shards
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				Offline: []string{runningComp1.Name}, // but shard 1 is offline
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp1, runningComp2})
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(1))
			Expect(specs).Should(HaveKey(defaultShardTemplateName))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(2))
			Expect(specs[defaultShardTemplateName][0].Name).Should(Or(HaveSuffix(ids[9]), HaveSuffix(ids[0])))
			Expect(specs[defaultShardTemplateName][1].Name).Should(Or(HaveSuffix(ids[9]), HaveSuffix(ids[0])))
		})

		It("scale in & out - shard template", func() {
			runningComp1 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[8]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			runningComp2 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: shardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2, // still 2 shards
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				ShardTemplates: []appsv1.ShardTemplate{
					{
						Name:     shardTemplateName,
						Shards:   ptr.To[int32](1),
						Replicas: ptr.To[int32](5),
					},
				},
				Offline: []string{runningComp1.Name, runningComp2.Name}, // both shard 1 and shard 2 are offline
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp1, runningComp2})
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(2))
			Expect(specs).Should(And(HaveKey(defaultShardTemplateName), HaveKey(shardTemplateName)))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[0]))
			Expect(specs[defaultShardTemplateName][0].Replicas).Should(Equal(int32(3)))
			Expect(specs[shardTemplateName]).Should(HaveLen(1))
			Expect(specs[shardTemplateName][0].Name).Should(HaveSuffix(ids[1]))
			Expect(specs[shardTemplateName][0].Replicas).Should(Equal(int32(5)))
		})

		It("take over", func() {
			runningComp1 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[8]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			runningComp2 := appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-%s", clusterName, shardingName, ids[9]),
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			}
			sharding := &appsv1.ClusterSharding{
				Name:   shardingName,
				Shards: 2,
				Template: appsv1.ClusterComponentSpec{
					Replicas: 3,
				},
				ShardTemplates: []appsv1.ShardTemplate{
					{
						Name:     shardTemplateName,
						Shards:   ptr.To[int32](1),
						ShardIDs: []string{ids[9]}, // take over
						Replicas: ptr.To[int32](5),
					},
				},
			}

			specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{runningComp1, runningComp2})
			Expect(err).Should(Succeed())

			Expect(len(specs)).Should(BeEquivalentTo(2))
			Expect(specs).Should(And(HaveKey(defaultShardTemplateName), HaveKey(shardTemplateName)))
			Expect(specs[defaultShardTemplateName]).Should(HaveLen(1))
			Expect(specs[defaultShardTemplateName][0].Name).Should(HaveSuffix(ids[8]))
			Expect(specs[defaultShardTemplateName][0].Replicas).Should(Equal(int32(3)))
			Expect(specs[shardTemplateName]).Should(HaveLen(1))
			Expect(specs[shardTemplateName][0].Name).Should(HaveSuffix(ids[9]))
			Expect(specs[shardTemplateName][0].Replicas).Should(Equal(int32(5)))
		})
	})
})
