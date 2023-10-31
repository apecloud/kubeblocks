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
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubebench/api/v1alpha1"

	"github.com/apecloud/kubeblocks/pkg/cli/cluster"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
)

var (
	ycsbDriverMap = map[string]string{
		"mongodb":    "mongodb",
		"mysql":      "mysql",
		"postgresql": "postgresql",
		"redis":      "redis",
	}
	ycsbSupportedDrivers = []string{"mongodb", "mysql", "postgresql", "redis"}
)

var ycsbExample = templates.Examples(`
	# ycsb on a cluster,  that will exec for all steps, cleanup, prepare and run
	kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb
	
	# ycsb on a cluster with cleanup, only cleanup by deleting the testdata
	kbcli bench ycsb cleanup mytest --cluster mycluster --user xxx --password xxx --database mydb
	
	# ycsb on a cluster with prepare, just prepare by creating the testdata
	kbcli bench ycsb prepare mytest --cluster mycluster --user xxx --password xxx --database mydb
	
	# ycsb on a cluster with run, just run by running the test
	kbcli bench ycsb run mytest --cluster mycluster --user xxx --password xxx --database mydb
	
	# ycsb on a cluster with thread counts
	kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb --threads 4,8
	
	# ycsb on a cluster with record number and operation number
	kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb --record-count 10000 --operation-count 10000
	
	# ycsb on a cluster mixed read/write
	kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb --read-proportion 50 --update-proportion 50
`)

type YcsbOptions struct {
	Threads                   []int // the number of threads to use
	RecordCount               int   // the number of records to use
	OperationCount            int   // the number of operations to use during the run phase
	ReadProportion            int   // the proportion of operations that are reads
	UpdateProportion          int   // the proportion of operations that are updates
	InsertProportion          int   // the proportion of operations that are inserts
	ScanProportion            int   // the proportion of operations that are scans
	ReadModifyWriteProportion int   // the proportion of operations that are read then modify a record

	BenchBaseOptions
}

func NewYcsbCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &YcsbOptions{
		BenchBaseOptions: BenchBaseOptions{
			IOStreams: streams,
			factory:   f,
		},
	}

	cmd := &cobra.Command{
		Use:     "ycsb [Step] [BenchmarkName]",
		Short:   "Run YCSB benchmark on a cluster",
		Example: ycsbExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.BenchBaseOptions.AddFlags(cmd)
	cmd.Flags().IntSliceVar(&o.Threads, "threads", []int{1}, "the number of threads to use")
	cmd.Flags().IntVar(&o.RecordCount, "record-count", 1000, "the number of records to use")
	cmd.Flags().IntVar(&o.OperationCount, "operation-count", 1000, "the number of operations to use during the run phase")
	cmd.Flags().IntVar(&o.ReadProportion, "read-proportion", 0, "the percentage of read operations in benchmark")
	cmd.Flags().IntVar(&o.UpdateProportion, "update-proportion", 0, "the percentage of update operations in benchmark")
	cmd.Flags().IntVar(&o.InsertProportion, "insert-proportion", 0, "the percentage of insert operations in benchmark")
	cmd.Flags().IntVar(&o.ScanProportion, "scan-proportion", 0, "the percentage of scan operations in benchmark")
	cmd.Flags().IntVar(&o.ReadModifyWriteProportion, "read-modify-write-proportion", 0, "the percentage of read-modify-write operations in benchmark, which read a record, modify it, and write it back")

	return cmd
}

func (o *YcsbOptions) Complete(args []string) error {
	var err error
	var driver string
	var host string
	var port int

	if err = o.BenchBaseOptions.BaseComplete(); err != nil {
		return err
	}

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

	o.Step, o.name = parseStepAndName(args, "ycsb")
	if o.ClusterName != "" {
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

	// don't overwrite the driver if it's already set
	if v, ok := ycsbDriverMap[driver]; ok && o.Driver == "" {
		o.Driver = v
	}

	// don't overwrite the host and port if they are already set
	if o.Host == "" && o.Port == 0 {
		o.Host = host
		o.Port = port
	}

	return nil
}

func (o *YcsbOptions) Validate() error {
	if err := o.BaseValidate(); err != nil {
		return err
	}

	var supported bool
	for _, v := range ycsbDriverMap {
		if v == o.Driver {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("ycsb now only supports drivers in [%s], current cluster driver is %s", strings.Join(ycsbSupportedDrivers, ","), o.Driver)
	}

	if o.RecordCount < 0 {
		return fmt.Errorf("record count should be positive")
	}
	if o.OperationCount < 0 {
		return fmt.Errorf("operation count should be positive")
	}

	// constraint the proportion in [0, 100]
	if o.ReadProportion < 0 || o.ReadProportion > 100 {
		return fmt.Errorf("read proportion should be in [0, 100]")
	}
	if o.UpdateProportion < 0 || o.UpdateProportion > 100 {
		return fmt.Errorf("update proportion should be in [0, 100]")
	}
	if o.InsertProportion < 0 || o.InsertProportion > 100 {
		return fmt.Errorf("insert proportion should be in [0, 100]")
	}
	if o.ScanProportion < 0 || o.ScanProportion > 100 {
		return fmt.Errorf("scan proportion should be in [0, 100]")
	}
	if o.ReadModifyWriteProportion < 0 || o.ReadModifyWriteProportion > 100 {
		return fmt.Errorf("read-modify-write proportion should be in [0, 100]")
	}

	return nil
}

func (o *YcsbOptions) Run() error {
	ycsb := v1alpha1.Ycsb{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ycsb",
			APIVersion: types.YcsbGVR().GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Spec: v1alpha1.YcsbSpec{
			RecordCount:               o.RecordCount,
			OperationCount:            o.OperationCount,
			Threads:                   o.Threads,
			ReadProportion:            o.ReadProportion,
			UpdateProportion:          o.UpdateProportion,
			InsertProportion:          o.InsertProportion,
			ScanProportion:            o.ScanProportion,
			ReadModifyWriteProportion: o.ReadModifyWriteProportion,
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
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ycsb)
	if err != nil {
		return err
	}
	obj.SetUnstructuredContent(data)

	obj, err = o.dynamic.Resource(types.YcsbGVR()).Namespace(o.namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "%s %s created\n", obj.GetKind(), obj.GetName())
	return nil
}
