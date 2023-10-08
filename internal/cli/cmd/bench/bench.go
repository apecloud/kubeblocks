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
	"sort"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	benchListExample = templates.Examples(`
		# List all benchmarks
		kbcli bench list
	`)

	benchDeleteExample = templates.Examples(`
		# Delete  benchmark
		kbcli bench delete mybench
	`)

	benchDescribeExample = templates.Examples(`
		# Describe  benchmark
		kbcli bench describe mybench
	`)
)

var benchGVRList = []schema.GroupVersionResource{
	types.PgBenchGVR(),
	types.SysbenchGVR(),
	types.YcsbGVR(),
	types.TpccGVR(),
	types.TpchGVR(),
}

type BenchBaseOptions struct {
	// define the target database
	Driver      string
	Database    string
	Host        string
	Port        int
	User        string
	Password    string
	ClusterName string

	// define the config of pod that run benchmark
	name           string
	namespace      string
	Step           string // specify the benchmark step, exec all, cleanup, prepare or run
	TolerationsRaw []string
	Tolerations    []corev1.Toleration
	ExtraArgs      []string // extra arguments for benchmark

	factory cmdutil.Factory
	client  clientset.Interface
	dynamic dynamic.Interface
	*cluster.ClusterObjects
	genericclioptions.IOStreams
}

func (o *BenchBaseOptions) BaseComplete() error {
	tolerations, err := util.BuildTolerations(o.TolerationsRaw)
	if err != nil {
		return err
	}
	tolerationsJSON, err := json.Marshal(tolerations)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(tolerationsJSON, &o.Tolerations); err != nil {
		return err
	}

	return nil
}

// BaseValidate validates the base options
// In some cases, for example, in redis, the database is not required, the username is not required
// and password can be empty for many databases,
// so we don't validate them here
func (o *BenchBaseOptions) BaseValidate() error {
	if o.Driver == "" {
		return fmt.Errorf("driver is required")
	}

	if o.Host == "" {
		return fmt.Errorf("host is required")
	}

	if o.Port == 0 {
		return fmt.Errorf("port is required")
	}

	if err := validateBenchmarkExist(o.factory, o.IOStreams, o.name); err != nil {
		return err
	}

	return nil
}

func (o *BenchBaseOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.Driver, "driver", "", "the driver of database")
	cmd.Flags().StringVar(&o.Database, "database", "", "database name")
	cmd.Flags().StringVar(&o.Host, "host", "", "the host of database")
	cmd.Flags().StringVar(&o.User, "user", "", "the user of database")
	cmd.Flags().StringVar(&o.Password, "password", "", "the password of database")
	cmd.Flags().IntVar(&o.Port, "port", 0, "the port of database")
	cmd.Flags().StringVar(&o.ClusterName, "cluster", "", "the cluster of database")
	cmd.Flags().StringSliceVar(&o.TolerationsRaw, "tolerations", nil, `Tolerations for benchmark, such as '"dev=true:NoSchedule,large=true:NoSchedule"'`)
	cmd.Flags().StringSliceVar(&o.ExtraArgs, "extra-args", nil, "extra arguments for benchmark")

	util.RegisterClusterCompletionFunc(cmd, o.factory)
}

// NewBenchCmd creates the bench command
func NewBenchCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run a benchmark.",
	}

	// add subcommands
	cmd.AddCommand(
		NewSysBenchCmd(f, streams),
		NewPgBenchCmd(f, streams),
		NewYcsbCmd(f, streams),
		NewTpccCmd(f, streams),
		NewTpchCmd(f, streams),
		newListCmd(f, streams),
		newDeleteCmd(f, streams),
		newDescribeCmd(f, streams),
	)

	return cmd
}

type benchListOption struct {
	Factory       cmdutil.Factory
	LabelSelector string
	Format        string
	AllNamespaces bool

	genericclioptions.IOStreams
}

type benchDeleteOption struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string

	genericclioptions.IOStreams
}

type benchDescribeOption struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string
	benchs    []string

	genericclioptions.IOStreams
}

func newListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &benchListOption{
		Factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all benchmarks.",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Example: benchListExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")

	return cmd
}

func newDeleteCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &benchDeleteOption{
		factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a benchmark.",
		Aliases: []string{"del"},
		Example: benchDeleteExample,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return registerBenchmarkCompletionFunc(cmd, f, args, toComplete)
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete())
			cmdutil.CheckErr(o.run(args))
		},
	}

	return cmd
}

func newDescribeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &benchDescribeOption{
		factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "describe",
		Short:   "Describe a benchmark.",
		Aliases: []string{"desc"},
		Example: benchDescribeExample,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return registerBenchmarkCompletionFunc(cmd, f, args, toComplete)
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.run())
		},
	}

	return cmd
}

func (o *benchListOption) run() error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(o.Out, "it seems that kubebench is not running, please run `kbcli addon enable kubebench` to install it or check the kubebench pod status.\n")
		}
	}()

	var infos []*resource.Info
	for _, gvr := range benchGVRList {
		bench := list.NewListOptions(o.Factory, o.IOStreams, gvr)

		bench.Print = false
		bench.LabelSelector = o.LabelSelector
		bench.AllNamespaces = o.AllNamespaces
		result, err := bench.Run()
		if err != nil {
			if strings.Contains(err.Error(), "the server doesn't have a resource type") {
				fmt.Fprintf(o.Out, "kubebench is not installed, please run `kbcli addon enable kubebench` to install it.\n")
				return nil
			}
			return err
		}

		benchInfos, err := result.Infos()
		if err != nil {
			return err
		}
		infos = append(infos, benchInfos...)
	}

	if len(infos) == 0 {
		fmt.Fprintf(o.Out, "No benchmarks found.\n")
		return nil
	}

	printRows := func(tbl *printer.TablePrinter) error {
		// sort bench with kind, then .status.phase, finally .metadata.name
		sort.SliceStable(infos, func(i, j int) bool {
			iKind := infos[i].Object.(*unstructured.Unstructured).GetKind()
			jKind := infos[j].Object.(*unstructured.Unstructured).GetKind()
			iPhase := infos[i].Object.(*unstructured.Unstructured).Object["status"].(map[string]interface{})["phase"]
			jPhase := infos[j].Object.(*unstructured.Unstructured).Object["status"].(map[string]interface{})["phase"]
			iName := infos[i].Object.(*unstructured.Unstructured).GetName()
			jName := infos[j].Object.(*unstructured.Unstructured).GetName()

			if iKind != jKind {
				return iKind < jKind
			}
			if iPhase != jPhase {
				return iPhase.(string) < jPhase.(string)
			}
			return iName < jName
		})

		for _, info := range infos {
			obj := info.Object.(*unstructured.Unstructured)
			tbl.AddRow(
				obj.GetName(),
				obj.GetNamespace(),
				obj.GetKind(),
				obj.Object["status"].(map[string]interface{})["phase"],
				obj.Object["status"].(map[string]interface{})["completions"],
			)
		}
		return nil
	}

	if err := printer.PrintTable(o.Out, nil, printRows, "NAME", "NAMESPACE", "KIND", "STATUS", "COMPLETIONS"); err != nil {
		return err
	}
	return nil
}

func (o *benchDeleteOption) complete() error {
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

	return nil
}

func (o *benchDeleteOption) run(args []string) error {
	delete := func(benchName string) error {
		var found bool

		for _, gvr := range benchGVRList {
			if err := o.dynamic.Resource(gvr).Namespace(o.namespace).Delete(context.TODO(), benchName, metav1.DeleteOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}

			found = true
			break
		}

		if !found {
			return fmt.Errorf("benchmark %s not found", benchName)
		}

		return nil
	}

	for _, benchName := range args {
		if err := delete(benchName); err != nil {
			return err
		}
	}
	return nil
}

func (o *benchDescribeOption) complete(args []string) error {
	var err error

	o.benchs = args

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

	return nil
}

func (o *benchDescribeOption) run() error {
	describe := func(benchName string) error {
		var found bool

		for _, gvr := range benchGVRList {
			obj, err := o.dynamic.Resource(gvr).Namespace(o.namespace).Get(context.Background(), benchName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}

			found = true

			if err := printer.PrettyPrintObj(obj); err != nil {
				return err
			}

			break
		}

		if !found {
			return fmt.Errorf("benchmark %s not found", benchName)
		}

		return nil
	}

	for _, benchName := range o.benchs {
		if err := describe(benchName); err != nil {
			return err
		}
	}
	return nil
}

func registerBenchmarkCompletionFunc(cmd *cobra.Command, f cmdutil.Factory, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var benchs []string
	for _, gvr := range benchGVRList {
		comp, _ := util.ResourceNameCompletionFunc(f, gvr)(cmd, args, toComplete)
		benchs = append(benchs, comp...)
	}

	return benchs, cobra.ShellCompDirectiveNoFileComp
}

func validateBenchmarkExist(factory cmdutil.Factory, streams genericclioptions.IOStreams, name string) error {
	var infos []*resource.Info
	for _, gvr := range benchGVRList {
		bench := list.NewListOptions(factory, streams, gvr)

		bench.Print = false
		result, err := bench.Run()
		if err != nil {
			if strings.Contains(err.Error(), "the server doesn't have a resource type") {
				return fmt.Errorf("kubebench is not installed, please run `kbcli addon enable kubebench` to install it")
			}
			return err
		}

		benchInfos, err := result.Infos()
		if err != nil {
			return err
		}
		infos = append(infos, benchInfos...)
	}

	for _, info := range infos {
		if info.Name == name {
			return fmt.Errorf("benchmark %s already exists", name)
		}
	}
	return nil
}

// parseStepAndName parses the step and name from the given arguments and name prefix.
// If no arguments are provided, it sets the step to "all" and generates a random name with the given prefix.
// If the first argument is "all", "cleanup", "prepare", or "run", it sets the step to the argument value.
// If a second argument is provided, it sets the name to the argument value.
// If the first argument is not a valid step value, it sets the name to the first argument value.
// Returns the step and name as strings.
func parseStepAndName(args []string, namePrefix string) (step, name string) {
	step = "all"
	name = fmt.Sprintf("%s-%s", namePrefix, util.RandRFC1123String(6))

	if len(args) > 0 {
		switch args[0] {
		case "all", "cleanup", "prepare", "run":
			step = args[0]
			if len(args) > 1 {
				name = args[1]
			}
		default:
			name = args[0]
		}
	}
	return
}
