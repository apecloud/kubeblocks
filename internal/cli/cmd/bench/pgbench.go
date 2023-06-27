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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	pgBenchDriver = "postgresql"
	pgBenchImage  = "postgres:latest"
)

var (
	pgbenchPrepareExample = templates.Examples(`
		# pgbench prepare data on a cluster
		kbcli bench pgbench prepare pgcluster --database postgres --user xxx --password xxx --scale 100
`)

	pgbenchRunExample = templates.Examples(`
		# pgbench run on a cluster
		kbcli bench pgbench run pgcluster --database postgres --user xxx --password xxx

		# pgbench run on a cluster with different threads and different client
		kbcli bench sysbench run  pgcluster --user xxx --password xxx --database xxx --clients 5 --threads 5

		# pgbench run on a cluster with specified transactions
		kbcli bench pgbench run pgcluster --database postgres --user xxx --password xxx --transactions 1000

		# pgbench run on a cluster with specified times
		kbcli bench pgbench run pgcluster --database postgres --user xxx --password xxx --times 1000

		# pgbench run on a cluster with select only
		kbcli bench pgbench run pgcluster --database postgres --user xxx --password xxx --select
`)

	pgbenchCleanupExample = templates.Examples(`
		# pgbench cleanup data on a cluster
		kbcli bench pgbench cleanup pgcluster --database postgres --user xxx --password xxx
`)
)

type PgBenchOptions struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string

	Mode         string   `json:"mode"`
	Cmd          []string `json:"cmd"`
	Args         []string `json:"args"`
	Scale        int      // specify the scale factor for the benchmark test
	Clients      int      // specify the number of clients to run
	Threads      int      // specify the number of threads per client
	Transactions int      // specify the number of transactions per client
	Times        int      // specify the duration of benchmark test in seconds
	Select       bool     // specify to run SELECT-only transactions

	BenchBaseOptions
	*cluster.ClusterObjects
	genericclioptions.IOStreams
}

func (o *PgBenchOptions) Complete(args []string) error {
	var err error
	var host string
	var port int

	if len(args) == 0 {
		return fmt.Errorf("cluster name should be specified")
	}
	if len(args) > 1 {
		return fmt.Errorf("only support to pgbench one cluster at a time")
	}
	clusterName := args[0]

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
		Name:      clusterName,
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

	if o.Mode == "" {
		return fmt.Errorf("mode is required")
	}

	return nil
}

func (o *PgBenchOptions) Run() error {
	switch o.Mode {
	case prepareOperation:
		o.Cmd = []string{"pgbench", "-i", "-s", fmt.Sprintf("%d", o.Scale)}
	case runOperation:
		o.Cmd = []string{"pgbench",
			"-c", fmt.Sprintf("%d", o.Clients),
			"-j", fmt.Sprintf("%d", o.Threads),
			"-s", fmt.Sprintf("%d", o.Scale)}

		if o.Select {
			o.Cmd = append(o.Cmd, "-S")
		}
		// check if only one of transactions and times is specified
		if o.Transactions > 0 && o.Times > 0 {
			return fmt.Errorf("only one of transactions and times can be specified")
		}
		if o.Times > 0 {
			o.Cmd = append(o.Cmd, "-T", fmt.Sprintf("%d", o.Times))
		}
		if o.Transactions > 0 {
			o.Cmd = append(o.Cmd, "-t", fmt.Sprintf("%d", o.Transactions))
		}
	case cleanupOperation:
		o.Cmd = []string{"pgbench", "-i", "-I", "d"}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    "default",
			GenerateName: fmt.Sprintf("test-pgbench-%s-", o.Mode),
			Labels: map[string]string{
				"pgbench": fmt.Sprintf("test-pgbench-%s", o.Database),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-pgbench",
					Image: pgBenchImage,
					Env: []corev1.EnvVar{
						{
							Name:  "PGPASSWORD",
							Value: o.Password,
						},
						{
							Name:  "PGHOST",
							Value: o.Host,
						},
						{
							Name:  "PGPORT",
							Value: fmt.Sprintf("%d", o.Port),
						},
						{
							Name:  "PGUSER",
							Value: o.User,
						},
						{
							Name:  "PGDATABASE",
							Value: o.Database,
						},
					},
					Command: o.Cmd,
					Args:    o.Args,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	pod, err := o.client.CoreV1().Pods(o.namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		fmt.Fprintf(o.ErrOut, "failed to create pod: %v\n", err)
		return err
	}
	fmt.Fprintf(o.Out, "pod/%s created\n", pod.Name)

	return nil
}

func NewPgBenchCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PgBenchOptions{
		factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "pgbench",
		Short: "Run pgbench against a PostgreSQL cluster",
	}

	o.BenchBaseOptions.AddFlags(cmd)

	cmd.AddCommand(newPgBenchPrepareCmd(f, o), newPgBenchRunCmd(f, o), newPgBenchCleanupCmd(f, o))

	return cmd
}

func newPgBenchPrepareCmd(f cmdutil.Factory, o *PgBenchOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "prepare [ClusterName]",
		Short:             "Prepare pgbench test data for a PostgreSQL cluster",
		Example:           pgbenchPrepareExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(executePgBench(o, args, prepareOperation))
		},
	}

	cmd.Flags().IntVar(&o.Scale, "scale", 1, "The scale factor to use for pgbench")

	return cmd
}

func newPgBenchRunCmd(f cmdutil.Factory, o *PgBenchOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "run",
		Short:             "Run pgbench against a PostgreSQL cluster",
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Example:           pgbenchRunExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(executePgBench(o, args, runOperation))
		},
	}

	cmd.Flags().IntVar(&o.Clients, "clients", 1, "The number of clients to use for pgbench")
	cmd.Flags().IntVar(&o.Threads, "threads", 1, "The number of threads to use for pgbench")
	cmd.Flags().IntVar(&o.Transactions, "transactions", 0, "The number of transactions to run for pgbench")
	cmd.Flags().IntVar(&o.Times, "time", 0, "The duration to run pgbench for")
	cmd.Flags().BoolVar(&o.Select, "select", false, "Run pgbench with select only")
	return cmd
}

func newPgBenchCleanupCmd(f cmdutil.Factory, o *PgBenchOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "cleanup",
		Short:             "Cleanup pgbench test data for a PostgreSQL cluster",
		Example:           pgbenchCleanupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(executePgBench(o, args, cleanupOperation))
		},
	}

	return cmd
}

func executePgBench(o *PgBenchOptions, args []string, mode string) error {
	o.Mode = mode
	if err := o.Complete(args); err != nil {
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
