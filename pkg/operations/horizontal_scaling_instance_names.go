/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

package operations

import (
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
)

// Deprecated: should use instancetemplate.PodNameBuilder.
func generateAllPodNamesToSet(
	compReplicas int32,
	instances []appsv1.InstanceTemplate,
	offlineInstances []string,
	clusterName,
	fullCompName string) (map[string]string, error) {
	compName := constant.GenerateClusterComponentName(clusterName, fullCompName)
	instanceNames, err := generateAllPodNames(compReplicas, instances, offlineInstances, compName)
	if err != nil {
		return nil, err
	}
	instanceSet := map[string]string{}
	for _, insName := range instanceNames {
		instanceSet[insName] = appsv1.GetInstanceTemplateName(clusterName, fullCompName, insName)
	}
	return instanceSet, nil
}

func generateAllPodNames(
	compReplicas int32,
	instances []appsv1.InstanceTemplate,
	offlineInstances []string,
	fullCompName string) ([]string, error) {
	var templates []instanceset.InstanceTemplate
	for i := range instances {
		templates = append(templates, &workloads.InstanceTemplate{
			Name:     instances[i].Name,
			Replicas: instances[i].Replicas,
			Ordinals: instances[i].Ordinals,
		})
	}
	return instanceset.GenerateAllInstanceNames(fullCompName, compReplicas, templates, offlineInstances, appsv1.Ordinals{})
}
