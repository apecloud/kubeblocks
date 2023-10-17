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
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubebench/api/v1alpha1"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var (
	sysbenchDriverMap = map[string]string{
		"mysql":      "mysql",
		"postgresql": "pgsql",
	}
)

var sysbenchExample = templates.Examples(`
		# sysbench on a cluster, that will exec for all steps, cleanup, prepare and run
		kbcli bench sysbench mytest --cluster mycluster --user xxx --password xxx --database mydb

		# sysbench run on a cluster with cleanup, only cleanup by deleting the testdata
		kbcli bench sysbench cleanup mytest --cluster mycluster --user xxx --password xxx --database mydb

		# sysbench run on a cluster with prepare, just prepare by creating the testdata
		kbcli bench sysbench prepare mytest --cluster mycluster --user xxx --password xxx --database mydb

		# sysbench run on a cluster with run, just run by running the test
		kbcli bench sysbench run mytest --cluster mycluster --user xxx --password xxx --database mydb

		# sysbench on a cluster with thread counts
		kbcli bench sysbench mytest --cluster mycluster --user xxx --password xxx --database mydb --threads 4,8

		# sysbench on a cluster with type
		kbcli bench sysbench mytest --cluster mycluster --user xxx --password xxx --database mydb --type oltp_read_only,oltp_read_write

		# sysbench on a cluster with specified read/write ratio
		kbcli bench sysbench mytest --cluster mycluster --user xxx --password xxx  --database mydb --type oltp_read_write_pct --read-percent 80 --write-percent 20

		# sysbench on a cluster with specified tables and size
		kbcli bench sysbench mytest --cluster mycluster --user xxx --password xxx --database mydb --tables 10 --size 25000
`)

type SysBenchOptions struct {
	Threads      []int // the number of threads
	Tables       int   // the number of tables
	Size         int   // the data size of per table
	Duration     int
	Type         []string
	ReadPercent  int
	WritePercent int

	BenchBaseOptions
}

func NewSysBenchCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &SysBenchOptions{
		BenchBaseOptions: BenchBaseOptions{
			IOStreams: streams,
			factory:   f,
		},
	}

	cmd := &cobra.Command{
		Use:     "sysbench [Step] [BenchmarkName]",
		Short:   "run a SysBench benchmark",
		Example: sysbenchExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringSliceVar(&o.Type, "type", []string{"oltp_read_write"}, "sysbench type, you can set multiple values")
	cmd.Flags().IntVar(&o.Size, "size", 25000, "the data size of per table")
	cmd.Flags().IntVar(&o.Tables, "tables", 10, "the number of tables")
	cmd.Flags().IntVar(&o.Duration, "duration", 60, "the seconds of running sysbench")
	cmd.Flags().IntSliceVar(&o.Threads, "threads", []int{4}, "the number of threads, you can set multiple values, like 4,8")
	cmd.Flags().IntVar(&o.ReadPercent, "read-percent", 0, "the percent of read, only useful when type is oltp_read_write_pct")
	cmd.Flags().IntVar(&o.WritePercent, "write-percent", 0, "the percent of write, only useful when type is oltp_read_write_pct")
	o.BenchBaseOptions.AddFlags(cmd)

	return cmd
}

func (o *SysBenchOptions) Complete(args []string) error {
	var err error
	var driver string
	var host string
	var port int

	if err = o.BenchBaseOptions.BaseComplete(); err != nil {
		return err
	}

	o.Step, o.name = parseStepAndName(args, "sysbench")

	if o.ClusterName != "" {
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

		clusterGetter := cluster.ObjectsGetter{
			Client:    o.client,
			Dynamic:   o.dynamic,
			Name:      o.ClusterName,
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
		driver, host, port, err = getDriverAndHostAndPort(o.Cluster, o.Services)
		if err != nil {
			return err
		}
	}

	if v, ok := sysbenchDriverMap[driver]; ok && o.Driver == "" {
		o.Driver = v
	}

	if o.Host == "" && o.Port == 0 {
		o.Host = host
		o.Port = port
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

	var supported bool
	for _, v := range sysbenchDriverMap {
		if v == o.Driver {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("driver %s is not supported", o.Driver)
	}

	if o.User == "" {
		return fmt.Errorf("user is required")
	}

	if o.Database == "" {
		return fmt.Errorf("database is required")
	}

	if len(o.Type) == 0 {
		return fmt.Errorf("type is required")
	}

	if o.Tables <= 0 {
		return fmt.Errorf("tables must be greater than 0")
	}

	if o.Duration <= 0 {
		return fmt.Errorf("duration must be greater than 0")
	}

	if o.ReadPercent < 0 || o.ReadPercent > 100 {
		return fmt.Errorf("readPercent must be between 0 and 100")
	}
	if o.WritePercent < 0 || o.WritePercent > 100 {
		return fmt.Errorf("writePercent must be between 0 and 100")
	}

	return nil
}

func (o *SysBenchOptions) Run() error {
	if o.ReadPercent > 0 {
		o.ExtraArgs = append(o.ExtraArgs, fmt.Sprintf("--read-percent=%d", o.ReadPercent))
	}
	if o.WritePercent > 0 {
		o.ExtraArgs = append(o.ExtraArgs, fmt.Sprintf("--write-percent=%d", o.WritePercent))
	}

	sysbench := v1alpha1.Sysbench{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Sysbench",
			APIVersion: types.SysbenchGVR().GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Spec: v1alpha1.SysbenchSpec{
			Tables:   o.Tables,
			Size:     o.Size,
			Threads:  o.Threads,
			Types:    o.Type,
			Duration: o.Duration,
			BenchCommon: v1alpha1.BenchCommon{
				ExtraArgs:   o.ExtraArgs,
				Step:        o.Step,
				Tolerations: o.Tolerations,
				Target: v1alpha1.Target{
					Driver:   o.Driver,
					Host:     o.Host,
					Port:     o.Port,
					User:     o.User,
					Password: o.Password,
					Database: o.Database,
				},
			},
		},
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&sysbench)
	if err != nil {
		return err
	}
	obj.SetUnstructuredContent(data)

	obj, err = o.dynamic.Resource(types.SysbenchGVR()).Namespace(o.namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "%s %s created\n", obj.GetKind(), obj.GetName())
	return nil
}
