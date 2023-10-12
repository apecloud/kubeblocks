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

package flags

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	clientfake "k8s.io/client-go/rest/fake"
	"k8s.io/kube-openapi/pkg/validation/spec"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

const singleFlags = `{
  "$schema": "http://json-schema.org/schema#",
  "type": "object",
  "properties": {
    "version": {
      "title": "Version",
      "description": "Cluster version.",
      "type": "string",
      "default": "ac-mysql-8.0.30"
    },
    "mode": {
      "title": "Mode",
      "description": "Cluster topology mode.",
      "type": "string",
      "default": "standalone",
      "enum": [
        "standalone",
        "raftGroup"
      ]
    },
    "replicas": {
      "title": "Replicas",
      "description": "The number of replicas, for standalone mode, the replicas is 1, for raftGroup mode, the default replicas is 3.",
      "type": "integer",
      "default": 1,
      "minimum": 1,
      "maximum": 5
    },
    "cpu": {
      "title": "CPU",
      "description": "CPU cores.",
      "type": [
        "number",
        "string"
      ],
      "default": 0.5,
      "minimum": 0.5,
      "maximum": 64,
      "multipleOf": 0.5
    },
    "memory": {
      "title": "Memory(Gi)",
      "description": "Memory, the unit is Gi.",
      "type": [
        "number",
        "string"
      ],
      "default": 0.5,
      "minimum": 0.5,
      "maximum": 1000
    },
    "storage": {
      "title": "Storage(Gi)",
      "description": "Storage size, the unit is Gi.",
      "type": [
        "number",
        "string"
      ],
      "default": 20,
      "minimum": 1,
      "maximum": 10000
    },
    "proxyEnabled": {
      "title": "Proxy",
      "description": "Enable proxy or not.",
      "type": "boolean",
      "default": false
    }
  }
}
`

// objectFlags is the schema.json of risingwave-cluster
const objectFlags = `
{
    "$schema": "http://json-schema.org/schema#",
    "type": "object",
    "properties": {
        "risingwave": {
            "type": "object",
            "properties": {
                "metaStore": {
                    "type": "object",
                    "properties": {
                        "etcd": {
                            "type": "object",
                            "properties": {
                                "endpoints": {
                                    "title": "ETCD EndPoints",
                                    "description": "Specify ETCD cluster endpoints of the form host:port",
                                    "type": "string",
                                    "pattern": "^.+:\\d+$"
                                }
                            }
                        }
                    }
                },
                "stateStore": {
                    "type": "object",
                    "properties": {
                        "s3": {
                            "type": "object",
                            "properties": {
                                "authentication": {
                                    "type": "object",
                                    "properties": {
                                        "accessKey": {
                                            "$ref": "#/definitions/nonEmptyString",
                                            "description": "Specify the S3 access key."
                                        },
                                        "secretAccessKey": {
                                            "$ref": "#/definitions/nonEmptyString",
                                            "description": "Specify the S3 secret access key."
                                        }
                                    }
                                },
                                "bucket": {
                                    "$ref": "#/definitions/nonEmptyString",
                                    "description": "Specify the S3 bucket."
                                },
                                "endpoint": {
                                    "$ref": "#/definitions/nonEmptyString",
                                    "description": "Specify the S3 endpoint."
                                },
                                "region": {
                                    "$ref": "#/definitions/nonEmptyString",
                                    "description": "Specify the S3 region."
                                }
                            }
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "nonEmptyString": {
            "type": "string",
            "minLength": 1
        }
    }
}
`

/*
	objectArrayFlags yaml example:

apiVersion: 1
name: myapp
servers:
  - name: server1
    address: 192.168.1.10
    port: 8080
  - name: server2
    address: 192.168.1.20
    port: 8081
*/
const objectArrayFlags = `{
  "$schema": "http://json-schema.org/schema#",
  "type": "object",
  "properties": {
    "apiVersion": {
      "type": "integer"
    },
    "name": {
      "type": "string"
    },
    "servers": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string"
          },
          "address": {
            "type": "string",
            "format": "ipv4"
          },
          "port": {
            "type": "integer",
            "minimum": 1,
            "maximum": 65535
          }
        },
        "required": ["name", "address", "port"]
      }
    }
  },
  "required": ["apiVersion", "name"]
}
`

/*
	simpleArrayFlags yaml example:

name:
- alal
- jack
*/
const simpleArrayFlags = `
{
  "$schema": "http://json-schema.org/schema#",
  "type": "object",
  "properties": {
    "name": {
      "type": "array",
      "items": {
        "type": "string"
      }
    }
  }
}
`

/*
	arrayWithArray yaml example:

data:
  - name: John
    age: 30
    hobbies:
  - Reading
  - Swimming
  - name: Alice
    age: 28
    hobbies:
  - Painting
  - Hiking
  - name: Bob
    age: 35
    hobbies:
  - Cycling
*/
const arrayWithArray = `{
    "$schema": "https://json-schema.org/draft/2019-09/schema",
    "title": "Root Schema",
    "type": "object",
    "properties": {
        "data": {
            "title": "The data Schema",
            "type": "array",
            "items": {
                "title": "A Schema",
                "type": "object",
                "properties": {
                    "name": {
                        "title": "The name Schema",
                        "type": "string"
                    },
                    "age": {
                        "title": "The age Schema",
                        "type": "number"
                    },
                    "hobbies": {
                        "title": "The hobbies Schema",
                        "type": "array",
                        "items": {
                            "title": "A Schema",
                            "type": "string"
                        }
                    }
                }
            }
        }
    }
}`

var _ = Describe("flag", func() {
	var cmd *cobra.Command
	var schema *spec.Schema
	testCast := []struct {
		description   string
		rawSchemaJSON string
		flags         []string
		success       bool
	}{
		{"test single flags",
			singleFlags,
			[]string{
				"version", "mode", "replicas", "cpu", "memory", "storage", "proxy-enabled",
			},
			true,
		}, {
			"test complex flags",
			objectFlags,
			[]string{"risingwave.meta-store.etcd.endpoints",
				"risingwave.state-store.s3.authentication.access-key",
				"risingwave.state-store.s3.authentication.secret-access-key",
				"risingwave.state-store.s3.bucket",
				"risingwave.state-store.s3.endpoint",
				"risingwave.state-store.s3.region",
			},
			true,
		}, {
			"test object array flags",
			objectArrayFlags,
			[]string{"api-version", "name", "servers.name", "servers.address", "servers.port"},
			true,
		}, {
			"test simple array flags",
			simpleArrayFlags,
			[]string{"name"},
			true,
		}, {
			"test array with array object",
			arrayWithArray,
			nil,
			false,
		},
	}
	It("test BuildFlagsBySchema", func() {
		for i := range testCast {
			cmd = &cobra.Command{}
			schema = &spec.Schema{}
			By(testCast[i].description, func() {
				Expect(schema.UnmarshalJSON([]byte(testCast[i].rawSchemaJSON))).Should(Succeed())
				if testCast[i].success {
					Expect(BuildFlagsBySchema(cmd, schema)).Should(Succeed())
					for _, flag := range testCast[i].flags {
						Expect(cmd.Flags().Lookup(flag)).ShouldNot(BeNil())
					}
				} else {
					Expect(BuildFlagsBySchema(cmd, schema)).Should(HaveOccurred())
				}
			})
		}
	})

	Context("test autoCompleteClusterComponent ", func() {
		var tf *cmdtesting.TestFactory
		var flag string
		BeforeEach(func() {
			tf = cmdtesting.NewTestFactory()
			fakeCluster := testing.FakeCluster("fake-cluster", "")
			tf.FakeDynamicClient = testing.FakeDynamicClient(fakeCluster)
			tf.Client = &clientfake.RESTClient{}
			cmd = &cobra.Command{
				Use:   "test",
				Short: "test for autoComplete",
			}
			cmd.Flags().StringVar(&flag, "component", "", "test")
		})

		AfterEach(func() {
			tf.Cleanup()
		})

		It("test build autoCompleteClusterComponent", func() {
			Expect(autoCompleteClusterComponent(cmd, tf, "component")).Should(Succeed())
		})
	})
})
