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
	"github.com/apecloud/kubeblocks/pkg/cli/cluster"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
)

var (
	tpccDriverMap = map[string]string{
		"mysql":      "mysql",
		"postgresql": "postgres",
	}
)

var tpccExample = templates.Examples(`
	# tpcc on a cluster, that will exec for all steps, cleanup, prepare and run
	kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb
	
	# tpcc on a cluster with cleanup, only cleanup by deleting the testdata
	kbcli bench tpcc cleanup mytest --cluster mycluster --user xxx --password xxx --database mydb
	
	# tpcc on a cluster with prepare, just prepare by creating the testdata
	kbcli bench tpcc prepare mytest --cluster mycluster --user xxx --password xxx --database mydb
	
	# tpcc on a cluster with run, just run by running the test
	kbcli bench tpcc run mytest --cluster mycluster --user xxx --password xxx --database mydb
	
	# tpcc on a cluster with warehouse counts, which is the overall database size scaling parameter
	kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --warehouses 100
	
	# tpcc on a cluster with thread counts
	kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --threads 4,8
	
	# tpcc on a cluster with transactions counts
	kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --transactions 1000
	
	# tpcc on a cluster with duration 10 minutes
	kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --duration 10
`)

type TpccOptions struct {
	WareHouses    int   // specify the overall database size scaling parameter
	Threads       []int // specify the number of threads to use
	Transactions  int   // specify the number of transactions that each thread should run
	Duration      int   // specify the number of minutes to run
	LimitTxPerMin int   // limit the number of transactions to run per minute, 0 means no limit
	NewOrder      int   // specify the percentage of transactions that should be new orders
	Payment       int   // specify the percentage of transactions that should be payments
	OrderStatus   int   // specify the percentage of transactions that should be order status
	Delivery      int   // specify the percentage of transactions that should be delivery
	StockLevel    int   // specify the percentage of transactions that should be stock level

	BenchBaseOptions
}

func NewTpccCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &TpccOptions{
		BenchBaseOptions: BenchBaseOptions{
			factory:   f,
			IOStreams: streams,
		},
	}
	cmd := &cobra.Command{
		Use:     "tpcc [Step] [BenchmarkName]",
		Short:   "Run tpcc benchmark",
		Example: tpccExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.AddFlags(cmd)
	cmd.Flags().IntVar(&o.WareHouses, "warehouses", 1, "specify the overall database size scaling parameter")
	cmd.Flags().IntSliceVar(&o.Threads, "threads", []int{1}, "specify the number of threads to use")
	cmd.Flags().IntVar(&o.Transactions, "transactions", 0, "specify the number of transactions that each thread should run")
	cmd.Flags().IntVar(&o.Duration, "duration", 1, "specify the number of minutes to run")
	cmd.Flags().IntVar(&o.LimitTxPerMin, "limit-tx-per-min", 0, "limit the number of transactions to run per minute, 0 means no limit")
	cmd.Flags().IntVar(&o.NewOrder, "new-order", 45, "specify the percentage of transactions that should be new orders")
	cmd.Flags().IntVar(&o.Payment, "payment", 43, "specify the percentage of transactions that should be payments")
	cmd.Flags().IntVar(&o.OrderStatus, "order-status", 4, "specify the percentage of transactions that should be order status")
	cmd.Flags().IntVar(&o.Delivery, "delivery", 4, "specify the percentage of transactions that should be delivery")
	cmd.Flags().IntVar(&o.StockLevel, "stock-level", 4, "specify the percentage of transactions that should be stock level")

	return cmd
}

func (o *TpccOptions) Complete(args []string) error {
	var err error
	var driver string
	var host string
	var port int

	if err = o.BenchBaseOptions.BaseComplete(); err != nil {
		return err
	}

	o.Step, o.name = parseStepAndName(args, "tpcc")

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
	if v, ok := tpccDriverMap[driver]; ok && o.Driver == "" {
		o.Driver = v
	}

	// don't overwrite the host and port if they are already set
	if o.Host == "" && o.Port == 0 {
		o.Host = host
		o.Port = port
	}

	return nil
}

func (o *TpccOptions) Validate() error {
	if err := o.BaseValidate(); err != nil {
		return err
	}

	var supported bool
	for _, v := range tpccDriverMap {
		if o.Driver == v {
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

	if o.WareHouses < 1 {
		return fmt.Errorf("warehouses must be greater than 0")
	}

	if o.Duration <= 0 {
		return fmt.Errorf("duration must be greater than 0")
	}

	if o.NewOrder+o.Payment+o.OrderStatus+o.Delivery+o.StockLevel != 100 {
		return fmt.Errorf("the sum of new-order, payment, order-status, delivery and stock-level must be 100")
	}

	return nil
}

func (o *TpccOptions) Run() error {
	tpcc := v1alpha1.Tpcc{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Tpcc",
			APIVersion: types.TpccGVR().GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Spec: v1alpha1.TpccSpec{
			WareHouses:    o.WareHouses,
			Threads:       o.Threads,
			Transactions:  o.Transactions,
			Duration:      o.Duration,
			LimitTxPerMin: o.LimitTxPerMin,
			NewOrder:      o.NewOrder,
			Payment:       o.Payment,
			OrderStatus:   o.OrderStatus,
			Delivery:      o.Delivery,
			StockLevel:    o.StockLevel,
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
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&tpcc)
	if err != nil {
		return err
	}
	obj.SetUnstructuredContent(data)

	obj, err = o.dynamic.Resource(types.TpccGVR()).Namespace(o.namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "%s %s created\n", obj.GetKind(), obj.GetName())
	return nil
}
