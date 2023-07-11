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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubebench/api/v1alpha1"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

const (
	pgBenchDriver = "postgresql"
)

var pgbenchExample = templates.Examples(`
# pgbench run on a cluster
kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx

# pgbench run on a cluster with different threads and different client
kbcli bench sysbench mytest --cluster pgcluster --user xxx --password xxx --database xxx --clients 5 --threads 5

# pgbench run on a cluster with specified transactions
kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --transactions 1000

# pgbench run on a cluster with specified seconds
kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --duration 60

# pgbench run on a cluster with select only
kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --select
`)

type PgBenchOptions struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	name      string
	namespace string

	ClusterName  string   // specify the name of the cluster to run benchmark test
	Scale        int      // specify the scale factor for the benchmark test
	Clients      []int    // specify the number of clients to run
	Threads      int      // specify the number of threads per client
	Transactions int      // specify the number of transactions per client
	Duration     int      // specify the duration of benchmark test in seconds
	Select       bool     // specify to run SELECT-only transactions
	ExtraArgs    []string // specify extra arguments for pgbench

	BenchBaseOptions
	*cluster.ClusterObjects
	genericclioptions.IOStreams
}

func NewPgBenchCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PgBenchOptions{
		factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "pgbench",
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
	cmd.Flags().StringVar(&o.ClusterName, "cluster", "", "The name of the cluster to run pgbench against")
	return cmd
}

func (o *PgBenchOptions) Complete(args []string) error {
	var err error
	var host string
	var port int

	// TODO if don't give pgbench name, generate a random name
	if len(args) == 0 {
		return fmt.Errorf("pgbench name should be specified")
	}
	if len(args) > 1 {
		return fmt.Errorf("only support to create one pgbench at a time")
	}
	o.name = args[0]

	if o.ClusterName == "" {
		return fmt.Errorf("cluster name should be specified")
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
	o.Driver, host, port, err = getDriverAndHostAndPort(o.Cluster, o.Services)
	if err != nil {
		return err
	}

	if o.Driver != pgBenchDriver {
		return fmt.Errorf("pgbench only support to run against PostgreSQL cluster, your cluster's driver is %s", o.Driver)
	}

	if o.Host == "" || o.Port == 0 {
		o.Host = host
		o.Port = port
	}

	return nil
}

func (o *PgBenchOptions) Validate() error {
	if err := o.BaseValidate(); err != nil {
		return err
	}

	if len(o.Clients) == 0 {
		return fmt.Errorf("clients should be specified")
	}

	return nil
}

func (o *PgBenchOptions) Run() error {
	pgbench := v1alpha1.Pgbench{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pgbench",
			APIVersion: "benchmark.kubebench.io/v1alpha1",
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
			Target: v1alpha1.PgbenchTarget{
				Host:     o.Host,
				Port:     o.Port,
				User:     o.User,
				Password: o.Password,
				Database: o.Database,
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
