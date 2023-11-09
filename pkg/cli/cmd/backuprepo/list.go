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

package backuprepo

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/list"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

var (
	listExample = templates.Examples(`
	# List all backup repositories
	kbcli backuprepo list
	`)
)

type listBackupRepoOptions struct {
	dynamic dynamic.Interface
	*list.ListOptions
}

func newListCommand(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &listBackupRepoOptions{
		ListOptions: list.NewListOptions(f, streams, types.BackupRepoGVR()),
	}
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "List Backup Repositories.",
		Aliases:           []string{"ls"},
		Example:           listExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.BackupRepoGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Names = args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(printBackupRepoList(o))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func (o *listBackupRepoOptions) Complete() error {
	var err error
	o.dynamic, err = o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	return nil
}

func printBackupRepoList(o *listBackupRepoOptions) error {
	// if format is JSON or YAML, use default printer to output the result.
	if o.Format == printer.JSON || o.Format == printer.YAML {
		_, err := o.Run()
		return err
	}

	backupRepoList, err := o.dynamic.Resource(types.BackupRepoGVR()).List(context.TODO(), metav1.ListOptions{
		LabelSelector: o.LabelSelector,
		FieldSelector: o.FieldSelector,
	})
	if err != nil {
		return err
	}

	if len(backupRepoList.Items) == 0 {
		fmt.Fprintln(o.IOStreams.Out, "No backup repository found")
		return nil
	}

	backupRepos := make([]*dpv1alpha1.BackupRepo, 0)
	for _, item := range backupRepoList.Items {
		backupRepo := &dpv1alpha1.BackupRepo{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, backupRepo); err != nil {
			return err
		}
		backupRepos = append(backupRepos, backupRepo)
	}

	printRows := func(tbl *printer.TablePrinter) error {
		// sort BackupRepos with isDefault, then phase and name
		sort.SliceStable(backupRepos, func(i, j int) bool {
			iBackupRepo := backupRepos[i]
			jBackupRepo := backupRepos[j]
			if iBackupRepo.Status.IsDefault != jBackupRepo.Status.IsDefault {
				return iBackupRepo.Status.IsDefault
			}
			if iBackupRepo.Status.Phase == jBackupRepo.Status.Phase {
				return iBackupRepo.GetName() < jBackupRepo.GetName()
			}
			return iBackupRepo.Status.Phase < jBackupRepo.Status.Phase
		})
		for _, backupRepo := range backupRepos {
			backups, backupSize, err := countBackupNumsAndSize(o.dynamic, backupRepo)
			if err != nil {
				return err
			}
			tbl.AddRow(backupRepo.Name,
				backupRepo.Status.Phase,
				backupRepo.Spec.StorageProviderRef,
				backupRepo.Spec.AccessMethod,
				backupRepo.Status.IsDefault,
				fmt.Sprintf("%d", backups),
				backupSize,
			)
		}
		return nil
	}

	if err = printer.PrintTable(o.Out, nil, printRows,
		"NAME", "STATUS", "STORAGE-PROVIDER", "ACCESS-METHOD", "DEFAULT", "BACKUPS", "TOTAL-SIZE"); err != nil {
		return err
	}
	return nil
}
