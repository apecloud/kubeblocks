/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package instanceset

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

var _ = Describe("revision util test", func() {
	Context("NewRevision function", func() {
		It("should work well", func() {
			stsJSON := `
{
    "apiVersion": "workloads.kubeblocks.io/v1alpha1",
    "kind": "InstanceSet",
    "metadata": {
        "annotations": {
            "config.kubeblocks.io/tpl-redis-metrics-config": "redis-test-redis-redis-metrics-config",
            "config.kubeblocks.io/tpl-redis-replication-config": "redis-test-redis-redis-replication-config",
            "config.kubeblocks.io/tpl-redis-scripts": "redis-test-redis-redis-scripts",
            "kubeblocks.io/generation": "1"
        },
        "creationTimestamp": "2024-01-31T11:27:08Z",
        "finalizers": [
            "cluster.kubeblocks.io/finalizer"
        ],
        "generation": 1,
        "labels": {
            "app.kubernetes.io/component": "redis",
            "app.kubernetes.io/instance": "redis-test",
            "app.kubernetes.io/managed-by": "kubeblocks",
            "app.kubernetes.io/name": "redis",
            "apps.kubeblocks.io/component-name": "redis"
        },
        "name": "redis-test-redis",
        "namespace": "default",
        "ownerReferences": [
            {
                "apiVersion": "apps.kubeblocks.io/v1alpha1",
                "blockOwnerDeletion": true,
                "controller": true,
                "kind": "Component",
                "name": "redis-test-redis",
                "uid": "17a36bc9-1778-46af-b45f-6d518567fc6a"
            }
        ],
        "resourceVersion": "8299684",
        "uid": "4b294085-2364-4238-a70a-58da618e9d13"
    },
    "spec": {
        "credential": {
            "password": {
                "valueFrom": {
                    "secretKeyRef": {
                        "key": "password",
                        "name": "redis-test-conn-credential"
                    }
                }
            },
            "username": {
                "valueFrom": {
                    "secretKeyRef": {
                        "key": "username",
                        "name": "redis-test-conn-credential"
                    }
                }
            }
        },
        "memberUpdateStrategy": "Serial",
        "podManagementPolicy": "Parallel",
        "replicas": 1,
        "roleProbe": {
            "builtinHandlerName": "redis",
            "failureThreshold": 2,
            "initialDelaySeconds": 0,
            "periodSeconds": 2,
            "roleUpdateMechanism": "DirectAPIServerEventUpdate",
            "successThreshold": 1,
            "timeoutSeconds": 1
        },
        "roles": [
            {
                "accessMode": "ReadWrite",
                "canVote": true,
                "isLeader": true,
                "name": "primary"
            },
            {
                "accessMode": "Readonly",
                "canVote": true,
                "isLeader": false,
                "name": "secondary"
            }
        ],
        "selector": {
            "matchLabels": {
                "app.kubernetes.io/instance": "redis-test",
                "app.kubernetes.io/managed-by": "kubeblocks",
                "app.kubernetes.io/name": "redis",
                "apps.kubeblocks.io/component-name": "redis"
            }
        },
        "serviceName": "redis-test-redis-headless",
        "template": {
            "metadata": {
                "creationTimestamp": null,
                "labels": {
                    "app.kubernetes.io/component": "redis",
                    "app.kubernetes.io/instance": "redis-test",
                    "app.kubernetes.io/managed-by": "kubeblocks",
                    "app.kubernetes.io/name": "redis",
                    "app.kubernetes.io/version": "",
                    "apps.kubeblocks.io/component-name": "redis"
                }
            },
            "spec": {
                "affinity": {
                    "nodeAffinity": {
                        "preferredDuringSchedulingIgnoredDuringExecution": [
                            {
                                "preference": {
                                    "matchExpressions": [
                                        {
                                            "key": "kb-data",
                                            "operator": "In",
                                            "values": [
                                                "true"
                                            ]
                                        }
                                    ]
                                },
                                "weight": 100
                            }
                        ]
                    },
                    "podAntiAffinity": {}
                },
                "containers": [
                    {
                        "command": [
                            "/scripts/redis-start.sh"
                        ],
                        "env": [
                            {
                                "name": "KB_POD_NAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.name"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_UID",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.uid"
                                    }
                                }
                            },
                            {
                                "name": "KB_NAMESPACE",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.namespace"
                                    }
                                }
                            },
                            {
                                "name": "KB_SA_NAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "spec.serviceAccountName"
                                    }
                                }
                            },
                            {
                                "name": "KB_NODENAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "spec.nodeName"
                                    }
                                }
                            },
                            {
                                "name": "KB_HOST_IP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.hostIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_IP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_IPS",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIPs"
                                    }
                                }
                            },
                            {
                                "name": "KB_HOSTIP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.hostIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_PODIP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_PODIPS",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIPs"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_FQDN",
                                "value": "$(KB_POD_NAME).redis-test-redis-headless.$(KB_NAMESPACE).svc"
                            },
                            {
                                "name": "SERVICE_PORT",
                                "value": "6379"
                            },
                            {
                                "name": "REDIS_REPL_USER",
                                "value": "kbreplicator"
                            },
                            {
                                "name": "REDIS_REPL_PASSWORD",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "password",
                                        "name": "redis-test-conn-credential",
                                        "optional": false
                                    }
                                }
                            },
                            {
                                "name": "REDIS_DEFAULT_USER",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "username",
                                        "name": "redis-test-conn-credential",
                                        "optional": false
                                    }
                                }
                            },
                            {
                                "name": "REDIS_DEFAULT_PASSWORD",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "password",
                                        "name": "redis-test-conn-credential",
                                        "optional": false
                                    }
                                }
                            },
                            {
                                "name": "REDIS_SENTINEL_USER",
                                "value": "$(REDIS_REPL_USER)-sentinel"
                            },
                            {
                                "name": "REDIS_SENTINEL_PASSWORD",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "password",
                                        "name": "redis-test-conn-credential",
                                        "optional": false
                                    }
                                }
                            },
                            {
                                "name": "REDIS_ARGS",
                                "value": "--requirepass $(REDIS_PASSWORD)"
                            }
                        ],
                        "envFrom": [
                            {
                                "configMapRef": {
                                    "name": "redis-test-redis-env",
                                    "optional": false
                                }
                            },
                            {
                                "configMapRef": {
                                    "name": "redis-test-redis-its-env",
                                    "optional": false
                                }
                            }
                        ],
                        "image": "infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/redis-stack-server:7.0.6-RC8",
                        "imagePullPolicy": "IfNotPresent",
                        "lifecycle": {
                            "preStop": {
                                "exec": {
                                    "command": [
                                        "/bin/bash",
                                        "-c",
                                        "/scripts/redis-preStop.sh"
                                    ]
                                }
                            }
                        },
                        "name": "redis",
                        "ports": [
                            {
                                "containerPort": 6379,
                                "name": "redis",
                                "protocol": "TCP"
                            }
                        ],
                        "readinessProbe": {
                            "exec": {
                                "command": [
                                    "sh",
                                    "-c",
                                    "/scripts/redis-ping.sh 1"
                                ]
                            },
                            "failureThreshold": 5,
                            "initialDelaySeconds": 10,
                            "periodSeconds": 5,
                            "successThreshold": 1,
                            "timeoutSeconds": 1
                        },
                        "resources": {
                            "limits": {
                                "cpu": "500m",
                                "memory": "512Mi"
                            },
                            "requests": {
                                "cpu": "500m",
                                "memory": "512Mi"
                            }
                        },
                        "terminationMessagePath": "/dev/termination-log",
                        "terminationMessagePolicy": "File",
                        "volumeMounts": [
                            {
                                "mountPath": "/data",
                                "name": "data"
                            },
                            {
                                "mountPath": "/etc/conf",
                                "name": "redis-config"
                            },
                            {
                                "mountPath": "/scripts",
                                "name": "scripts"
                            },
                            {
                                "mountPath": "/etc/redis",
                                "name": "redis-conf"
                            },
                            {
                                "mountPath": "/kb-podinfo",
                                "name": "pod-info"
                            }
                        ]
                    },
                    {
                        "command": [
                            "/bin/agamotto",
                            "--config=/opt/conf/metrics-config.yaml"
                        ],
                        "env": [
                            {
                                "name": "KB_POD_NAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.name"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_UID",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.uid"
                                    }
                                }
                            },
                            {
                                "name": "KB_NAMESPACE",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.namespace"
                                    }
                                }
                            },
                            {
                                "name": "KB_SA_NAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "spec.serviceAccountName"
                                    }
                                }
                            },
                            {
                                "name": "KB_NODENAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "spec.nodeName"
                                    }
                                }
                            },
                            {
                                "name": "KB_HOST_IP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.hostIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_IP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_IPS",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIPs"
                                    }
                                }
                            },
                            {
                                "name": "KB_HOSTIP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.hostIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_PODIP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_PODIPS",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIPs"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_FQDN",
                                "value": "$(KB_POD_NAME).redis-test-redis-headless.$(KB_NAMESPACE).svc"
                            },
                            {
                                "name": "ENDPOINT",
                                "value": "localhost:6379"
                            },
                            {
                                "name": "REDIS_USER",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "username",
                                        "name": "redis-test-conn-credential",
                                        "optional": false
                                    }
                                }
                            },
                            {
                                "name": "REDIS_PASSWORD",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "password",
                                        "name": "redis-test-conn-credential",
                                        "optional": false
                                    }
                                }
                            }
                        ],
                        "envFrom": [
                            {
                                "configMapRef": {
                                    "name": "redis-test-redis-env",
                                    "optional": false
                                }
                            },
                            {
                                "configMapRef": {
                                    "name": "redis-test-redis-its-env",
                                    "optional": false
                                }
                            }
                        ],
                        "image": "infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/agamotto:0.1.2-beta.1",
                        "imagePullPolicy": "IfNotPresent",
                        "name": "metrics",
                        "ports": [
                            {
                                "containerPort": 9121,
                                "name": "http-metrics",
                                "protocol": "TCP"
                            }
                        ],
                        "resources": {
                            "limits": {
                                "cpu": "0",
                                "memory": "0"
                            }
                        },
                        "securityContext": {
                            "runAsNonRoot": true,
                            "runAsUser": 1001
                        },
                        "terminationMessagePath": "/dev/termination-log",
                        "terminationMessagePolicy": "File",
                        "volumeMounts": [
                            {
                                "mountPath": "/opt/conf",
                                "name": "redis-metrics-config"
                            }
                        ]
                    },
                    {
                        "command": [
                            "lorry",
                            "--port",
                            "3501",
                            "--grpcport",
                            "50001"
                        ],
                        "env": [
                            {
                                "name": "KB_POD_NAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.name"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_UID",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.uid"
                                    }
                                }
                            },
                            {
                                "name": "KB_NAMESPACE",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.namespace"
                                    }
                                }
                            },
                            {
                                "name": "KB_SA_NAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "spec.serviceAccountName"
                                    }
                                }
                            },
                            {
                                "name": "KB_NODENAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "spec.nodeName"
                                    }
                                }
                            },
                            {
                                "name": "KB_HOST_IP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.hostIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_IP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_IPS",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIPs"
                                    }
                                }
                            },
                            {
                                "name": "KB_HOSTIP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.hostIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_PODIP",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIP"
                                    }
                                }
                            },
                            {
                                "name": "KB_PODIPS",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "status.podIPs"
                                    }
                                }
                            },
                            {
                                "name": "KB_POD_FQDN",
                                "value": "$(KB_POD_NAME).redis-test-redis-headless.$(KB_NAMESPACE).svc"
                            },
                            {
                                "name": "KB_SERVICE_PORT",
                                "value": "6379"
                            },
                            {
                                "name": "KB_DATA_PATH",
                                "value": "/data"
                            },
                            {
                                "name": "KB_BUILTIN_HANDLER",
                                "value": "redis"
                            },
                            {
                                "name": "KB_SERVICE_USER",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "username",
                                        "name": "redis-test-conn-credential"
                                    }
                                }
                            },
                            {
                                "name": "KB_SERVICE_PASSWORD",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "password",
                                        "name": "redis-test-conn-credential"
                                    }
                                }
                            },
                            {
                                "name": "KB_RSM_ACTION_SVC_LIST",
                                "value": "null"
                            },
                            {
                                "name": "KB_RSM_ROLE_UPDATE_MECHANISM",
                                "value": "DirectAPIServerEventUpdate"
                            },
                            {
                                "name": "KB_RSM_ROLE_PROBE_TIMEOUT",
                                "value": "1"
                            },
                            {
                                "name": "KB_CLUSTER_NAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.labels['app.kubernetes.io/instance']"
                                    }
                                }
                            },
                            {
                                "name": "KB_COMP_NAME",
                                "valueFrom": {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.labels['apps.kubeblocks.io/component-name']"
                                    }
                                }
                            },
                            {
                                "name": "KB_SERVICE_CHARACTER_TYPE",
                                "value": "redis"
                            }
                        ],
                        "envFrom": [
                            {
                                "configMapRef": {
                                    "name": "redis-test-redis-env",
                                    "optional": false
                                }
                            },
                            {
                                "configMapRef": {
                                    "name": "redis-test-redis-its-env",
                                    "optional": false
                                }
                            }
                        ],
                        "image": "infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/kubeblocks-tools:0.8.1",
                        "imagePullPolicy": "IfNotPresent",
                        "name": "kb-checkrole",
                        "ports": [
                            {
                                "containerPort": 3501,
                                "name": "lorry-http-port",
                                "protocol": "TCP"
                            },
                            {
                                "containerPort": 50001,
                                "name": "lorry-grpc-port",
                                "protocol": "TCP"
                            }
                        ],
                        "readinessProbe": {
                            "failureThreshold": 2,
                            "httpGet": {
                                "path": "/v1.0/checkrole",
                                "port": 3501,
                                "scheme": "HTTP"
                            },
                            "periodSeconds": 2,
                            "successThreshold": 1,
                            "timeoutSeconds": 1
                        },
                        "resources": {
                            "limits": {
                                "cpu": "0",
                                "memory": "0"
                            }
                        },
                        "startupProbe": {
                            "failureThreshold": 3,
                            "periodSeconds": 10,
                            "successThreshold": 1,
                            "tcpSocket": {
                                "port": 3501
                            },
                            "timeoutSeconds": 1
                        },
                        "terminationMessagePath": "/dev/termination-log",
                        "terminationMessagePolicy": "File",
                        "volumeMounts": [
                            {
                                "mountPath": "/data",
                                "name": "data"
                            }
                        ]
                    }
                ],
                "dnsPolicy": "ClusterFirst",
                "restartPolicy": "Always",
                "schedulerName": "default-scheduler",
                "securityContext": {},
                "serviceAccount": "kb-redis-test",
                "serviceAccountName": "kb-redis-test",
                "terminationGracePeriodSeconds": 30,
                "tolerations": [
                    {
                        "effect": "NoSchedule",
                        "key": "kb-data",
                        "operator": "Equal",
                        "value": "true"
                    }
                ],
                "volumes": [
                    {
                        "downwardAPI": {
                            "defaultMode": 420,
                            "items": [
                                {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.labels['kubeblocks.io/role']"
                                    },
                                    "path": "pod-role"
                                },
                                {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.annotations['rs.apps.kubeblocks.io/primary']"
                                    },
                                    "path": "primary-pod"
                                },
                                {
                                    "fieldRef": {
                                        "apiVersion": "v1",
                                        "fieldPath": "metadata.annotations['apps.kubeblocks.io/component-replicas']"
                                    },
                                    "path": "component-replicas"
                                }
                            ]
                        },
                        "name": "pod-info"
                    },
                    {
                        "configMap": {
                            "defaultMode": 292,
                            "name": "redis-test-redis-redis-metrics-config"
                        },
                        "name": "redis-metrics-config"
                    },
                    {
                        "configMap": {
                            "defaultMode": 420,
                            "name": "redis-test-redis-redis-replication-config"
                        },
                        "name": "redis-config"
                    },
                    {
                        "configMap": {
                            "defaultMode": 365,
                            "name": "redis-test-redis-redis-scripts"
                        },
                        "name": "scripts"
                    },
                    {
                        "emptyDir": {},
                        "name": "data"
                    },
                    {
                        "emptyDir": {},
                        "name": "redis-conf"
                    }
                ]
            }
        },
        "updateStrategy": {
            "type": "OnDelete"
        },
        "volumeClaimTemplates": [
            {
                "metadata": {
                    "labels": {
                        "apps.kubeblocks.io/vct-name": "data",
                        "kubeblocks.io/volume-type": "data"
                    },
                    "name": "data"
                },
                "spec": {
                    "accessModes": [
                        "ReadWriteOnce"
                    ],
                    "resources": {
                        "requests": {
                            "storage": "1Gi"
                        }
                    }
                },
                "status": {}
            }
        ]
    },
    "status": {
        "availableReplicas": 1,
        "collisionCount": 0,
        "currentGeneration": 1,
        "currentReplicas": 1,
        "currentRevision": "redis-test-redis-7665b47874",
        "initReplicas": 1,
        "membersStatus": [
            {
                "podName": "redis-test-redis-0",
                "readyWithoutPrimary": false,
                "role": {
                    "accessMode": "ReadWrite",
                    "canVote": true,
                    "isLeader": true,
                    "name": "primary"
                }
            }
        ],
        "observedGeneration": 1,
        "readyInitReplicas": 1,
        "readyReplicas": 1,
        "replicas": 1,
        "updateRevision": "redis-test-redis-7665b47874",
        "updatedReplicas": 1
    }
}
`
			its := &workloads.InstanceSet{}
			err := json.Unmarshal([]byte(stsJSON), its)
			Expect(err).Should(Succeed())
			cr, err := NewRevision(its)
			Expect(err).Should(Succeed())
			Expect(cr.Name).Should(Equal("redis-test-redis-694cf8dbf8"))
		})
	})

	Context("buildUpdateRevisions & getUpdateRevisions", func() {
		It("should work well", func() {
			updateRevisions := map[string]string{
				"pod-0": "revision-0",
				"pod-1": "revision-1",
				"pod-2": "revision-2",
				"pod-3": "revision-3",
				"pod-4": "revision-4",
			}
			revisions, err := buildRevisions(updateRevisions)
			Expect(err).Should(BeNil())
			decodeRevisions, err := GetRevisions(revisions)
			Expect(err).Should(BeNil())
			Expect(decodeRevisions).Should(Equal(updateRevisions))
		})
	})
})
