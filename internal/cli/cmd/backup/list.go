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

package backup

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
	listBackupExample = templates.Examples(`
		# list all backups
		kbcli backup list

		# list all backups of specified cluster
		kbcli backup list --cluster mycluster
	`)
)

type backupListOptions struct {
	*list.ListOptions
	cluster string
}

func newListCommand(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &backupListOptions{ListOptions: list.NewListOptions(f, streams, types.BackupGVR())}
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "List backups.",
		Aliases:           []string{"ls"},
		Example:           listBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			o.Names = args
			util.CheckErr(backupListRun(o))
		},
	}
	o.AddFlags(cmd, true)
	cmd.Flags().StringVar(&o.cluster, "cluster", "", "List backups in the specified cluster")
	util.RegisterClusterCompletionFunc(cmd, f)

	return cmd
}

func backupListRun(o *backupListOptions) error {
	if o.cluster != "" {
		o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, []string{o.cluster})
	}

	// if format is JSON or YAML, use default printer to output the result.
	if o.Format == printer.JSON || o.Format == printer.YAML {
		_, err := o.Run()
		return err
	}

	// get and output the result
	o.Print = false
	r, err := o.Run()
	if err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Fprintln(o.IOStreams.Out, "No backup found")
		return nil
	}

	printRows := func(tbl *printer.TablePrinter) error {
		// sort backups with .status.StartTimestamp
		sort.SliceStable(infos, func(i, j int) bool {
			toBackup := func(idx int) *dataprotectionv1alpha1.Backup {
				backup := &dataprotectionv1alpha1.Backup{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(infos[idx].Object.(*unstructured.Unstructured).Object, backup); err != nil {
					return nil
				}
				return backup
			}
			iBackup := toBackup(i)
			jBackup := toBackup(j)
			if iBackup == nil {
				return true
			}
			if jBackup == nil {
				return false
			}
			return iBackup.Status.StartTimestamp.Time.Before(jBackup.Status.StartTimestamp.Time)
		})
		for _, info := range infos {
			backup := &dataprotectionv1alpha1.Backup{}
			obj := info.Object.(*unstructured.Unstructured)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, backup); err != nil {
				return err
			}
			sourceCluster := backup.Status.SourceCluster
			if sourceCluster == "" {
				sourceCluster = backup.Labels[constant.AppInstanceLabelKey]
			}
			durationStr := ""
			if backup.Status.Duration != nil {
				durationStr = duration.HumanDuration(backup.Status.Duration.Duration)
			}
			statusString := string(backup.Status.Phase)
			if backup.Status.Phase == dataprotectionv1alpha1.BackupRunning && backup.Status.AvailableReplicas != nil {
				statusString = fmt.Sprintf("%s(AvailablePods: %d)", statusString, *backup.Status.AvailableReplicas)
			}
			tbl.AddRow(
				backup.Name,
				backup.Namespace,
				sourceCluster,
				backup.Spec.BackupType,
				statusString,
				backup.Status.TotalSize,
				durationStr,
				util.TimeFormat(&backup.CreationTimestamp),
				util.TimeFormat(backup.Status.CompletionTimestamp),
				util.TimeFormat(backup.Status.Expiration))
		}
		return nil
	}

	if err = printer.PrintTable(o.Out, nil, printRows,
		"NAME", "NAMESPACE", "SOURCE-CLUSTER", "TYPE", "STATUS", "TOTAL-SIZE", "DURATION", "CREATE-TIME", "COMPLETION-TIME", "EXPIRATION"); err != nil {
		return err
	}
	return nil
}
