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

package accounts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dapr/components-contrib/bindings"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	clusterutil "github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/sqlchannel"
)

type AccountBaseOptions struct {
	ClusterName   string
	CharType      string
	ComponentName string
	PodName       string
	Pod           *corev1.Pod
	Verbose       bool
	AccountOp     bindings.OperationKind
	RequestMeta   map[string]interface{}
	*exec.ExecOptions
}

var (
	errClusterNameNum        = fmt.Errorf("please specify ONE cluster-name at a time")
	errMissingUserName       = fmt.Errorf("please specify username")
	errMissingRoleName       = fmt.Errorf("please specify at least ONE role name")
	errInvalidRoleName       = fmt.Errorf("invalid role name, should be one of [SUPERUSER, READWRITE, READONLY] ")
	errInvalidOp             = fmt.Errorf("invalid operation")
	errCompNameOrInstName    = fmt.Errorf("please specify either --component or --instance, not both")
	errClusterNameorInstName = fmt.Errorf("specify either cluster name or --instance")
)

func NewAccountBaseOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, op bindings.OperationKind) *AccountBaseOptions {
	return &AccountBaseOptions{
		ExecOptions: exec.NewExecOptions(f, streams),
		AccountOp:   op,
	}
}

func (o *AccountBaseOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.ComponentName, "component", "", "Specify the name of component to be connected. If not specified, the first component will be used.")
	cmd.Flags().StringVarP(&o.PodName, "instance", "i", "", "Specify the name of instance to be connected.")
}

func (o *AccountBaseOptions) Validate(args []string) error {
	if len(args) > 1 {
		return errClusterNameNum
	}

	if len(o.PodName) > 0 {
		if len(o.ComponentName) > 0 {
			return errCompNameOrInstName
		}
		if len(args) > 0 {
			return errClusterNameorInstName
		}
	} else if len(args) == 0 {
		return errClusterNameorInstName
	}
	if len(args) == 1 {
		o.ClusterName = args[0]
	}
	return nil
}

func (o *AccountBaseOptions) Complete(f cmdutil.Factory) error {
	var err error
	err = o.ExecOptions.Complete()
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	if len(o.PodName) > 0 {
		// get pod by name
		o.Pod, err = o.ExecOptions.Client.CoreV1().Pods(o.Namespace).Get(ctx, o.PodName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		o.ClusterName = clusterutil.GetPodClusterName(o.Pod)
		o.ComponentName = clusterutil.GetPodComponentName(o.Pod)
	}

	compInfo, err := clusterutil.FillCompInfoByName(ctx, o.ExecOptions.Dynamic, o.Namespace, o.ClusterName, o.ComponentName)
	if err != nil {
		return err
	}
	// fill component name
	if len(o.ComponentName) == 0 {
		o.ComponentName = compInfo.Component.Name
	}
	// fill character type
	o.CharType = compInfo.ComponentDef.CharacterType

	if len(o.PodName) == 0 {
		if o.PodName, err = compInfo.InferPodName(); err != nil {
			return err
		}
		// get pod by name
		o.Pod, err = o.ExecOptions.Client.CoreV1().Pods(o.Namespace).Get(ctx, o.PodName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}

	o.ExecOptions.Pod = o.Pod
	o.ExecOptions.Namespace = o.Namespace
	o.ExecOptions.Quiet = true
	o.ExecOptions.TTY = true
	o.ExecOptions.Stdin = true

	o.Verbose = klog.V(1).Enabled()

	return nil
}

func (o *AccountBaseOptions) Run(cmd *cobra.Command, f cmdutil.Factory, streams genericclioptions.IOStreams) error {
	var err error
	response, err := o.Do()
	if err != nil {
		if sqlchannel.IsUnSupportedError(err) {
			return fmt.Errorf("command `%s` on characterType `%s` (defined in cluster: %s, component: %s) is not supported yet", cmd.Use, o.CharType, o.ClusterName, o.ComponentName)
		}
		return err
	}

	switch o.AccountOp {
	case
		sqlchannel.DeleteUserOp,
		sqlchannel.RevokeUserRoleOp,
		sqlchannel.GrantUserRoleOp:
		o.printGeneralInfo(response)
		err = nil
	case sqlchannel.CreateUserOp:
		o.printGeneralInfo(response)
		if response.Event == sqlchannel.RespEveSucc {
			printer.Alert(o.Out, "Please do REMEMBER the password for the new user! Once forgotten, it cannot be retrieved!\n")
		}
		err = nil
	case sqlchannel.DescribeUserOp:
		err = o.printRoleInfo(response)
	case sqlchannel.ListUsersOp:
		err = o.printUserInfo(response)
	default:
		err = errInvalidOp
	}
	if err != nil {
		return err
	}

	if o.Verbose {
		fmt.Fprintln(o.Out, "")
		o.printMeta(response)
	}
	return err
}

func (o *AccountBaseOptions) Do() (sqlchannel.SQLChannelResponse, error) {
	klog.V(1).Info(fmt.Sprintf("connect to cluster %s, component %s, instance %s\n", o.ClusterName, o.ComponentName, o.PodName))
	response := sqlchannel.SQLChannelResponse{}
	sqlClient, err := sqlchannel.NewHTTPClientWithChannelPod(o.Pod, o.CharType)
	if err != nil {
		return response, err
	}

	request := sqlchannel.SQLChannelRequest{Operation: (string)(o.AccountOp), Metadata: o.RequestMeta}
	response, err = sqlClient.SendRequest(o.ExecOptions, request)
	return response, err
}

func (o *AccountBaseOptions) newTblPrinterWithStyle(title string, header []interface{}) *printer.TablePrinter {
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetStyle(printer.TerminalStyle)
	// tblPrinter.Tbl.SetTitle(title)
	tblPrinter.SetHeader(header...)
	return tblPrinter
}

func (o *AccountBaseOptions) printGeneralInfo(response sqlchannel.SQLChannelResponse) {
	tblPrinter := o.newTblPrinterWithStyle("QUERY RESULT", []interface{}{"RESULT", "MESSAGE"})
	tblPrinter.AddRow(response.Event, response.Message)
	tblPrinter.Print()
}

func (o *AccountBaseOptions) printMeta(response sqlchannel.SQLChannelResponse) {
	meta := response.Metadata
	tblPrinter := o.newTblPrinterWithStyle("QUERY META", []interface{}{"START TIME", "END TIME", "OPERATION", "DATA"})
	tblPrinter.SetStyle(printer.KubeCtlStyle)
	tblPrinter.AddRow(util.TimeTimeFormat(meta.StartTime), util.TimeTimeFormat(meta.EndTime), meta.Operation, meta.Extra)
	tblPrinter.Print()
}

func (o *AccountBaseOptions) printUserInfo(response sqlchannel.SQLChannelResponse) error {
	if response.Event == sqlchannel.RespEveFail {
		o.printGeneralInfo(response)
		return nil
	}
	// decode user info from metadata
	users := []sqlchannel.UserInfo{}
	err := json.Unmarshal([]byte(response.Message), &users)
	if err != nil {
		return err
	}

	// render user info with username and password expired boolean
	tblPrinter := o.newTblPrinterWithStyle("USER INFO", []interface{}{"USERNAME", "EXPIRED"})
	for _, user := range users {
		tblPrinter.AddRow(user.UserName, user.Expired)
	}

	tblPrinter.Print()
	return nil
}

func (o *AccountBaseOptions) printRoleInfo(response sqlchannel.SQLChannelResponse) error {
	if response.Event == sqlchannel.RespEveFail {
		o.printGeneralInfo(response)
		return nil
	}

	// decode role info from metadata
	users := []sqlchannel.UserInfo{}
	err := json.Unmarshal([]byte(response.Message), &users)
	if err != nil {
		return err
	}

	tblPrinter := o.newTblPrinterWithStyle("USER INFO", []interface{}{"USERNAME", "ROLE"})
	for _, user := range users {
		tblPrinter.AddRow(user.UserName, user.RoleName)
	}
	tblPrinter.Print()
	return nil
}
