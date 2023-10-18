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

package cluster

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/cluster"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
)

var (
	describeExample = templates.Examples(`
		# describe a specified cluster
		kbcli cluster describe mycluster`)

	newTbl = func(out io.Writer, title string, header ...interface{}) *printer.TablePrinter {
		fmt.Fprintln(out, title)
		tbl := printer.NewTablePrinter(out)
		tbl.SetHeader(header...)
		return tbl
	}
)

type describeOptions struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string

	// resource type and names
	gvr   schema.GroupVersionResource
	names []string

	*cluster.ClusterObjects
	genericiooptions.IOStreams
}

func newOptions(f cmdutil.Factory, streams genericiooptions.IOStreams) *describeOptions {
	return &describeOptions{
		factory:   f,
		IOStreams: streams,
		gvr:       types.ClusterGVR(),
	}
}

func NewDescribeCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := newOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "describe NAME",
		Short:             "Show details of a specific cluster.",
		Example:           describeExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(args))
			util.CheckErr(o.run())
		},
	}
	return cmd
}

func (o *describeOptions) complete(args []string) error {
	var err error

	if len(args) == 0 {
		return fmt.Errorf("cluster name should be specified")
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
		if err := o.describeCluster(name); err != nil {
			return err
		}
	}
	return nil
}

func (o *describeOptions) describeCluster(name string) error {
	clusterGetter := cluster.ObjectsGetter{
		Client:    o.client,
		Dynamic:   o.dynamic,
		Name:      name,
		Namespace: o.namespace,
		GetOptions: cluster.GetOptions{
			WithClusterDef:     true,
			WithService:        true,
			WithPod:            true,
			WithPVC:            true,
			WithDataProtection: true,
		},
	}

	var err error
	if o.ClusterObjects, err = clusterGetter.Get(); err != nil {
		return err
	}

	// cluster summary
	showCluster(o.Cluster, o.Out)

	// show endpoints
	showEndpoints(o.Cluster, o.Services, o.Out)

	// topology
	showTopology(o.ClusterObjects.GetInstanceInfo(), o.Out)

	comps := o.ClusterObjects.GetComponentInfo()
	// resources
	showResource(comps, o.Out)

	// images
	showImages(comps, o.Out)

	// data protection info
	defaultBackupRepo, err := o.getDefaultBackupRepo()
	if err != nil {
		return err
	}
	showDataProtection(o.BackupPolicies, o.BackupSchedules, defaultBackupRepo, o.Out)

	// events
	showEvents(o.Cluster.Name, o.Cluster.Namespace, o.Out)
	fmt.Fprintln(o.Out)

	return nil
}

func (o *describeOptions) getDefaultBackupRepo() (string, error) {
	backupRepoListObj, err := o.dynamic.Resource(types.BackupRepoGVR()).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return printer.NoneString, err
	}
	for _, item := range backupRepoListObj.Items {
		repo := dpv1alpha1.BackupRepo{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &repo); err != nil {
			return printer.NoneString, err
		}
		if repo.Status.IsDefault {
			return repo.Name, nil
		}
	}
	return printer.NoneString, nil
}

func showCluster(c *appsv1alpha1.Cluster, out io.Writer) {
	if c == nil {
		return
	}
	title := fmt.Sprintf("Name: %s\t Created Time: %s", c.Name, util.TimeFormat(&c.CreationTimestamp))
	tbl := newTbl(out, title, "NAMESPACE", "CLUSTER-DEFINITION", "VERSION", "STATUS", "TERMINATION-POLICY")
	tbl.AddRow(c.Namespace, c.Spec.ClusterDefRef, c.Spec.ClusterVersionRef, string(c.Status.Phase), string(c.Spec.TerminationPolicy))
	tbl.Print()
}

func showTopology(instances []*cluster.InstanceInfo, out io.Writer) {
	tbl := newTbl(out, "\nTopology:", "COMPONENT", "INSTANCE", "ROLE", "STATUS", "AZ", "NODE", "CREATED-TIME")
	for _, ins := range instances {
		tbl.AddRow(ins.Component, ins.Name, ins.Role, ins.Status, ins.AZ, ins.Node, ins.CreatedTime)
	}
	tbl.Print()
}

func showResource(comps []*cluster.ComponentInfo, out io.Writer) {
	tbl := newTbl(out, "\nResources Allocation:", "COMPONENT", "DEDICATED", "CPU(REQUEST/LIMIT)", "MEMORY(REQUEST/LIMIT)", "STORAGE-SIZE", "STORAGE-CLASS")
	for _, c := range comps {
		tbl.AddRow(c.Name, "false", c.CPU, c.Memory, cluster.BuildStorageSize(c.Storage), cluster.BuildStorageClass(c.Storage))
	}
	tbl.Print()
}

func showImages(comps []*cluster.ComponentInfo, out io.Writer) {
	tbl := newTbl(out, "\nImages:", "COMPONENT", "TYPE", "IMAGE")
	for _, c := range comps {
		tbl.AddRow(c.Name, c.Type, c.Image)
	}
	tbl.Print()
}

func showEvents(name string, namespace string, out io.Writer) {
	// hint user how to get events
	fmt.Fprintf(out, "\nShow cluster events: kbcli cluster list-events -n %s %s", namespace, name)
}

func showEndpoints(c *appsv1alpha1.Cluster, svcList *corev1.ServiceList, out io.Writer) {
	if c == nil {
		return
	}

	tbl := newTbl(out, "\nEndpoints:", "COMPONENT", "MODE", "INTERNAL", "EXTERNAL")
	for _, comp := range c.Spec.ComponentSpecs {
		internalEndpoints, externalEndpoints := cluster.GetComponentEndpoints(svcList, &comp)
		if len(internalEndpoints) == 0 && len(externalEndpoints) == 0 {
			continue
		}
		tbl.AddRow(comp.Name, "ReadWrite", util.CheckEmpty(strings.Join(internalEndpoints, "\n")),
			util.CheckEmpty(strings.Join(externalEndpoints, "\n")))
	}
	tbl.Print()
}

func showDataProtection(backupPolicies []dpv1alpha1.BackupPolicy, backupSchedules []dpv1alpha1.BackupSchedule, defaultBackupRepo string, out io.Writer) {
	if len(backupPolicies) == 0 || len(backupSchedules) == 0 {
		return
	}
	tbl := newTbl(out, "\nData Protection:", "BACKUP-REPO", "AUTO-BACKUP", "BACKUP-SCHEDULE", "BACKUP-METHOD", "BACKUP-RETENTION")
	for _, schedule := range backupSchedules {
		backupRepo := defaultBackupRepo
		for _, policy := range backupPolicies {
			if policy.Name != schedule.Spec.BackupPolicyName {
				continue
			}
			if policy.Spec.BackupRepoName != nil {
				backupRepo = *policy.Spec.BackupRepoName
			}
		}
		for _, schedulePolicy := range schedule.Spec.Schedules {
			if !boolptr.IsSetToTrue(schedulePolicy.Enabled) {
				continue
			}

			tbl.AddRow(backupRepo, "Enabled", schedulePolicy.CronExpression, schedulePolicy.BackupMethod, schedulePolicy.RetentionPeriod.String())
		}
	}
	tbl.Print()
}

//	 getBackupRecoverableTime returns the recoverable time range string
//	func getBackupRecoverableTime(backups []dpv1alpha1.Backup) string {
//	recoverabelTime := dpv1alpha1.GetRecoverableTimeRange(backups)
//	var result string
//	for _, i := range recoverabelTime {
//		result = addTimeRange(result, i.StartTime, i.StopTime)
//	}
//	if result == "" {
//		return printer.NoneString
//	}
//	return result
//	}

//	func addTimeRange(result string, start, end *metav1.Time) string {
//		if result != "" {
//			result += ", "
//		}
//		result += fmt.Sprintf("%s ~ %s", util.TimeFormatWithDuration(start, time.Second),
//			util.TimeFormatWithDuration(end, time.Second))
//		return result
//	}
