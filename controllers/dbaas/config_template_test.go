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
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InsClassType struct {
	memSize int64
	cpu     int
	// recommended buffer size
	bufferSize string

	maxBufferSize int
}

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
component_replica = {{ .RoleGroup.Replicas }}

{{- $test_value := call_buffer_size_by_resource ( index .PodSpec.Containers 0 ) }}
{{ if $test_value -}}
test_size = {{ $test_value }}
{{- else }}
test_size = 128M
{{ end -}}

{{ $buffer_pool_size_tmp := 2147483648 -}}
{{ if .ComponentResource -}}
{{ $buffer_pool_size_tmp = .ComponentResource.MemorySize }}
{{- end }}
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
test_size = 4096M
innodb_buffer_pool_size = 8589934592
loose_rds_audit_log_buffer_size = 85899345
loose_innodb_replica_log_parse_buf_size = 858993459
loose_innodb_primary_flush_max_lsn_lag =  780903144
`
	)

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		podTemplate = &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "mytest",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								MountPath: "/data/mysql",
							},
							{
								Name:      "log",
								MountPath: "/log/mysql",
							},
						},
						Env: []corev1.EnvVar{
							{
								Name:  "t1",
								Value: "value1",
							},
							{
								Name:  "t2",
								Value: "value2",
							},
							{
								Name:  "a",
								Value: "b",
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "mysql",
								ContainerPort: 3356,
								Protocol:      "TCP",
							},
							{
								Name:          "paxos",
								ContainerPort: 3356,
								Protocol:      "TCP",
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceMemory: resource.MustParse("8Gi"),
								corev1.ResourceCPU:    resource.MustParse("4"),
							},
						},
					},
					{
						Name: "invalid_contaienr",
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "cluster_name_for_test",
								},
							},
						},
					},
				},
			},
		}
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
			cfgBuilder := NewCfgTemplateBuilder(
				"my_test",
				"default",
				&dbaasv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my_test",
						Namespace: "default",
					},
				},
				nil,
			)

			Expect(cfgBuilder.InjectBuiltInObjectsAndFunctions(podTemplate, cfgTemplate, component, group)).Should(BeNil())

			cfgBuilder.componentValues.Resource = &ResourceDefinition{
				MemorySize: 8 * 1024 * 1024 * 1024,
				CoreNum:    4,
			}

			rendered, err := cfgBuilder.Render(map[string]string{
				MYSQL_CFG_NAME: MYSQL_CFG_TMP_CONTEXT,
			})

			// Debug
			fmt.Println(rendered[MYSQL_CFG_NAME])
			fmt.Printf("%s\n", MYSQL_CFG_RENDERED_CONTEXT)

			Expect(err).Should(BeNil())
			Expect(rendered[MYSQL_CFG_NAME]).Should(Equal(MYSQL_CFG_RENDERED_CONTEXT))
		})
		It("test built-in function", func() {
			cfgBuilder := NewCfgTemplateBuilder(
				"my_test",
				"default",
				&dbaasv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my_test",
						Namespace: "default",
					},
				},
				nil,
			)

			Expect(cfgBuilder.InjectBuiltInObjectsAndFunctions(podTemplate, cfgTemplate, component, group)).Should(BeNil())

			rendered, err := cfgBuilder.Render(map[string]string{
				"a":                 "{{ get_volume_path_by_name ( index .PodSpec.Containers 0 ) \"log\" }}",
				"b":                 "{{ get_volume_path_by_name ( index .PodSpec.Containers 0 ) \"data\" }}",
				"c":                 "{{ ( get_port_by_name ( index .PodSpec.Containers 0 ) \"mysql\" ).ContainerPort }}",
				"d":                 "{{ call_buffer_size_by_resource ( index .PodSpec.Containers 0 ) }}",
				"e":                 "{{ get_arg_by_name ( index .PodSpec.Containers 0 ) \"User\" }}",
				"f":                 "{{ get_volume_path_by_name ( get_container_by_name .PodSpec.Containers \"mytest\") \"data\" }}",
				"i":                 "{{ get_env_by_name ( index .PodSpec.Containers 0 ) \"a\" }}",
				"j":                 "{{ ( get_pvc_by_name .PodSpec.Volumes \"config\" ).ConfigMap.Name }}",
				"invalid_volume":    "{{ get_volume_path_by_name ( index .PodSpec.Containers 0 ) \"invalid\" }}",
				"invalid_port":      "{{ get_port_by_name ( index .PodSpec.Containers 0 ) \"invalid\" }}",
				"invalid_container": "{{ get_container_by_name .PodSpec.Containers  \"invalid\" }}",
				"invalid_resource":  "{{ call_buffer_size_by_resource ( index .PodSpec.Containers 1 ) }}",
				"invalid_env":       "{{ get_env_by_name ( index .PodSpec.Containers 0 ) \"invalid\" }}",
				"invalid_pvc":       "{{ get_pvc_by_name .PodSpec.Volumes \"invalid\" }}",
			})

			Expect(err).Should(BeNil())
			// for test volumeMounts
			Expect(rendered["a"]).Should(Equal("/log/mysql"))
			// for test volumeMounts
			Expect(rendered["b"]).Should(Equal("/data/mysql"))
			// for test port
			Expect(rendered["c"]).Should(Equal("3356"))
			// for test resource
			Expect(rendered["d"]).Should(Equal("4096M"))
			// for test args
			Expect(rendered["e"]).Should(Equal(""))
			// for test volumeMounts
			Expect(rendered["f"]).Should(Equal("/data/mysql"))
			// for test env
			Expect(rendered["i"]).Should(Equal("b"))
			// for test volume
			Expect(rendered["j"]).Should(Equal("cluster_name_for_test"))
			Expect(rendered["invalid_volume"]).Should(Equal(""))
			Expect(rendered["invalid_port"]).Should(Equal("nil"))
			Expect(rendered["invalid_container"]).Should(Equal("nil"))
			Expect(rendered["invalid_env"]).Should(Equal(""))
			Expect(rendered["invalid_pvc"]).Should(Equal("nil"))
		})
	})

	Context("calMysqlPoolSizeByResource test", func() {
		It("mysql test", func() {
			Expect(calMysqlPoolSizeByResource(nil, false)).Should(Equal("128M"))

			Expect(calMysqlPoolSizeByResource(nil, true)).Should(Equal("128M"))

			// for small instance class
			Expect(calMysqlPoolSizeByResource(&ResourceDefinition{
				MemorySize: 1024 * 1024 * 1024,
				CoreNum:    1,
			}, false)).Should(Equal("128M"))

			Expect(calMysqlPoolSizeByResource(&ResourceDefinition{
				MemorySize: 2 * 1024 * 1024 * 1024,
				CoreNum:    2,
			}, false)).Should(Equal("256M"))

			// for shard
			Expect(calMysqlPoolSizeByResource(&ResourceDefinition{
				MemorySize: 2 * 1024 * 1024 * 1024,
				CoreNum:    2,
			}, true)).Should(Equal("1024M"))

			insClassTest := []InsClassType{
				// for 2 core
				{
					memSize:       4,
					cpu:           2,
					bufferSize:    "1024M",
					maxBufferSize: 1024,
				},
				{
					memSize:       8,
					cpu:           2,
					bufferSize:    "4096M",
					maxBufferSize: 4096,
				},
				{
					memSize:       16,
					cpu:           2,
					bufferSize:    "9216M",
					maxBufferSize: 10240,
				},
				// for 4 core
				{
					memSize:       8,
					cpu:           4,
					bufferSize:    "4096M",
					maxBufferSize: 4096,
				},
				{
					memSize:       16,
					cpu:           4,
					bufferSize:    "9216M",
					maxBufferSize: 10240,
				},
				{
					memSize:       32,
					cpu:           4,
					bufferSize:    "21504M",
					maxBufferSize: 22528,
				},
				// for 8 core
				{
					memSize:       16,
					cpu:           8,
					bufferSize:    "9216M",
					maxBufferSize: 10240,
				},
				{
					memSize:       32,
					cpu:           8,
					bufferSize:    "21504M",
					maxBufferSize: 22528,
				},
				{
					memSize:       64,
					cpu:           8,
					bufferSize:    "45056M",
					maxBufferSize: 48128,
				},
				// for 12 core
				{
					memSize:       24,
					cpu:           12,
					bufferSize:    "15360M",
					maxBufferSize: 16384,
				},
				{
					memSize:       48,
					cpu:           12,
					bufferSize:    "33792M",
					maxBufferSize: 35840,
				},
				{
					memSize:       96,
					cpu:           12,
					bufferSize:    "69632M",
					maxBufferSize: 73728,
				},
				// for 16 core
				{
					memSize:       32,
					cpu:           16,
					bufferSize:    "21504M",
					maxBufferSize: 22528,
				},
				{
					memSize:       64,
					cpu:           16,
					bufferSize:    "45056M",
					maxBufferSize: 48128,
				},
				{
					memSize:       128,
					cpu:           16,
					bufferSize:    "93184M",
					maxBufferSize: 99328,
				},
				// for 24 core
				{
					memSize:       48,
					cpu:           24,
					bufferSize:    "32768M",
					maxBufferSize: 34816,
				},
				{
					memSize:       96,
					cpu:           24,
					bufferSize:    "69632M",
					maxBufferSize: 73728,
				},
				{
					memSize:       192,
					cpu:           24,
					bufferSize:    "140288M",
					maxBufferSize: 149504,
				},
				// for 32 core
				{
					memSize:       64,
					cpu:           32,
					bufferSize:    "45056M",
					maxBufferSize: 47104,
				},
				{
					memSize:       128,
					cpu:           32,
					bufferSize:    "93184M",
					maxBufferSize: 99328,
				},
				{
					memSize:       256,
					cpu:           32,
					bufferSize:    "188416M",
					maxBufferSize: 200704,
				},
				// for 52 core
				{
					memSize:       96,
					cpu:           52,
					bufferSize:    "67584M",
					maxBufferSize: 72704,
				},
				{
					memSize:       192,
					cpu:           52,
					bufferSize:    "140288M",
					maxBufferSize: 149504,
				},
				{
					memSize:       384,
					cpu:           52,
					bufferSize:    "283648M",
					maxBufferSize: 302080,
				},
				// for 64 core
				{
					memSize:       256,
					cpu:           64,
					bufferSize:    "188416M",
					maxBufferSize: 200704,
				},
				{
					memSize:       512,
					cpu:           64,
					bufferSize:    "378880M",
					maxBufferSize: 403456,
				},
				// for 102
				{
					memSize:       768,
					cpu:           102,
					bufferSize:    "569344M",
					maxBufferSize: 607232,
				},
				// for 104 core
				{
					memSize:       192,
					cpu:           104,
					bufferSize:    "138240M",
					maxBufferSize: 147456,
				},
				{
					memSize:       384,
					cpu:           104,
					bufferSize:    "282624M",
					maxBufferSize: 302080,
				},
			}

			// for debug
			for _, r := range insClassTest {
				fmt.Printf("cal : %s, expect [%v] [%s]\n", calMysqlPoolSizeByResource(&ResourceDefinition{
					MemorySize: r.memSize * 1024 * 1024 * 1024, // 4G
					CoreNum:    r.cpu,                          // 2core
				}, false), r, r.bufferSize)
			}

			for _, r := range insClassTest {
				ret := calMysqlPoolSizeByResource(&ResourceDefinition{
					MemorySize: r.memSize * 1024 * 1024 * 1024, // 4G
					CoreNum:    r.cpu,                          // 2core
				}, false)
				Expect(ret).Should(Equal(r.bufferSize))
				Expect(strconv.ParseInt(strings.Trim(ret, "M"), 10, 64)).Should(BeNumerically("<=", r.maxBufferSize))
			}
		})
	})

})
