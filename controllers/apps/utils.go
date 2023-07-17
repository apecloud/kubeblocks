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

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	celgo "github.com/google/cel-go/cel"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// default reconcile requeue after duration
var requeueDuration = time.Millisecond * 1000

func getEnvReplacementMapForAccount(name, passwd string) map[string]string {
	return map[string]string{
		"$(USERNAME)": name,
		"$(PASSWD)":   passwd,
	}
}

// notifyClusterStatusChange notifies cluster changes occurred and triggers it to reconcile.
func notifyClusterStatusChange(ctx context.Context, cli client.Client, recorder record.EventRecorder, obj client.Object, event *corev1.Event) error {
	if obj == nil || !intctrlutil.WorkloadFilterPredicate(obj) {
		return nil
	}

	cluster, ok := obj.(*appsv1alpha1.Cluster)
	if !ok {
		var err error
		if cluster, err = components.GetClusterByObject(ctx, cli, obj); err != nil {
			return err
		}
	}

	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[constant.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
	if err := cli.Patch(ctx, cluster, patch); err != nil {
		return err
	}

	if recorder != nil && event != nil {
		recorder.Eventf(cluster, corev1.EventTypeWarning, event.Reason, getFinalEventMessageForRecorder(event))
	}
	return nil
}

// getFinalEventMessageForRecorder gets final event message by event involved object kind for recorded it
func getFinalEventMessageForRecorder(event *corev1.Event) string {
	if event.InvolvedObject.Kind == constant.PodKind {
		return fmt.Sprintf("Pod %s: %s", event.InvolvedObject.Name, event.Message)
	}
	return event.Message
}

func mergeClusterTemplates(cts []appsv1alpha1.ClusterTemplate) *appsv1alpha1.ClusterTemplate {
	var finalClusterTpl appsv1alpha1.ClusterTemplate
	for i, ct := range cts {
		if i == 0 {
			finalClusterTpl = ct
			continue
		}
		for _, comp := range ct.Spec.ComponentSpecs {
			compSpec := finalClusterTpl.Spec.GetComponentByName(comp.Name)
			if compSpec == nil {
				finalClusterTpl.Spec.ComponentSpecs = append(finalClusterTpl.Spec.ComponentSpecs, comp)
			}
		}
	}
	return &finalClusterTpl
}

func getTemplateNamesFromCF(ctx context.Context, cf *appsv1alpha1.ClusterFamily, cluster *appsv1alpha1.Cluster) ([]string, error) {
	var tplNames []string
	for _, ref := range cf.Spec.ClusterTemplateRefs {
		exp := ref.Key
		if len(ref.Expression) > 0 {
			exp = ref.Expression
		}
		if len(exp) == 0 {
			tplNames = append(tplNames, ref.TemplateRef)
			continue
		}
		res, err := evalCEL(ctx, exp, cluster)
		if err != nil {
			// ignore errors if key not exists
			if strings.Contains(res, "no such key") {
				continue
			}
			return nil, err
		}
		if res == ref.Value {
			tplNames = append(tplNames, ref.TemplateRef)
		}
	}
	return tplNames, nil
}

func evalCEL(ctx context.Context, exp string, cluster *appsv1alpha1.Cluster) (string, error) {
	env, err := celgo.NewEnv(celgo.Variable("cluster", celgo.AnyType))
	if err != nil {
		return "", err
	}
	ast, iss := env.Compile(exp)
	if iss.Err() != nil {
		return "", iss.Err()
	}
	prg, err := env.Program(ast, celgo.EvalOptions(celgo.OptOptimize|celgo.OptTrackState), celgo.InterruptCheckFrequency(100))
	if err != nil {
		return "", err
	}
	clusterByte, err := json.Marshal(cluster)
	if err != nil {
		return "", err
	}
	clusterMap := map[string]any{}
	if err := json.Unmarshal(clusterByte, &clusterMap); err != nil {
		return "", err
	}
	out, _, err := prg.ContextEval(ctx, map[string]any{"cluster": clusterMap})
	return fmt.Sprintf("%v", out.Value()), err
}
