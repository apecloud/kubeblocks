/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package parameters

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	parameterreconfigure "github.com/apecloud/kubeblocks/controllers/parameters/reconfigure"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func TestReconcileRollbackAcceptsIntentBeforeApplyingDesired(t *testing.T) {
	ctx := context.Background()
	compParam := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "default",
			Name:        "cluster-mysql",
			Generation:  8,
			Annotations: map[string]string{constant.OpsRequestUIDAnnotationKey: "ops-uid"},
		},
		Spec: parametersv1alpha1.ComponentParameterSpec{
			ClusterName:   "cluster",
			ComponentName: "mysql",
			Desired: &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"max_connections": ptr.To("200")},
			},
			Rollback: &parametersv1alpha1.ParameterRollbackRequest{
				RequestID:        "ops-uid",
				SourceGeneration: 7,
				Desired: &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{"max_connections": ptr.To("100")},
				},
				Restart: true,
			},
		},
		Status: parametersv1alpha1.ComponentParameterStatus{
			ObservedGeneration: 7,
			Phase:              parametersv1alpha1.CMergeFailedPhase,
		},
	}
	cli := newRollbackFakeClient(t, compParam)
	reconciler := &ComponentParameterReconciler{Client: cli}

	handled, err := reconciler.reconcileRollback(rollbackRequestCtx(ctx, compParam), compParam)
	require.NoError(t, err)
	require.True(t, handled)

	fetched := &parametersv1alpha1.ComponentParameter{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(compParam), fetched))
	require.Equal(t, ptr.To("200"), fetched.Spec.Desired.Assignments["max_connections"],
		"accepting the public intent must not let the requester mutate Parameters-owned desired state")
	require.NotNil(t, fetched.Status.Rollback)
	require.Equal(t, parametersv1alpha1.ParameterRollbackPending, fetched.Status.Rollback.Phase)

	handled, err = reconciler.reconcileRollback(rollbackRequestCtx(ctx, fetched), fetched)
	require.NoError(t, err)
	require.True(t, handled)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(compParam), fetched))
	require.Equal(t, ptr.To("100"), fetched.Spec.Desired.Assignments["max_connections"])
}

func TestReconcileRollbackRejectsSameIDPayloadChange(t *testing.T) {
	ctx := context.Background()
	compParam := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "default",
			Name:        "cluster-mysql",
			Generation:  8,
			Annotations: map[string]string{constant.OpsRequestUIDAnnotationKey: "ops-uid"},
		},
		Spec: parametersv1alpha1.ComponentParameterSpec{
			Desired: &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{"a": ptr.To("new")}},
			Rollback: &parametersv1alpha1.ParameterRollbackRequest{
				RequestID: "ops-uid", SourceGeneration: 7,
				Desired: &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{"a": ptr.To("old")}},
				Restart: true,
			},
		},
		Status: parametersv1alpha1.ComponentParameterStatus{
			ObservedGeneration: 7,
			Phase:              parametersv1alpha1.CMergeFailedPhase,
		},
	}
	cli := newRollbackFakeClient(t, compParam)
	reconciler := &ComponentParameterReconciler{Client: cli}

	_, err := reconciler.reconcileRollback(rollbackRequestCtx(ctx, compParam), compParam)
	require.NoError(t, err)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(compParam), compParam))
	require.Equal(t, parametersv1alpha1.ParameterRollbackPending, compParam.Status.Rollback.Phase)
	require.NotEmpty(t, compParam.Status.Rollback.RequestHash)

	compParam.Spec.Rollback.Restart = false
	require.NoError(t, cli.Update(ctx, compParam))
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(compParam), compParam))
	_, err = reconciler.reconcileRollback(rollbackRequestCtx(ctx, compParam), compParam)
	require.NoError(t, err)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(compParam), compParam))
	require.Equal(t, parametersv1alpha1.ParameterRollbackFailed, compParam.Status.Rollback.Phase)
	require.Contains(t, compParam.Status.Rollback.Message, "changed after it was accepted")
}

func TestApplyRollbackMetadataClearsOnlyOwnedFailureGate(t *testing.T) {
	newObjects := func(failureRevision string) (*parametersv1alpha1.ComponentParameter, *corev1.ConfigMap) {
		compParam := &parametersv1alpha1.ComponentParameter{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster-mysql", Generation: 10},
			Spec: parametersv1alpha1.ComponentParameterSpec{
				ClusterName:   "cluster",
				ComponentName: "mysql",
				ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{
					Name: "mysql-config", ConfigSpec: &structConfigTemplate,
				}},
			},
		}
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      parameterscore.GetComponentCfgName("cluster", "mysql", "mysql-config"),
			Annotations: map[string]string{
				constant.ConfigurationRevision:                       "10",
				constant.DisableUpgradeInsConfigurationAnnotationKey: "true",
				constant.ReconfigureFailureRevisionAnnotationKey:     failureRevision,
			},
		}}
		return compParam, configMap
	}
	request := &parametersv1alpha1.ParameterRollbackRequest{RequestID: "ops-uid", SourceGeneration: 7, Restart: true}

	t.Run("owned gate", func(t *testing.T) {
		compParam, configMap := newObjects("7")
		compParam.Spec.Rollback = request

		err := applyRollbackMetadata(compParam, configMap, "10")
		require.NoError(t, err)
		require.NotContains(t, configMap.Annotations, constant.DisableUpgradeInsConfigurationAnnotationKey)
		require.NotContains(t, configMap.Annotations, constant.ReconfigureFailureRevisionAnnotationKey)
		require.Equal(t, "10", configMap.Annotations[constant.ParameterRollbackRevisionAnnotationKey])
		require.Equal(t, "true", configMap.Annotations[constant.ParameterRollbackRestartAnnotationKey])
	})

	t.Run("unowned gate", func(t *testing.T) {
		compParam, configMap := newObjects("user-supplied")
		compParam.Spec.Rollback = request

		err := applyRollbackMetadata(compParam, configMap, "10")
		require.ErrorContains(t, err, "does not carry the Parameters-owned failure gate")
		require.Equal(t, "true", configMap.Annotations[constant.DisableUpgradeInsConfigurationAnnotationKey])
		require.NotContains(t, configMap.Annotations, constant.ParameterRollbackRevisionAnnotationKey)
	})

	t.Run("ungated sibling config", func(t *testing.T) {
		compParam, configMap := newObjects("7")
		compParam.Spec.Rollback = request
		delete(configMap.Annotations, constant.DisableUpgradeInsConfigurationAnnotationKey)
		delete(configMap.Annotations, constant.ReconfigureFailureRevisionAnnotationKey)

		err := applyRollbackMetadata(compParam, configMap, "10")
		require.NoError(t, err)
		require.Equal(t, "10", configMap.Annotations[constant.ParameterRollbackRevisionAnnotationKey])
		require.Equal(t, "true", configMap.Annotations[constant.ParameterRollbackRestartAnnotationKey])
	})
}

func TestReconcileRollbackPublishesSuccessThenCleansIntent(t *testing.T) {
	ctx := context.Background()
	compParam := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "default",
			Name:        "cluster-mysql",
			Generation:  10,
			Annotations: map[string]string{constant.OpsRequestUIDAnnotationKey: "ops-uid"},
		},
		Spec: parametersv1alpha1.ComponentParameterSpec{
			ClusterName:   "cluster",
			ComponentName: "mysql",
			Desired:       &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{"a": ptr.To("old")}},
			Rollback: &parametersv1alpha1.ParameterRollbackRequest{
				RequestID: "ops-uid", SourceGeneration: 7,
				Desired: &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{"a": ptr.To("old")}},
				Restart: true,
			},
			ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{
				Name: "mysql-config", ConfigSpec: &structConfigTemplate,
			}},
		},
		Status: parametersv1alpha1.ComponentParameterStatus{
			ObservedGeneration: 10,
			Phase:              parametersv1alpha1.CFinishedPhase,
			Rollback: &parametersv1alpha1.ParameterRollbackStatus{
				RequestID: "ops-uid", Phase: parametersv1alpha1.ParameterRollbackRunning, TargetGeneration: 10,
			},
		},
	}
	requestHash, err := rollbackRequestHash(compParam.Spec.Rollback)
	require.NoError(t, err)
	compParam.Status.Rollback.RequestHash = requestHash
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      parameterscore.GetComponentCfgName("cluster", "mysql", "mysql-config"),
		Annotations: map[string]string{
			constant.ParameterRollbackRevisionAnnotationKey: "10",
			constant.ParameterRollbackRestartAnnotationKey:  "true",
		},
	}}
	cli := newRollbackFakeClient(t, compParam, configMap)
	reconciler := &ComponentParameterReconciler{Client: cli}

	handled, err := reconciler.reconcileRollback(rollbackRequestCtx(ctx, compParam), compParam)
	require.NoError(t, err)
	require.True(t, handled)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(compParam), compParam))
	require.Equal(t, parametersv1alpha1.ParameterRollbackSucceeded, compParam.Status.Rollback.Phase)
	require.NotNil(t, compParam.Spec.Rollback, "status must be observable before the request is removed")

	handled, err = reconciler.reconcileRollback(rollbackRequestCtx(ctx, compParam), compParam)
	require.NoError(t, err)
	require.True(t, handled)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(configMap), configMap))
	require.NotContains(t, configMap.Annotations, constant.ParameterRollbackRevisionAnnotationKey)

	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(compParam), compParam))
	handled, err = reconciler.reconcileRollback(rollbackRequestCtx(ctx, compParam), compParam)
	require.NoError(t, err)
	require.True(t, handled)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(compParam), compParam))
	require.Nil(t, compParam.Spec.Rollback)
	require.Equal(t, parametersv1alpha1.ParameterRollbackSucceeded, compParam.Status.Rollback.Phase)
}

func TestUpdateConfigPhaseMarksParametersOwnedFailureGate(t *testing.T) {
	ctx := context.Background()
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "config",
		Annotations: map[string]string{
			constant.ConfigurationRevision: "7",
		},
	}}
	cli := newRollbackFakeClient(t, configMap)
	requestCtx := intctrlutil.RequestCtx{Ctx: ctx, Log: log.FromContext(ctx)}
	result := reconciled(
		parameterreconfigure.Status{Status: parameterreconfigure.StatusFailed},
		"exec",
		parametersv1alpha1.CFailedAndPausePhase,
		withFailed(errors.New("rejected"), false),
	)

	_, err := updateConfigPhaseWithResult(cli, requestCtx, configMap, result)
	require.NoError(t, err)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(configMap), configMap))
	require.Equal(t, "true", configMap.Annotations[constant.DisableUpgradeInsConfigurationAnnotationKey])
	require.Equal(t, "7", configMap.Annotations[constant.ReconfigureFailureRevisionAnnotationKey])
}

var structConfigTemplate = appsv1.ComponentFileTemplate{Name: "mysql-config"}

func newRollbackFakeClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, parametersv1alpha1.AddToScheme(scheme))
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&parametersv1alpha1.ComponentParameter{}).
		WithObjects(objects...).
		Build()
}

func rollbackRequestCtx(ctx context.Context, object client.Object) intctrlutil.RequestCtx {
	return intctrlutil.RequestCtx{Ctx: ctx, Log: log.FromContext(ctx).WithValues("object", client.ObjectKeyFromObject(object))}
}
