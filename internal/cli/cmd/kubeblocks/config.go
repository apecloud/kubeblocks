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

package kubeblocks

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var helmExpectValuesKey = []string{
	"image",
	"updateStrategy",
	"podDisruptionBudget",
	"loggerSettings",
	"serviceAccount",
	"securityContext",
	"podSecurityContext",
	"service",
	"serviceMonitor",
	"resources",
	"autoscaling",
	"nodeSelector",
	"affinity",
	"dataPlane",
	"admissionWebhooks",
	"dataProtection",
	"addonController",
	"topologySpreadConstraints",
	"tolerations",
	"priorityClassName",
	"nameOverride",
	"fullnameOverride",
	"dnsPolicy",
	"replicaCount",
	"hostNetwork",
	"keepAddons",
}

var backupConfigExample = templates.Examples(`
		# Enable the snapshot-controller and volume snapshot, to support snapshot backup.
		kbcli kubeblocks config --set snapshot-controller.enabled=true
        
		Options Parameters:
		# If you have already installed a snapshot-controller, only enable the snapshot backup feature
		dataProtection.enableVolumeSnapshot=true

		# the global pvc name which persistent volume claim to store the backup data.
		# replace the pvc name when it is empty in the backup policy.
		dataProtection.backupPVCName=backup-data
		
		# the init capacity of pvc for creating the pvc, e.g. 10Gi.
		# replace the init capacity when it is empty in the backup policy.
		dataProtection.backupPVCInitCapacity=100Gi

		# the pvc storage class name. replace the storageClassName when it is nil in the backup policy.
		dataProtection.backupPVCStorageClassName=csi-s3

		# the pvc create policy.
		# if the storageClass supports dynamic provisioning, recommend "IfNotPresent" policy.
		# otherwise, using "Never" policy. only affect the backupPolicy automatically created by KubeBlocks.
		dataProtection.backupPVCCreatePolicy=Never

		# the configmap name of the pv template. if the csi-driver not support dynamic provisioning,
		# you can provide a configmap which contains key "persistentVolume" and value of the persistentVolume struct.
		dataProtection.backupPVConfigMapName=pv-template

		# the configmap namespace of the pv template.
		dataProtection.backupPVConfigMapNamespace=default
	`)

var describeConfigExample = templates.Examples(`
		# Describe the KubeBlocks config.
		kbcli kubeblocks describe-config
`)

// NewConfigCmd creates the config command
func NewConfigCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
			Wait:      true,
		},
	}

	cmd := &cobra.Command{
		Use:     "config",
		Short:   "KubeBlocks config.",
		Example: backupConfigExample,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.Upgrade())
			util.CheckErr(markKubeBlocksPodsToLoadConfigMap(o.Client))
		},
	}
	helm.AddValueOptionsFlags(cmd.Flags(), &o.ValueOpts)
	return cmd
}

func NewDescribeConfigCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}
	var output printer.Format
	cmd := &cobra.Command{
		Use:     "describe-config",
		Short:   "describe KubeBlocks config.",
		Example: describeConfigExample,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(describeConfig(o, output, func(release string, opt *Options) (map[string]interface{}, error) {
				values, err := GetHelmValues(release, opt)
				if err != nil {
					return nil, err
				}
				res := make(map[string]interface{})
				// filter the addons values
				for i := range helmExpectValuesKey {
					res[helmExpectValuesKey[i]] = values[helmExpectValuesKey[i]]
				}
				return res, nil
			}))
		},
	}
	printer.AddOutputFlag(cmd, &output)
	return cmd
}

// GetHelmValues gives an implementation of 'helm get values' for target release
func GetHelmValues(release string, opt *Options) (map[string]interface{}, error) {
	if len(opt.HelmCfg.Namespace()) == 0 {
		namespace, err := util.GetKubeBlocksNamespace(opt.Client)
		if err != nil {
			return nil, err
		}
		opt.HelmCfg.SetNamespace(namespace)
	}
	actionConfig, err := helm.NewActionConfig(opt.HelmCfg)
	if err != nil {
		return nil, err
	}
	client := action.NewGetValues(actionConfig)
	client.AllValues = true
	return client.Run(release)
}

type fn func(release string, opt *Options) (map[string]interface{}, error)

// describeConfig will output the configs get by the fn in specified format
func describeConfig(o *InstallOptions, format printer.Format, f fn) error {
	values, err := f(types.KubeBlocksReleaseName, &o.Options)
	if err != nil {
		return err
	}
	printer.PrintHelmValues(values, format, o.Out)
	return nil
}

// markKubeBlocksPodsToLoadConfigMap marks an annotation of the KubeBlocks pods to load the projected volumes of configmap.
// kubelet periodically requeues the Pod after 60-90 seconds, exactly how long it takes Secret/ConfigMaps updates to be reflected to the volumes.
// so can modify the annotation of the pod to directly enter the coordination queue and make changes of the configmap to effective in a timely.
func markKubeBlocksPodsToLoadConfigMap(client kubernetes.Interface) error {
	deploy, err := util.GetKubeBlocksDeploy(client)
	if err != nil {
		return err
	}
	if deploy == nil {
		return nil
	}
	pods, err := client.CoreV1().Pods(deploy.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + types.KubeBlocksChartName,
	})
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return nil
	}
	condition := deploymentutil.GetDeploymentCondition(deploy.Status, appsv1.DeploymentProgressing)
	if condition == nil {
		return nil
	}
	podBelongToKubeBlocks := func(pod corev1.Pod) bool {
		for _, v := range pod.OwnerReferences {
			if v.Kind == constant.ReplicaSetKind && strings.Contains(condition.Message, v.Name) {
				return true
			}
		}
		return false
	}
	for _, pod := range pods.Items {
		belongToKubeBlocks := podBelongToKubeBlocks(pod)
		if !belongToKubeBlocks {
			continue
		}
		// mark the pod to load configmap
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[types.ReloadConfigMapAnnotationKey] = time.Now().Format(time.RFC3339Nano)
		_, _ = client.CoreV1().Pods(deploy.Namespace).Update(context.TODO(), &pod, metav1.UpdateOptions{})
	}
	return nil
}
