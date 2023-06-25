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

package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/leaanthony/debme"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	all              = "all"
	prepareOperation = "prepare"
	runOperation     = "run"
	cleanupOperation = "cleanup"
)

var (
	driverMap = map[string]string{
		"mysql":      "mysql",
		"postgresql": "pgsql",
	}
)

type SysBenchOptions struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string

	Mode         string   `json:"mode"`
	Threads      []int    `json:"thread"`
	Tables       int      `json:"tables"`
	Size         int      `json:"size"`
	Times        int      `json:"times"`
	Type         []string `json:"type"`
	ReadPercent  int      `json:"readPercent"`
	WritePercent int      `json:"writePercent"`
	Value        string   `json:"value"`
	Flag         int      `json:"flag"`

	BenchBaseOptions
	*cluster.ClusterObjects     `json:"-"`
	genericclioptions.IOStreams `json:"-"`
}

func (o *SysBenchOptions) Complete(name string) error {
	var err error
	o.namespace, _, err = o.factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.dynamic, err = o.factory.DynamicClient(); err != nil {
		return err
	}

	if o.client, err = o.factory.KubernetesClientSet(); err != nil {
		return err
	}

	if o.Driver == "" || o.Host == "" || o.Port == 0 {
		clusterGetter := cluster.ObjectsGetter{
			Client:    o.client,
			Dynamic:   o.dynamic,
			Name:      name,
			Namespace: o.namespace,
			GetOptions: cluster.GetOptions{
				WithClusterDef:     true,
				WithService:        true,
				WithPod:            true,
				WithEvent:          true,
				WithPVC:            true,
				WithDataProtection: true,
			},
		}
		if o.ClusterObjects, err = clusterGetter.Get(); err != nil {
			return err
		}
		o.Driver, o.Host, o.Port, err = getDriverAndHostAndPort(o.Cluster, o.Services)
		if err != nil {
			return err
		}
		if driver, ok := driverMap[o.Driver]; ok {
			o.Driver = driver
		} else {
			return fmt.Errorf("unsupported driver %s", o.Driver)
		}
	}

	// if user just give readPercent or writePercent, we will calculate the other one
	if o.ReadPercent != 0 && o.WritePercent == 0 {
		o.WritePercent = 100 - o.ReadPercent
	}
	if o.ReadPercent == 0 && o.WritePercent != 0 {
		o.ReadPercent = 100 - o.WritePercent
	}

	return nil
}

func (o *SysBenchOptions) Validate() error {
	if err := o.BaseValidate(); err != nil {
		return err
	}

	if o.Mode == "" {
		return fmt.Errorf("mode is required")
	}

	if len(o.Type) == 0 {
		return fmt.Errorf("type is required")
	}

	if o.Tables <= 0 {
		return fmt.Errorf("tables must be greater than 0")
	}

	if o.Times <= 0 {
		return fmt.Errorf("times must be greater than 0")
	}

	if o.ReadPercent < 0 || o.ReadPercent > 100 {
		return fmt.Errorf("readPercent must be between 0 and 100")
	}
	if o.WritePercent < 0 || o.WritePercent > 100 {
		return fmt.Errorf("writePercent must be between 0 and 100")
	}

	return nil
}

func (o *SysBenchOptions) PreCreate(obj *unstructured.Unstructured) error {
	p := &corev1.Pod{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, p); err != nil {
		return err
	}

	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
	if err != nil {
		return err
	}
	obj.SetUnstructuredContent(data)
	return nil
}

func (o *SysBenchOptions) Run() error {
	var (
		err            error
		unstructureObj *unstructured.Unstructured
		optionsByte    []byte
	)

	o.Value = fmt.Sprintf("mode:%s", o.Mode)
	o.Value = fmt.Sprintf("%s,driver:%s", o.Value, o.Driver)
	o.Value = fmt.Sprintf("%s,host:%s", o.Value, o.Host)
	o.Value = fmt.Sprintf("%s,user:%s", o.Value, o.User)
	o.Value = fmt.Sprintf("%s,password:%s", o.Value, o.Password)
	o.Value = fmt.Sprintf("%s,port:%d", o.Value, o.Port)
	o.Value = fmt.Sprintf("%s,db:%s", o.Value, o.Database)
	o.Value = fmt.Sprintf("%s,tables:%d", o.Value, o.Tables)
	o.Value = fmt.Sprintf("%s,size:%d", o.Value, o.Size)
	o.Value = fmt.Sprintf("%s,times:%d", o.Value, o.Times)
	if len(o.Threads) > 0 {
		threads := make([]string, 0)
		for _, thread := range o.Threads {
			threads = append(threads, fmt.Sprintf("%d", thread))
		}
		o.Value = fmt.Sprintf("%s,threads:%s", o.Value, strings.Join(threads, " "))
	}
	if len(o.Type) > 0 {
		o.Value = fmt.Sprintf("%s,type:%s", o.Value, strings.Join(o.Type, " "))
	}
	if o.ReadPercent != 0 && o.WritePercent != 0 {
		o.Value = fmt.Sprintf("%s,others:--read-percent=%d --write-percent=%d", o.Value, o.ReadPercent, o.WritePercent)
	}

	if optionsByte, err = json.Marshal(o); err != nil {
		return err
	}

	cueFS, _ := debme.FS(cueTemplate, "template")
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(CueSysBenchTemplateName))
	if err != nil {
		return err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	if err := cueValue.Fill("options", optionsByte); err != nil {
		return err
	}
	if unstructureObj, err = cueValue.ConvertContentToUnstructured("content"); err != nil {
		return err
	}

	if _, err := o.dynamic.Resource(types.PodGVR()).Namespace(o.namespace).Create(context.Background(), unstructureObj, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func NewSysBenchCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &SysBenchOptions{
		factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:               "sysbench",
		Short:             "run a SysBench benchmark",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(executeSysBench(o, args[0], all))
		},
	}

	cmd.PersistentFlags().StringSliceVar(&o.Type, "type", []string{"oltp_read_write"}, "sysbench type, you can set multiple values")
	cmd.PersistentFlags().IntVar(&o.Size, "size", 25000, "the data size of per table")
	cmd.PersistentFlags().IntVar(&o.Tables, "tables", 10, "the number of tables")
	cmd.PersistentFlags().IntVar(&o.Times, "times", 60, "the number of test times")
	cmd.PersistentFlags().IntSliceVar(&o.Threads, "threads", []int{4}, "the number of threads, you can set multiple values, like 4,8")
	cmd.PersistentFlags().IntVar(&o.ReadPercent, "read-percent", 0, "the percent of read, only useful when type is oltp_read_write_pct")
	cmd.PersistentFlags().IntVar(&o.WritePercent, "write-percent", 0, "the percent of write, only useful when type is oltp_read_write_pct")
	cmd.PersistentFlags().IntVar(&o.Flag, "flag", 0, "the flag of sysbench, 0(normal), 1(long), 2(three nodes)")
	o.BenchBaseOptions.AddFlags(cmd)

	cmd.AddCommand(newPrepareCmd(f, o), newRunCmd(f, o), newCleanCmd(f, o))

	return cmd
}

func newPrepareCmd(f cmdutil.Factory, o *SysBenchOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "prepare [NAME]",
		Short:             "Prepare the data of SysBench for a cluster",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(executeSysBench(o, args[0], prepareOperation))
		},
	}
	return cmd
}

func newRunCmd(f cmdutil.Factory, o *SysBenchOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "run [NAME]",
		Short:             "Run  SysBench on cluster",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(executeSysBench(o, args[0], runOperation))
		},
	}
	return cmd
}

func newCleanCmd(f cmdutil.Factory, o *SysBenchOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "cleanup [NAME]",
		Short:             "Cleanup the data of SysBench for cluster",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(executeSysBench(o, args[0], cleanupOperation))
		},
	}
	return cmd
}

func executeSysBench(o *SysBenchOptions, name string, mode string) error {
	o.Mode = mode
	if err := o.Complete(name); err != nil {
		return err
	}
	if err := o.Validate(); err != nil {
		return err
	}
	if err := o.Run(); err != nil {
		return err
	}
	return nil
}

func getDriverAndHostAndPort(c *appsv1alpha1.Cluster, svcList *corev1.ServiceList) (driver string, host string, port int, err error) {
	var internalEndpoints []string
	var externalEndpoints []string

	if c == nil {
		return "", "", 0, fmt.Errorf("cluster is nil")
	}

	for _, comp := range c.Spec.ComponentSpecs {
		driver = comp.Name
		internalEndpoints, externalEndpoints = cluster.GetComponentEndpoints(svcList, &comp)
		if len(internalEndpoints) > 0 || len(externalEndpoints) > 0 {
			break
		}
	}
	switch {
	case len(internalEndpoints) > 0:
		host, port, err = parseHostAndPort(internalEndpoints[0])
	case len(externalEndpoints) > 0:
		host, port, err = parseHostAndPort(externalEndpoints[0])
	default:
		err = fmt.Errorf("no endpoints found")
	}

	return
}

func parseHostAndPort(s string) (string, int, error) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, err
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, err
	}
	return host, portInt, nil
}
