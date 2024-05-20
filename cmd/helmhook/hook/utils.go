/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package hook

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	errors "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func CheckErr(err error) {
	if err != nil {
		panic(err.Error())
	}
}

type deploymentGetter func(ctx context.Context, client *kubernetes.Clientset, ns string, componentName string) (*appsv1.Deployment, error)

// GetKubeBlocksDeploy gets deployment include KubeBlocks.
func GetKubeBlocksDeploy(ctx context.Context, client *kubernetes.Clientset, ns string, componentName string) (*appsv1.Deployment, error) {
	deployments, err := client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{
		LabelSelector: toLabelSelector(kubeBlocksSelectorLabels(componentName)),
	})

	if err != nil {
		return nil, errors.IgnoreNotFound(err)
	}
	if deployments == nil || len(deployments.Items) == 0 {
		return nil, nil
	}
	if len(deployments.Items) > 1 {
		return nil, fmt.Errorf("found multiple KubeBlocks deployments, please check your cluster")
	}

	return &deployments.Items[0], nil
}

// stopKubeBlocksDeploy gets deployment include KubeBlocks.
func stopKubeBlocksDeploy(ctx context.Context, client *kubernetes.Clientset, ns, componentName string, getter deploymentGetter) error {
	deploy, err := getter(ctx, client, ns, componentName)
	if err != nil {
		return err
	}

	if _, err = client.AppsV1().Deployments(deploy.Namespace).Patch(ctx, deploy.Name, types.JSONPatchType,
		[]byte(`[{"op": "replace", "path": "/spec/replicas", "value": 0}]`),
		metav1.PatchOptions{}); err != nil {
		return err
	}

	// wait for deployment to be stopped
	return wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 1*time.Minute, true,
		func(_ context.Context) (bool, error) {
			deploy, err = getter(ctx, client, ns, componentName)
			if err != nil {
				return false, err
			}
			if *deploy.Spec.Replicas == 0 &&
				deploy.Status.Replicas == 0 &&
				deploy.Status.AvailableReplicas == 0 {
				return true, nil
			}
			return false, nil
		})
}

func toLabelSelector(labels map[string]string) string {
	var keyValues []string
	for key, val := range labels {
		keyValues = append(keyValues, fmt.Sprintf("%s=%s", key, val))
	}
	return strings.Join(keyValues, ",")
}

func kubeBlocksSelectorLabels(componentName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:  constant.AppName,
		"app.kubernetes.io/component": componentName,
	}
}

func addonSelectorLabels() map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey: constant.AppName,
		constant.AppNameLabelKey:     constant.AppName,
	}
}
