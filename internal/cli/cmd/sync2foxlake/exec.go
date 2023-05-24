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

package sync2foxlake

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdexec "k8s.io/kubectl/pkg/cmd/exec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	ConfigMapName = "sync2foxlake-config"
)

type Sync2FoxLakeExecOptions struct {
	Cm      *corev1.ConfigMap
	Stdout  io.Writer
	Outputs []string
	*exec.ExecOptions
}

func newSync2FoxLakeExecOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *Sync2FoxLakeExecOptions {
	return &Sync2FoxLakeExecOptions{
		ExecOptions: &exec.ExecOptions{
			Factory: f,
			StreamOptions: cmdexec.StreamOptions{
				IOStreams: streams,
				Stdin:     false,
				TTY:       false,
			},
			Executor: &cmdexec.DefaultRemoteExecutor{},
		}}
}

func (o *Sync2FoxLakeExecOptions) complete() error {
	var err error
	if err = o.ExecOptions.Complete(); err != nil {
		return err
	}
	o.Stdout = o.Out

	labels := fmt.Sprintf("%s in (%s)", constant.AppNameLabelKey, "sync2foxlake")
	if cmList, err := o.Client.CoreV1().ConfigMaps(o.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels,
	}); err != nil {
		return err
	} else if len(cmList.Items) > 0 {
		o.Cm = &cmList.Items[0]
	} else {
		o.Cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: o.Namespace,
				Labels: map[string]string{
					constant.AppNameLabelKey: "sync2foxlake",
				},
			},
			Data: map[string]string{},
		}
		if _, err := o.Client.CoreV1().ConfigMaps(o.Namespace).Create(context.TODO(), o.Cm, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *Sync2FoxLakeExecOptions) run(cmKey string, buildSQL func(string) string) error {
	info, hasExists := o.Cm.Data[cmKey]
	if !hasExists {
		return fmt.Errorf("Sync2foxlake task %s not found", cmKey)
	}
	s := strings.Split(info, ":")
	if len(s) != 7 {
		return fmt.Errorf("invalid info format: %s", info)
	}
	if o.PodName == "" {
		o.PodName = s[2]
	}
	o.Command = []string{"mysql", "-h", s[3], "-P", s[4], "-u", s[5], "-p" + s[6], "-e", buildSQL(s[1])}

	output := &bytes.Buffer{}
	o.Out = output
	errout := &bytes.Buffer{}
	o.ErrOut = errout

	var err error
	if err = o.ExecOptions.Run(); err != nil {
		if errout.String() != "" {
			fmt.Println(errout.String())
		}
		return err
	}
	o.Outputs = append(o.Outputs, output.String())
	return err
}
