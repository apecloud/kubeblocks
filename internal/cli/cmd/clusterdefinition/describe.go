/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package clusterdefinition

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
)

var (
	describeExample = templates.Examples(`
		# describe a specified cluster definition
		kbcli clusterdefinition describe myclusterdef`)
)

type describeOptions struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string

	names []string
	genericclioptions.IOStreams
}

func NewDescribeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &describeOptions{
		factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:               "describe",
		Short:             "Describe ClusterDefinition.",
		Example:           describeExample,
		Aliases:           []string{"desc"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterDefGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.run())
		},
	}
	return cmd
}

func (o *describeOptions) complete(args []string) error {
	var err error

	if len(args) == 0 {
		return fmt.Errorf("cluster definition name should be specified")
	}
	o.names = args

	if o.client, err = o.factory.KubernetesClientSet(); err != nil {
		return err
	}

	if o.dynamic, err = o.factory.DynamicClient(); err != nil {
		return err
	}

	if o.namespace, _, err = o.factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	return nil
}

func (o *describeOptions) run() error {
	for _, name := range o.names {
		if err := o.describeClusterDef(name); err != nil {
			return err
		}
	}
	return nil
}

func (o *describeOptions) describeClusterDef(name string) error {
	// get cluster definition
	clusterDefObject, err := o.dynamic.Resource(types.ClusterDefGVR()).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterDef := v1alpha1.ClusterDefinition{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(clusterDefObject.Object, &clusterDef); err != nil {
		return err
	}

	// get backup policy templates of the cluster definition
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", constant.ClusterDefLabelKey, name),
	}
	backupTemplatesListObj, err := o.dynamic.Resource(types.BackupPolicyTemplateGVR()).List(context.TODO(), opts)
	if err != nil {
		return err
	}
	var backupPolicyTemplates []*v1alpha1.BackupPolicyTemplate
	for _, item := range backupTemplatesListObj.Items {
		backupTemplate := v1alpha1.BackupPolicyTemplate{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &backupTemplate); err != nil {
			return err
		}
		backupPolicyTemplates = append(backupPolicyTemplates, &backupTemplate)
	}

	showClusterDef(&clusterDef, o.Out)

	showBackupConfig(backupPolicyTemplates, o.Out)

	return nil
}

func showClusterDef(cd *v1alpha1.ClusterDefinition, out io.Writer) {
	if cd == nil {
		return
	}
	fmt.Fprintf(out, "Name: %s\t Type: %s\n\n", cd.Name, cd.Spec.Type)
}

func showBackupConfig(backupPolicyTemplates []*v1alpha1.BackupPolicyTemplate, out io.Writer) {
	if len(backupPolicyTemplates) == 0 {
		return
	}
	fmt.Fprintf(out, "Backup Config:\n")
	tbl := printer.NewTablePrinter(out)
	tbl.SetHeader("AUTO-BACKUP", "BACKUP-SCHEDULE", "BACKUP-METHOD", "BACKUP-RETENTION")
	defaultBackupPolicyTemplate := &v1alpha1.BackupPolicyTemplate{}
	for _, item := range backupPolicyTemplates {
		if item.Annotations[dptypes.DefaultBackupPolicyTemplateAnnotationKey] == "true" {
			defaultBackupPolicyTemplate = item
			break
		}
	}
	for _, policy := range defaultBackupPolicyTemplate.Spec.BackupPolicies {
		for _, schedule := range policy.Schedules {
			scheduleEnable := "Disabled"
			if schedule.Enabled != nil && *schedule.Enabled {
				scheduleEnable = "Enabled"
			}
			tbl.AddRow(scheduleEnable, schedule.CronExpression, schedule.BackupMethod, policy.RetentionPeriod)
		}
	}
	tbl.Print()
}
