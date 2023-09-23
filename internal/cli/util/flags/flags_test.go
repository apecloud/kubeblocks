package flags

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/kube-openapi/pkg/validation/spec"
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

const complexFlags = `
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

// The helm chart values.yaml corresponding to 'wrongFlags' with an array type configuration
/*
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
const wrongFlags = `{
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
  "required": ["apiVersion", "name", "servers"]
}

`

var _ = Describe("flag", func() {
	var cmd *cobra.Command
	var schema *spec.Schema
	testCast := []struct {
		describtion   string
		rawSchemaJson string
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
			complexFlags,
			[]string{"risingwave.meta-store.etcd.endpoints",
				"risingwave.state-store.s3.authentication.access-key",
				"risingwave.state-store.s3.authentication.secret-access-key",
				"risingwave.state-store.s3.bucket",
				"risingwave.state-store.s3.endpoint",
				"risingwave.state-store.s3.region",
			},
			true,
		}, {
			"test wrong flags",
			wrongFlags,
			nil,
			false,
		},
	}
	Context("test BuildFlagsBySchema", func() {
		BeforeEach(func() {
			cmd = &cobra.Command{}
			schema = &spec.Schema{}
		})
		for i := range testCast {
			It(testCast[i].describtion, func() {
				Expect(schema.UnmarshalJSON([]byte(testCast[i].rawSchemaJson))).Should(Succeed())
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

})
