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

package component

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestHandleTemplateObjectChangesPreservesExternalAnnotations(t *testing.T) {
	const (
		namespace   = "default"
		compDefName = "test-compdef"
		clusterName = "test-cluster"
		compName    = "comp"
	)

	templateCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "mytemplate",
		},
		Data: map[string]string{
			"my.cnf": "key=value",
		},
	}

	synthComp := &component.SynthesizedComponent{
		Namespace:    namespace,
		ClusterName:  clusterName,
		Name:         compName,
		FullCompName: fmt.Sprintf("%s-%s", clusterName, compName),
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
		FileTemplates: []component.SynthesizedFileTemplate{
			{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name:       "sysconf",
					Template:   templateCM.Name,
					Namespace:  namespace,
					VolumeName: "sysconf",
				},
			},
		},
	}

	objName := fileTemplateObjectName(synthComp, "sysconf")

	compLabels := constant.GetCompLabels(clusterName, compName)
	compLabels[kubeBlockFileTemplateLabelKey] = "true"
	compLabels[constant.CMConfigurationSpecProviderLabelKey] = "sysconf"
	compLabels[constant.CMConfigurationTemplateNameLabelKey] = templateCM.Name

	runningCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      objName,
			Labels:    compLabels,
			Annotations: map[string]string{
				constant.CMInsConfigurationHashLabelKey:              "old-hash",
				constant.LastAppliedConfigAnnotationKey:              `{"my.cnf":"key=old-value"}`,
				constant.ConfigurationRevision:                      "5",
				constant.ConfigAppliedVersionAnnotationKey:           `{"name":"sysconf"}`,
				constant.ParametersAppliedComponentGenerationKey:     "3",
				"config.kubeblocks.io/revision-reconcile-phase-5":   `{"phase":"Upgrading","revision":"5","policy":"syncReload","execResult":"Retry","succeedCount":0,"expectedCount":3}`,
			},
		},
		Data: map[string]string{
			"my.cnf": "key=value",
		},
	}

	comp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      constant.GenerateClusterComponentName(clusterName, compName),
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: compName,
			},
		},
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-%s", clusterName, compName),
		},
	}

	reader := &appsutil.MockReader{
		Objects: []client.Object{its, templateCM, runningCM},
	}

	graphCli := model.NewGraphClient(reader)
	dag := graph.NewDAG()
	graphCli.Root(dag, comp, comp, model.ActionStatusPtr())

	transCtx := &componentTransformContext{
		Context:       context.Background(),
		Client:        graphCli,
		Logger:        log.Log.WithName("test"),
		CompDef:       &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: compDefName}},
		Component:     comp,
		ComponentOrig: comp.DeepCopy(),
		SynthesizeComponent: synthComp,
	}

	transformer := &componentFileTemplateTransformer{}
	if err := transformer.Transform(transCtx, dag); err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	objs := graphCli.FindAll(dag, &corev1.ConfigMap{})
	var protoObj *corev1.ConfigMap
	for _, obj := range objs {
		if obj.GetName() == objName {
			protoObj = obj.(*corev1.ConfigMap)
			break
		}
	}
	if protoObj == nil {
		t.Fatalf("proto ConfigMap %s not found in DAG", objName)
	}

	checkAnnotation := func(key, wantValue string) {
		t.Helper()
		got, ok := protoObj.Annotations[key]
		if !ok {
			t.Errorf("annotation %s missing from proto ConfigMap", key)
			return
		}
		if wantValue != "" && got != wantValue {
			t.Errorf("annotation %s = %q, want %q", key, got, wantValue)
		}
	}

	checkLabel := func(key, wantValue string) {
		t.Helper()
		got, ok := protoObj.Labels[key]
		if !ok {
			t.Errorf("label %s missing from proto ConfigMap", key)
			return
		}
		if wantValue != "" && got != wantValue {
			t.Errorf("label %s = %q, want %q", key, got, wantValue)
		}
	}

	checkAnnotation(constant.LastAppliedConfigAnnotationKey, `{"my.cnf":"key=old-value"}`)
	checkAnnotation(constant.ConfigurationRevision, "5")
	checkAnnotation(constant.ConfigAppliedVersionAnnotationKey, `{"name":"sysconf"}`)
	checkAnnotation(constant.ParametersAppliedComponentGenerationKey, "3")
	checkAnnotation("config.kubeblocks.io/revision-reconcile-phase-5", "")

	checkLabel(constant.CMConfigurationSpecProviderLabelKey, "sysconf")
	checkLabel(constant.CMConfigurationTemplateNameLabelKey, templateCM.Name)
}
