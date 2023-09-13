package backup

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	createBackupExample = templates.Examples(`
		# Create a backup for the cluster
		kbcli backup create mybackup --cluster mycluster

		# create a snapshot backup
		kbcli backup create mybackup --cluster mycluster --type snapshot

		# create a backup with specified policy
		kbcli backup create mybackup --cluster mycluster --policy mypolicy
	`)
)

func newCreateCommand(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	customOutPut := func(opt *create.CreateOptions) {
		output := fmt.Sprintf("Backup %s created successfully, you can view the progress:", opt.Name)
		printer.PrintLine(output)
		nextLine := fmt.Sprintf("\tkbcli backup list %s -n %s", opt.Name, opt.Namespace)
		printer.PrintLine(nextLine)
	}

	clusterName := ""

	o := &cluster.CreateBackupOptions{
		CreateOptions: create.CreateOptions{
			IOStreams:       streams,
			Factory:         f,
			GVR:             types.BackupGVR(),
			CueTemplateName: "backup_template.cue",
			CustomOutPut:    customOutPut,
		},
	}
	o.CreateOptions.Options = o

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a backup for the cluster.",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				o.BackupName = args[0]
			}
			if clusterName != "" {
				o.Args = []string{clusterName}
			}
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.CompleteBackup())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.BackupType, "type", "snapshot", "Backup type")
	cmd.Flags().StringVar(&clusterName, "cluster", "", "Cluster name")
	cmd.Flags().StringVar(&o.BackupPolicy, "policy", "", "Backup policy name, this flag will be ignored when backup-type is snapshot")
	util.RegisterClusterCompletionFunc(cmd, f)

	return cmd
}
