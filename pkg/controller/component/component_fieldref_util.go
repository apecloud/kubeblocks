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

package component

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	jsonpath "k8s.io/client-go/util/jsonpath"
	klog "k8s.io/klog/v2"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func buildComponentRef(clusterDef *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterComp *appsv1alpha1.ClusterComponentSpec,
	component *SynthesizedComponent) error {

	compRefs := clusterCompDef.ComponentDefRef
	if len(compRefs) == 0 {
		return nil
	}

	component.ComponentRefEnvs = make([]*corev1.EnvVar, 0)

	for _, compRef := range compRefs {
		referredComponentDef := clusterDef.GetComponentDefByName(compRef.ComponentDefName)
		referredComponents := cluster.Spec.GetDefNameMappingComponents()[compRef.ComponentDefName]

		if referredComponentDef == nil || len(referredComponents) == 0 {
			err := fmt.Errorf("failes to match %s in cluster %s", compRef.ComponentDefName, cluster.Name)
			if compRef.FailurePolicy == appsv1alpha1.FailurePolicyFail {
				return err
			} else {
				klog.V(1).Info(err.Error())
				continue
			}
		}

		envMap := make(map[string]string)
		for _, refEnv := range compRef.ComponentRefEnvs {
			env := &corev1.EnvVar{Name: refEnv.Name}
			var err error
			if len(refEnv.Value) != 0 {
				env.Value = refEnv.Value
			} else if refEnv.ValueFrom != nil {
				switch refEnv.ValueFrom.Type {
				case appsv1alpha1.FromFieldRef:
					if env.Value, err = resolveFieldRef(refEnv.ValueFrom, referredComponents, referredComponentDef); err != nil {
						return err
					}
				case appsv1alpha1.FromServiceRef:
					if env.Value, err = resolveServiceRef(cluster.Name, referredComponents, referredComponentDef); err != nil {
						return err
					}
				case appsv1alpha1.FromHeadlessServiceRef:
					if referredComponentDef.WorkloadType == appsv1alpha1.Stateless {
						errMsg := fmt.Sprintf("headless service ref is not supported for stateless component, cluster: %s, referred component: %s", cluster.Name, referredComponentDef.Name)
						klog.V(1).Infof(errMsg)
						if compRef.FailurePolicy == appsv1alpha1.FailurePolicyFail {
							return fmt.Errorf(errMsg)
						}
					}
					env.Value = resolveHeadlessServiceFieldRef(refEnv.ValueFrom, cluster, referredComponents)
				}
			}

			component.ComponentRefEnvs = append(component.ComponentRefEnvs, env)
			envMap[env.Name] = env.Value
		}

		// for each env in componentRefEnvs, resolve reference
		for _, env := range component.ComponentRefEnvs {
			val := env.Value
			for k, v := range envMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("$(%s)", k), v)
			}
			env.Value = val
		}
	}
	return nil
}

type referredObject struct {
	ComponentDef *appsv1alpha1.ClusterComponentDefinition `json:"componentDef"`
	Components   []appsv1alpha1.ClusterComponentSpec      `json:"components"`
}

func resolveFieldRef(valueFrom *appsv1alpha1.ComponentValueFrom, components []appsv1alpha1.ClusterComponentSpec, componentDef *appsv1alpha1.ClusterComponentDefinition) (string, error) {
	referred := referredObject{
		ComponentDef: componentDef,
		Components:   components,
	}

	if value, err := retrieveValueByJSONPath(referred, valueFrom.FieldPath); err != nil {
		return "", err
	} else {
		return string(value), nil
	}
}

func resolveServiceRef(clusterName string, components []appsv1alpha1.ClusterComponentSpec, componentDef *appsv1alpha1.ClusterComponentDefinition) (string, error) {
	if componentDef.Service == nil {
		return "", fmt.Errorf("componentDef %s does not have service", componentDef.Name)
	}
	if len(components) != 1 {
		return "", fmt.Errorf("expect one component but got %d for componentDef %s", len(components), componentDef.Name)
	}
	return fmt.Sprintf("%s-%s", clusterName, components[0].Name), nil
}

func resolveHeadlessServiceFieldRef(valueFrom *appsv1alpha1.ComponentValueFrom,
	cluster *appsv1alpha1.Cluster, components []appsv1alpha1.ClusterComponentSpec) string {

	preDefineVars := []string{"POD_NAME", "POD_FQDN", "POD_ORDINAL"}

	format := valueFrom.Format
	if len(format) == 0 {
		format = "$(POD_FQDN)"
	}
	joinWith := valueFrom.JoinWith
	if len(joinWith) == 0 {
		joinWith = ","
	}

	hosts := make([]string, 0)
	for _, comp := range components {
		for i := int32(0); i < comp.Replicas; i++ {
			qualifiedName := fmt.Sprintf("%s-%s", cluster.Name, comp.Name)
			podOrdinal := strconv.Itoa(int(i))
			podName := fmt.Sprintf("%s-%s", qualifiedName, podOrdinal)
			podFQDN := fmt.Sprintf("%s.%s-headless.%s.svc", podName, qualifiedName, cluster.Namespace)

			valuesToReplace := []string{podName, podFQDN, podOrdinal}

			host := format
			for idx, preDefineVar := range preDefineVars {
				host = strings.ReplaceAll(host, "$("+preDefineVar+")", valuesToReplace[idx])
			}
			hosts = append(hosts, host)
		}
	}
	return strings.Join(hosts, joinWith)
}

func retrieveValueByJSONPath(jsonObj interface{}, jpath string) ([]byte, error) {
	path := jsonpath.New("jsonpath")
	if err := path.Parse(fmt.Sprintf("{%s}", jpath)); err != nil {
		return nil, fmt.Errorf("failed to parse jsonpath %s", jpath)
	}
	buff := bytes.NewBuffer([]byte{})
	if err := path.Execute(buff, jsonObj); err != nil {
		return nil, fmt.Errorf("failed to execute jsonpath %s", jpath)
	}
	return buff.Bytes(), nil
}
