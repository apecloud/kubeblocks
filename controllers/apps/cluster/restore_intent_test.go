/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package cluster

import (
	"context"
	"os"
	"strconv"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/require"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	testRestoreSourceAPIGroup = "example.kubeblocks.io"
	testRestoreSourceKind     = "ExampleSource"
)

func TestInjectRestoreIntentRemovesStaleOptionalAnnotations(t *testing.T) {
	cluster := &appsv1.Cluster{}
	cluster.Name = "test-cluster"
	cluster.Namespace = "test-ns"
	cluster.Spec.Restore = &appsv1.ClusterRestore{
		Source: appsv1.ClusterRestoreSource{
			APIGroup:  testRestoreSourceAPIGroup,
			Kind:      testRestoreSourceKind,
			Name:      "backup",
			Namespace: "backup-ns",
		},
	}
	vct := &appsv1.PersistentVolumeClaimTemplate{
		Name: "data",
		Annotations: map[string]string{
			constant.RestorePITRAnnotationKey:       "stale-pitr",
			constant.RestoreParametersAnnotationKey: `{"stale":"true"}`,
		},
	}

	injectRestoreIntentToVCT(cluster, "mysql", vct)

	require.NotContains(t, vct.Annotations, constant.RestorePITRAnnotationKey)
	require.NotContains(t, vct.Annotations, constant.RestoreParametersAnnotationKey)
	require.Equal(t, "backup-ns", vct.Annotations[constant.RestoreSourceNamespaceAnnotationKey])
	require.Equal(t, testRestoreSourceKind, vct.Spec.DataSourceRef.Kind)
	require.Equal(t, "backup", vct.Spec.DataSourceRef.Name)
	require.NotNil(t, vct.Spec.DataSourceRef.Namespace)
	require.Equal(t, "backup-ns", *vct.Spec.DataSourceRef.Namespace)
}

func TestCopyAndMergeComponentPreservesShardingReconfigureIntent(t *testing.T) {
	hash := "target-hash"
	oldCompObj := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constant.KBAppShardingNameLabelKey: "shard",
			},
		},
		Spec: appsv1.ComponentSpec{
			Configs: []appsv1.ClusterComponentConfig{{
				Name:       ptr.To("mongodb.conf"),
				ConfigHash: ptr.To(hash),
				Restart:    ptr.To(true),
			}},
		},
	}
	newCompObj := oldCompObj.DeepCopy()
	newCompObj.Spec.Configs = []appsv1.ClusterComponentConfig{{
		Name: ptr.To("mongodb.conf"),
	}}

	require.Nil(t, copyAndMergeComponent(oldCompObj, newCompObj))
}

func TestCopyAndMergeComponentUsesNewShardingReconfigureIntent(t *testing.T) {
	oldHash := "old-hash"
	newHash := "new-hash"
	oldCompObj := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constant.KBAppShardingNameLabelKey: "shard",
			},
		},
		Spec: appsv1.ComponentSpec{
			Configs: []appsv1.ClusterComponentConfig{{
				Name:       ptr.To("mongodb.conf"),
				ConfigHash: ptr.To(oldHash),
				Restart:    ptr.To(true),
			}},
		},
	}
	newCompObj := oldCompObj.DeepCopy()
	newCompObj.Spec.Configs = []appsv1.ClusterComponentConfig{{
		Name:       ptr.To("mongodb.conf"),
		ConfigHash: ptr.To(newHash),
		Restart:    ptr.To(true),
	}}

	result := copyAndMergeComponent(oldCompObj, newCompObj)
	require.NotNil(t, result)
	require.Len(t, result.Spec.Configs, 1)
	require.NotNil(t, result.Spec.Configs[0].ConfigHash)
	require.Equal(t, newHash, *result.Spec.Configs[0].ConfigHash)
}

func TestCopyAndMergeComponentClearsNonShardingReconfigureIntent(t *testing.T) {
	hash := "target-hash"
	oldCompObj := &appsv1.Component{
		Spec: appsv1.ComponentSpec{
			Configs: []appsv1.ClusterComponentConfig{{
				Name:       ptr.To("mongodb.conf"),
				ConfigHash: ptr.To(hash),
				Restart:    ptr.To(true),
			}},
		},
	}
	newCompObj := oldCompObj.DeepCopy()
	newCompObj.Spec.Configs = []appsv1.ClusterComponentConfig{{
		Name: ptr.To("mongodb.conf"),
	}}

	result := copyAndMergeComponent(oldCompObj, newCompObj)
	require.NotNil(t, result)
	require.Len(t, result.Spec.Configs, 1)
	require.Nil(t, result.Spec.Configs[0].ConfigHash)
	require.Nil(t, result.Spec.Configs[0].Restart)
}

func TestInjectRestoreIntentOmitsDataSourceRefNamespaceForSameNamespaceSource(t *testing.T) {
	cluster := &appsv1.Cluster{}
	cluster.Name = "test-cluster"
	cluster.Namespace = "test-ns"
	cluster.Spec.Restore = &appsv1.ClusterRestore{
		Source: appsv1.ClusterRestoreSource{
			APIGroup:  testRestoreSourceAPIGroup,
			Kind:      testRestoreSourceKind,
			Name:      "backup",
			Namespace: "test-ns",
		},
	}
	vct := &appsv1.PersistentVolumeClaimTemplate{Name: "data"}

	injectRestoreIntentToVCT(cluster, "redis", vct)

	require.Equal(t, "test-ns", vct.Annotations[constant.RestoreSourceNamespaceAnnotationKey])
	require.NotNil(t, vct.Spec.DataSourceRef)
	require.Nil(t, vct.Spec.DataSourceRef.Namespace)
}

func TestInjectRestoreIntentOmitsDataSourceRefNamespaceForDefaultNamespaceSource(t *testing.T) {
	cluster := &appsv1.Cluster{}
	cluster.Name = "test-cluster"
	cluster.Namespace = "test-ns"
	cluster.Spec.Restore = &appsv1.ClusterRestore{
		Source: appsv1.ClusterRestoreSource{
			APIGroup: testRestoreSourceAPIGroup,
			Kind:     testRestoreSourceKind,
			Name:     "backup",
		},
	}
	vct := &appsv1.PersistentVolumeClaimTemplate{Name: "data"}

	injectRestoreIntentToVCT(cluster, "redis", vct)

	require.Equal(t, "test-ns", vct.Annotations[constant.RestoreSourceNamespaceAnnotationKey])
	require.NotNil(t, vct.Spec.DataSourceRef)
	require.Nil(t, vct.Spec.DataSourceRef.Namespace)
}

func TestClusterRestoreSourceAPIGroupIsRequiredByCRD(t *testing.T) {
	data, err := os.ReadFile("../../../config/crd/bases/apps.kubeblocks.io_clusters.yaml")
	require.NoError(t, err)

	var crd map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &crd))

	versionSchema := nestedMap(t, crd, "spec", "versions", "0", "schema", "openAPIV3Schema")
	sourceSchema := nestedMap(t, versionSchema, "properties", "spec", "properties", "restore", "properties", "source")
	required, ok := sourceSchema["required"].([]interface{})
	require.True(t, ok)
	require.Contains(t, required, "apiGroup")
	require.Contains(t, required, "kind")
	require.Contains(t, required, "name")
}

func TestApplyClusterRestoreIntentCleansTemplatesAfterRestoreCompleted(t *testing.T) {
	cluster := &appsv1.Cluster{}
	cluster.Name = "test-cluster"
	cluster.Namespace = "test-ns"
	cluster.Spec.Restore = &appsv1.ClusterRestore{
		Source: appsv1.ClusterRestoreSource{
			APIGroup: testRestoreSourceAPIGroup,
			Kind:     testRestoreSourceKind,
			Name:     "backup",
		},
	}
	cluster.Status.Conditions = []metav1.Condition{{
		Type:   appsv1.ConditionTypeRestore,
		Status: metav1.ConditionTrue,
	}}
	dataSourceAPIGroup := testRestoreSourceAPIGroup
	component := &appsv1.ClusterComponentSpec{
		Name: "mysql",
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
			Name: "data",
			Annotations: map[string]string{
				constant.RestoreSourceKindAnnotationKey: testRestoreSourceKind,
				constant.RestorePITRAnnotationKey:       "stale-pitr",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				DataSourceRef: &corev1.TypedObjectReference{
					APIGroup: &dataSourceAPIGroup,
					Kind:     testRestoreSourceKind,
					Name:     "backup",
				},
			},
		}},
	}

	require.NoError(t, applyClusterRestoreIntent(cluster, []*appsv1.ClusterComponentSpec{component}, nil))

	vct := component.VolumeClaimTemplates[0]
	require.Nil(t, vct.Spec.DataSourceRef)
	require.Nil(t, vct.Annotations)
	require.NotContains(t, vct.Annotations, constant.RestoreSourceKindAnnotationKey)
	require.NotContains(t, vct.Annotations, constant.RestorePITRAnnotationKey)
}

func TestApplyClusterRestoreIntentKeepsNonRestoreDataSourceAfterRestoreCompleted(t *testing.T) {
	cluster := &appsv1.Cluster{}
	cluster.Spec.Restore = &appsv1.ClusterRestore{
		Source: appsv1.ClusterRestoreSource{
			APIGroup: testRestoreSourceAPIGroup,
			Kind:     testRestoreSourceKind,
			Name:     "backup",
		},
	}
	cluster.Status.Conditions = []metav1.Condition{{
		Type:   appsv1.ConditionTypeRestore,
		Status: metav1.ConditionTrue,
	}}
	dataSourceAPIGroup := "snapshot.storage.k8s.io"
	component := &appsv1.ClusterComponentSpec{
		Name: "mysql",
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
			Name: "data",
			Spec: corev1.PersistentVolumeClaimSpec{
				DataSourceRef: &corev1.TypedObjectReference{
					APIGroup: &dataSourceAPIGroup,
					Kind:     "VolumeSnapshot",
					Name:     "snapshot",
				},
			},
		}},
	}

	require.NoError(t, applyClusterRestoreIntent(cluster, []*appsv1.ClusterComponentSpec{component}, nil))

	vct := component.VolumeClaimTemplates[0]
	require.NotNil(t, vct.Spec.DataSourceRef)
	require.Equal(t, dataSourceAPIGroup, *vct.Spec.DataSourceRef.APIGroup)
	require.Equal(t, "VolumeSnapshot", vct.Spec.DataSourceRef.Kind)
	require.Equal(t, "snapshot", vct.Spec.DataSourceRef.Name)
}

func TestApplyClusterRestoreIntentHandlesInstanceTemplateVCTs(t *testing.T) {
	cluster := &appsv1.Cluster{}
	cluster.Name = "test-cluster"
	cluster.Namespace = "test-ns"
	cluster.Spec.Restore = &appsv1.ClusterRestore{
		Source: appsv1.ClusterRestoreSource{
			APIGroup: testRestoreSourceAPIGroup,
			Kind:     testRestoreSourceKind,
			Name:     "backup",
		},
	}
	component := &appsv1.ClusterComponentSpec{
		Name: "mysql",
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
			Name: "data",
		}},
		Instances: []appsv1.InstanceTemplate{{
			Name: "hot",
			VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
				Name: "data",
			}, {
				Name: "log",
			}},
		}},
	}

	require.NoError(t, applyClusterRestoreIntent(cluster, []*appsv1.ClusterComponentSpec{component}, nil))

	require.Equal(t, testRestoreSourceKind, component.VolumeClaimTemplates[0].Spec.DataSourceRef.Kind)
	require.Equal(t, testRestoreSourceKind, component.Instances[0].VolumeClaimTemplates[0].Spec.DataSourceRef.Kind)
	require.Equal(t, testRestoreSourceKind, component.Instances[0].VolumeClaimTemplates[1].Spec.DataSourceRef.Kind)
	require.Equal(t, "mysql", component.Instances[0].VolumeClaimTemplates[1].Annotations[constant.RestoreComponentAnnotationKey])

	cluster.Status.Conditions = []metav1.Condition{{
		Type:   appsv1.ConditionTypeRestore,
		Status: metav1.ConditionTrue,
	}}

	require.NoError(t, applyClusterRestoreIntent(cluster, []*appsv1.ClusterComponentSpec{component}, nil))

	require.Nil(t, component.VolumeClaimTemplates[0].Spec.DataSourceRef)
	require.Nil(t, component.Instances[0].VolumeClaimTemplates[0].Spec.DataSourceRef)
	require.Nil(t, component.Instances[0].VolumeClaimTemplates[1].Spec.DataSourceRef)
}

func TestSetRestoreConditionSucceedsWhenNoRestorePVCsExist(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster",
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup: testRestoreSourceAPIGroup,
					Kind:     testRestoreSourceKind,
					Name:     "backup",
				},
			},
		},
	}

	require.NoError(t, (&clusterStatusTransformer{}).setRestoreCondition(context.Background(), cli, cluster))

	cond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionTrue, cond.Status)
	require.Equal(t, ReasonRestoreCompleted, cond.Reason)
}

func TestSetRestoreConditionWaitsWhenRestorePVCsAreNotCreatedYet(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster",
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup: testRestoreSourceAPIGroup,
					Kind:     testRestoreSourceKind,
					Name:     "backup",
				},
			},
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "mysql",
				VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
					Name: "data",
				}},
			}},
		},
	}

	require.NoError(t, (&clusterStatusTransformer{}).setRestoreCondition(context.Background(), cli, cluster))

	cond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionUnknown, cond.Status)
	require.Equal(t, ReasonRestoreRunning, cond.Reason)
}

func TestSetRestoreConditionWaitsForAllExpectedRestorePVCs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster",
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup: testRestoreSourceAPIGroup,
					Kind:     testRestoreSourceKind,
					Name:     "backup",
				},
			},
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name:     "mysql",
				Replicas: 2,
				VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
					Name: "data",
				}},
			}},
		},
	}
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster-mysql",
			Labels:    constant.GetCompLabels("test-cluster", "mysql"),
		},
		Spec: appsv1.ComponentSpec{
			Replicas: 2,
			VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
				Name: "data",
				Annotations: map[string]string{
					constant.RestoreSourceKindAnnotationKey: testRestoreSourceKind,
				},
			}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(component).Build()

	require.NoError(t, (&clusterStatusTransformer{}).setRestoreCondition(context.Background(), cli, cluster))

	cond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionUnknown, cond.Status)
	require.Equal(t, ReasonRestoreRunning, cond.Reason)
}

func TestSetRestoreConditionSucceedsWhenAllComponentsRestoreCompleted(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster",
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup: testRestoreSourceAPIGroup,
					Kind:     testRestoreSourceKind,
					Name:     "backup",
				},
			},
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name:     "mysql",
				Replicas: 1,
				VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
					Name: "data",
				}},
			}},
		},
	}
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster-mysql",
			Labels:    constant.GetCompLabels("test-cluster", "mysql"),
		},
		Spec: appsv1.ComponentSpec{
			Replicas: 1,
			VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
				Name: "data",
				Annotations: map[string]string{
					constant.RestoreSourceKindAnnotationKey: testRestoreSourceKind,
				},
			}},
		},
		Status: appsv1.ComponentStatus{
			Conditions: []metav1.Condition{{
				Type:   appsv1.ConditionTypeRestore,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(component).Build()

	require.NoError(t, (&clusterStatusTransformer{}).setRestoreCondition(context.Background(), cli, cluster))

	cond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionTrue, cond.Status)
	require.Equal(t, ReasonRestoreCompleted, cond.Reason)
}

func TestSetRestoreConditionWaitsForInstanceTemplateRestorePVCs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	replicas := int32(1)
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster",
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup: testRestoreSourceAPIGroup,
					Kind:     testRestoreSourceKind,
					Name:     "backup",
				},
			},
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name:     "mysql",
				Replicas: 2,
				VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
					Name: "data",
				}},
				Instances: []appsv1.InstanceTemplate{{
					Name:     "hot",
					Replicas: &replicas,
					VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
						Name: "data",
					}, {
						Name: "log",
					}},
				}},
			}},
		},
	}
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster-mysql",
			Labels:    constant.GetCompLabels("test-cluster", "mysql"),
		},
		Spec: appsv1.ComponentSpec{
			Replicas: 2,
			VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
				Name: "data",
				Annotations: map[string]string{
					constant.RestoreSourceKindAnnotationKey: testRestoreSourceKind,
				},
			}},
			Instances: []appsv1.InstanceTemplate{{
				Name:     "hot",
				Replicas: &replicas,
				VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
					Name: "data",
					Annotations: map[string]string{
						constant.RestoreSourceKindAnnotationKey: testRestoreSourceKind,
					},
				}, {
					Name: "log",
					Annotations: map[string]string{
						constant.RestoreSourceKindAnnotationKey: testRestoreSourceKind,
					},
				}},
			}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(component).Build()

	require.NoError(t, (&clusterStatusTransformer{}).setRestoreCondition(context.Background(), cli, cluster))

	cond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionUnknown, cond.Status)
	require.Equal(t, ReasonRestoreRunning, cond.Reason)
}

func TestSetRestoreConditionKeepsTerminalRestoreCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster",
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup: testRestoreSourceAPIGroup,
					Kind:     testRestoreSourceKind,
					Name:     "backup",
				},
			},
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name:     "mysql",
				Replicas: 3,
				VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
					Name: "data",
				}},
			}},
		},
		Status: appsv1.ClusterStatus{
			Conditions: []metav1.Condition{{
				Type:               appsv1.ConditionTypeRestore,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonRestoreCompleted,
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-cluster-mysql",
			Labels:    constant.GetCompLabels("test-cluster", "mysql"),
		},
		Spec: appsv1.ComponentSpec{
			Replicas: 3,
			VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
				Name: "data",
				Annotations: map[string]string{
					constant.RestoreSourceKindAnnotationKey: testRestoreSourceKind,
				},
			}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(component).Build()

	require.NoError(t, (&clusterStatusTransformer{}).setRestoreCondition(context.Background(), cli, cluster))

	cond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionTrue, cond.Status)
	require.Equal(t, ReasonRestoreCompleted, cond.Reason)
}

func nestedMap(t *testing.T, value interface{}, path ...string) map[string]interface{} {
	t.Helper()
	current := value
	for _, segment := range path {
		switch typed := current.(type) {
		case map[string]interface{}:
			next, ok := typed[segment]
			require.Truef(t, ok, "missing path segment %q", segment)
			current = next
		case []interface{}:
			index, err := strconv.Atoi(segment)
			require.NoError(t, err)
			require.Less(t, index, len(typed))
			current = typed[index]
		default:
			require.Failf(t, "unexpected schema node", "segment %q on %T", segment, current)
		}
	}
	result, ok := current.(map[string]interface{})
	require.Truef(t, ok, "expected map at path %v, got %T", path, current)
	return result
}
