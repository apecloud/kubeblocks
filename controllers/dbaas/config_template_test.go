/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dbaas

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("tpl template", func() {

	var (
		podTemplate *corev1.PodTemplateSpec
		cfgTemplate []dbaasv1alpha1.ConfigTemplate
		component   *Component
		group       *RoleGroup
	)

	const (
		MYSQL_CFG_NAME        = "my.cfg"
		MYSQL_CFG_TMP_CONTEXT = `
[mysqld]
loose_query_cache_type          = OFF
loose_query_cache_size          = 0
loose_innodb_thread_concurrency = 0
loose_concurrent_insert         = 0
loose_gts_lease                 = 2000
loose_log_bin_use_v1_row_events = off
loose_binlog_checksum           = crc32

#test
cluster_name = {{ .Cluster.Name }}
cluster_namespace = {{ .Cluster.Namespace }}
component_name = {{ .Component.Name }}
component_replica = {{ .Component.Replicas }}

{{- $buffer_pool_size_tmp := default 2147483648 .Component.Resource }}
innodb_buffer_pool_size = {{ $buffer_pool_size_tmp }}
loose_rds_audit_log_buffer_size = {{ div $buffer_pool_size_tmp 100 }}
loose_innodb_replica_log_parse_buf_size = {{ div $buffer_pool_size_tmp 10 }}
loose_innodb_primary_flush_max_lsn_lag =  {{ div $buffer_pool_size_tmp 11 }}
`
		MYSQL_CFG_RENDERED_CONTEXT = `
[mysqld]
loose_query_cache_type          = OFF
loose_query_cache_size          = 0
loose_innodb_thread_concurrency = 0
loose_concurrent_insert         = 0
loose_gts_lease                 = 2000
loose_log_bin_use_v1_row_events = off
loose_binlog_checksum           = crc32

#test
cluster_name = my_test
cluster_namespace = default
component_name = replicasets
component_replica = 5
innodb_buffer_pool_size = 2147483648
loose_rds_audit_log_buffer_size = 21474836
loose_innodb_replica_log_parse_buf_size = 214748364
loose_innodb_primary_flush_max_lsn_lag =  195225786
`
	)

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		podTemplate = &corev1.PodTemplateSpec{}
		component = &Component{
			ClusterDefName: "mysql-three-node-definition",
			ClusterType:    "state.mysql-8",
			Name:           "replicasets",
			Type:           "replicasets",
		}
		group = &RoleGroup{
			Name:     "mysql-a",
			Type:     "primary",
			Replicas: 5,
		}
		cfgTemplate = []dbaasv1alpha1.ConfigTemplate{
			{
				Name:       "mysql-config-8.0.2",
				VolumeName: "config1",
			},
		}
	})

	// for test GetContainerWithVolumeMount
	Context("ConfigTemplateBuilder sample test", func() {
		It("test render", func() {
			cfgBuilder := NewCfgTemplateBuilder("my_test", "default")

			Expect(cfgBuilder.InjectBuiltInObjectsAndFunctions(*podTemplate, cfgTemplate, component, group)).To(BeNil())

			rendered, err := cfgBuilder.Render(map[string]string{
				MYSQL_CFG_NAME: MYSQL_CFG_TMP_CONTEXT,
			})

			// Debug
			fmt.Print(rendered[MYSQL_CFG_NAME])

			Expect(err).To(BeNil())
			Expect(rendered[MYSQL_CFG_NAME]).To(Equal(MYSQL_CFG_RENDERED_CONTEXT))
		})
	})

})
