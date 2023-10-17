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
	tpchDriverMap = map[string]string{
		"mysql": "mysql",
	}
	tpchSupportedDrivers = []string{"mysql"}
)

var tpchExample = templates.Examples(`
	# tpch on a cluster, that will exec for all steps, cleanup, prepare and run
	kbcli bench tpch mytest --cluster mycluster --user xxx --password xxx --database mydb

	# tpch on a cluster with run, just run by running the test
	kbcli bench tpch run mytest --cluster mycluster --user xxx --password xxx --database mydb
`)

type TpchOptions struct {
	BenchBaseOptions
}

func NewTpchCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &TpchOptions{
		BenchBaseOptions: BenchBaseOptions{
			IOStreams: streams,
			factory:   f,
		},
	}
	cmd := &cobra.Command{
		Use:     "tpch [Step] [BenchmarkName]",
		Short:   "Run tpch benchmark",
		Example: tpchExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.AddFlags(cmd)

	return cmd
}

func (o *TpchOptions) Complete(args []string) error {
	var err error
	var driver string
	var host string
	var port int

	if err = o.BenchBaseOptions.BaseComplete(); err != nil {
		return err
	}

	o.Step, o.name = parseStepAndName(args, "tpch")

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
	if v, ok := tpchDriverMap[driver]; ok && o.Driver == "" {
		o.Driver = v
	}

	// don't overwrite the host and port if they are already set
	if o.Host == "" && o.Port == 0 {
		o.Host = host
		o.Port = port
	}

	return nil
}

func (o *TpchOptions) Validate() error {
	if err := o.BenchBaseOptions.BaseValidate(); err != nil {
		return err
	}

	var supported bool
	for _, v := range tpchDriverMap {
		if o.Driver == v {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("tpch now only support driver in [%s], your cluster driver is %s"+
			"if your cluster belongs to the category of MySQL, please set the driver to one of them",
			strings.Join(tpchSupportedDrivers, ", "), o.Driver)
	}

	if o.User == "" {
		return fmt.Errorf("user is required")
	}

	if o.Database == "" {
		return fmt.Errorf("database is required")
	}

	return nil
}

func (o *TpchOptions) Run() error {
	tpch := v1alpha1.Tpch{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Tpch",
			APIVersion: types.TpchGVR().GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Spec: v1alpha1.TpchSpec{
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
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&tpch)
	if err != nil {
		return err
	}
	obj.SetUnstructuredContent(data)

	obj, err = o.dynamic.Resource(types.TpchGVR()).Namespace(o.namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "%s %s created\n", obj.GetKind(), obj.GetName())
	return nil
}
