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

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

const (
	podNameList       = "podNameList"
	podIPList         = "podIPList"
	podHostNameList   = "podHostNameList"
	podHostIPList     = "podHostIPList"
	compName          = "compName"
	compReplicas      = "compReplicas"
	deletingCompList  = "deletingCompList"
	undeletedCompList = "undeletedCompList"
)

func getClusterPods(ctx context.Context, cli client.Reader, namespace, clusterName string) (map[string]string, error) {
	// TODO
	pods, err := component.ListOwnedPods(ctx, cli, namespace, clusterName, "")
	if err != nil {
		return nil, err
	}
	name := func(pod *corev1.Pod) string {
		return pod.Name
	}
	IP := func(pod *corev1.Pod) string {
		return pod.Status.PodIP
	}
	hostName := func(pod *corev1.Pod) string {
		return pod.Spec.Hostname
	}
	hostIP := func(pod *corev1.Pod) string {
		return pod.Status.HostIP
	}
	return map[string]string{
		podNameList:     strings.Join(traverse(pods, name), ","),
		podIPList:       strings.Join(traverse(pods, IP), ","),
		podHostNameList: strings.Join(traverse(pods, hostName), ","),
		podHostIPList:   strings.Join(traverse(pods, hostIP), ","),
	}, nil
}

func getCompPods(ctx context.Context, cli client.Reader, namespace, clusterName, compName string) (map[string]string, error) {
	pods, err := component.ListOwnedPods(ctx, cli, namespace, clusterName, compName)
	if err != nil {
		return nil, err
	}
	name := func(pod *corev1.Pod) string {
		return pod.Name
	}
	IP := func(pod *corev1.Pod) string {
		return pod.Status.PodIP
	}
	hostName := func(pod *corev1.Pod) string {
		return pod.Spec.Hostname
	}
	hostIP := func(pod *corev1.Pod) string {
		return pod.Status.HostIP
	}
	return map[string]string{
		podNameList:     strings.Join(traverse(pods, name), ","),
		podIPList:       strings.Join(traverse(pods, IP), ","),
		podHostNameList: strings.Join(traverse(pods, hostName), ","),
		podHostIPList:   strings.Join(traverse(pods, hostIP), ","),
	}, nil
}

func getComps(ctx context.Context, cli client.Reader, namespace, clusterName string) (map[string]string, error) {
	cluster := &appsv1alpha1.Cluster{}
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      clusterName,
	}
	if err := cli.Get(ctx, key, cluster); err != nil {
		return nil, err
	}

	name := func(spec appsv1alpha1.ClusterComponentSpec) string {
		return spec.Name
	}
	replicas := func(spec appsv1alpha1.ClusterComponentSpec) string {
		return fmt.Sprintf("%d", spec.Replicas)
	}
	components := cluster.Status.Components
	var deletingComps []string
	var undeletedComps []string
	for k, v := range components {
		if v.Phase == appsv1alpha1.DeletingClusterCompPhase {
			deletingComps = append(deletingComps, k)
		} else {
			undeletedComps = append(undeletedComps, k)
		}
	}
	return map[string]string{
		compName:          strings.Join(traverse(cluster.Spec.ComponentSpecs, name), ","),
		compReplicas:      strings.Join(traverse(cluster.Spec.ComponentSpecs, replicas), ","),
		deletingCompList:  strings.Join(deletingComps, ","),
		undeletedCompList: strings.Join(undeletedComps, ","),
	}, nil
}

func getServiceableNWritablePod(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent) (*corev1.Pod, error) {
	if synthesizedComp.Roles == nil {
		return nil, errors.New("component does not support switchover")
	}

	targetRole := ""
	for _, role := range synthesizedComp.Roles {
		if role.Serviceable && role.Writable {
			if targetRole != "" {
				return nil, errors.New("component has more than role is serviceable and writable, does not support switchover")
			}
			targetRole = role.Name
		}
	}
	if targetRole == "" {
		return nil, errors.New("component has no role is serviceable and writable, does not support switchover")
	}

	pods, err := component.ListOwnedPodsWithRole(ctx, cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, targetRole)
	if err != nil {
		return nil, err
	}
	if len(pods) != 1 {
		return nil, errors.New("component pod list is empty or has more than one serviceable and writable pod")
	}
	return pods[0], nil
}

func getScaledInFlag(ctx context.Context, cli client.Reader, namespace, clusterName, compName string) (bool, error) {
	cluster := &appsv1alpha1.Cluster{}
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      clusterName,
	}
	if err := cli.Get(ctx, key, cluster); err != nil {
		return false, err
	}
	compList := &appsv1alpha1.ComponentList{}
	if err := cli.List(ctx, compList, client.InNamespace(namespace), client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}); err != nil {
		return false, err
	}
	for _, comp := range compList.Items {
		if comp.Name == compName {
			_, ok := comp.Annotations[constant.ComponentScaleInAnnotationKey]
			if ok {
				return true, nil
			}
		}
	}
	return false, nil
}

func getDBEnvs(synthesizeComp *component.SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) []corev1.EnvVar {
	var (
		secretName     string
		sysInitAccount *appsv1alpha1.SystemAccount
	)

	for index, sysAccount := range synthesizeComp.SystemAccounts {
		// use first init account
		if sysAccount.InitAccount {
			sysInitAccount = &synthesizeComp.SystemAccounts[index]
			break
		}
	}

	if sysInitAccount != nil {
		secretName = constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, sysInitAccount.Name)
	}

	var envs []corev1.EnvVar
	if secretName == "" {
		return envs
	}

	envs = append(envs,
		corev1.EnvVar{
			Name: serviceUserVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountNameForSecret,
				},
			},
		},
		corev1.EnvVar{
			Name: servicePasswordVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountPasswdForSecret,
				},
			},
		})
	return envs
}

func getMainContainer(containers []corev1.Container) *corev1.Container {
	if len(containers) > 0 {
		return &containers[0]
	}
	return nil
}

func traverse[T any, R any](items []T, f func(T) R) []R {
	var result []R
	for _, item := range items {
		result = append(result, f(item))
	}
	return result
}
