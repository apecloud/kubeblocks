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

const (
	pgBenchDriver = "postgresql"
)

var pgbenchExample = templates.Examples(`
	# pgbench run on a cluster, that will exec for all steps, cleanup, prepare and run
	kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx
	
	# pgbench run on a cluster with cleanup, only cleanup by deleting the testdata
	kbcli bench pgbench cleanup mytest --cluster pgcluster --database postgres --user xxx --password xxx
	
	# pgbench run on a cluster with prepare, just prepare by creating the testdata
	kbcli bench pgbench prepare mytest --cluster pgcluster --database postgres --user xxx --password xxx
	
	# pgbench run on a cluster with run, just run by running the test
	kbcli bench pgbench run mytest --cluster pgcluster --database postgres --user xxx --password xxx
	
	# pgbench run on a cluster with  thread and  client counts
	kbcli bench sysbench mytest --cluster pgcluster --user xxx --password xxx --database xxx --clients 5 --threads 5
	
	# pgbench run on a cluster with specified transactions
	kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --transactions 1000
	
	# pgbench run on a cluster with specified seconds
	kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --duration 60
	
	# pgbench run on a cluster with 'select' only
	kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --select
`)

type PgBenchOptions struct {
	Scale        int   // specify the scale factor for the benchmark test
	Clients      []int // specify the number of clients to run
	Threads      int   // specify the number of threads per client
	Transactions int   // specify the number of transactions per client
	Duration     int   // specify the duration of benchmark test in seconds
	Select       bool  // specify to run SELECT-only transactions

	BenchBaseOptions
}

func NewPgBenchCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &PgBenchOptions{
		BenchBaseOptions: BenchBaseOptions{
			IOStreams: streams,
			factory:   f,
		},
	}

	cmd := &cobra.Command{
		Use:     "pgbench [Step] [BenchmarkName]",
		Short:   "Run pgbench against a PostgreSQL cluster",
		Example: pgbenchExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.BenchBaseOptions.AddFlags(cmd)
	cmd.Flags().IntVar(&o.Scale, "scale", 1, "The scale factor to use for pgbench")
	cmd.Flags().IntSliceVar(&o.Clients, "clients", []int{1}, "The number of clients to use for pgbench")
	cmd.Flags().IntVar(&o.Threads, "threads", 1, "The number of threads to use for pgbench")
	cmd.Flags().IntVar(&o.Transactions, "transactions", 0, "The number of transactions to run for pgbench")
	cmd.Flags().IntVar(&o.Duration, "duration", 60, "The seconds to run pgbench for")
	cmd.Flags().BoolVar(&o.Select, "select", false, "Run pgbench with select only")

	return cmd
}

func (o *PgBenchOptions) Complete(args []string) error {
	var err error
	var driver string
	var host string
	var port int

	if err = o.BenchBaseOptions.BaseComplete(); err != nil {
		return err
	}

	o.Step, o.name = parseStepAndName(args, "pgbench")

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

	if o.Driver == "" {
		o.Driver = driver
	}

	if o.Host == "" && o.Port == 0 {
		o.Host = host
		o.Port = port
	}

	return nil
}

func (o *PgBenchOptions) Validate() error {
	if err := o.BaseValidate(); err != nil {
		return err
	}

	if o.Driver != pgBenchDriver {
		return fmt.Errorf("pgbench only supports to run against PostgreSQL cluster, your cluster's driver is %s", o.Driver)
	}

	if len(o.Clients) == 0 {
		return fmt.Errorf("clients should be specified")
	}

	if o.User == "" {
		return fmt.Errorf("user is required")
	}

	if o.Database == "" {
		return fmt.Errorf("database is required")
	}

	return nil
}

func (o *PgBenchOptions) Run() error {
	pgbench := v1alpha1.Pgbench{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pgbench",
			APIVersion: types.PgBenchGVR().GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.namespace,
			Name:      o.name,
		},
		Spec: v1alpha1.PgbenchSpec{
			Scale:        o.Scale,
			Clients:      o.Clients,
			Threads:      o.Threads,
			SelectOnly:   o.Select,
			Transactions: o.Transactions,
			Duration:     o.Duration,
			BenchCommon: v1alpha1.BenchCommon{
				Tolerations: o.Tolerations,
				ExtraArgs:   o.ExtraArgs,
				Step:        o.Step,
				Target: v1alpha1.Target{
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
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pgbench)
	if err != nil {
		return err
	}
	obj.SetUnstructuredContent(data)

	obj, err = o.dynamic.Resource(types.PgBenchGVR()).Namespace(o.namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "%s %s created\n", obj.GetKind(), obj.GetName())

	return nil
}
