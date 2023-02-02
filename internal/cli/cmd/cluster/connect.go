/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/engine"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var connectExample = templates.Examples(`
		# connect to a specified cluster, default connect to the leader or primary instance
		kbcli cluster connect mycluster

		# connect to a specified instance
		kbcli cluster connect -i mycluster-instance-0

		# show cli connection example
		kbcli cluster connect mycluster --show-example --client=cli

		# show java connection example
		kbcli cluster connect mycluster --show-example --client=java

		# show all connection examples
		kbcli cluster connect mycluster --show-example`)

type ConnectOptions struct {
	name        string
	clientType  string
	showExample bool
	engine      engine.Interface
	*exec.ExecOptions
}

// NewConnectCmd return the cmd of connecting a cluster
func NewConnectCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ConnectOptions{ExecOptions: exec.NewExecOptions(f, streams)}
	cmd := &cobra.Command{
		Use:               "connect (NAME | -i INSTANCE-NAME)",
		Short:             "Connect to a cluster or instance",
		Example:           connectExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.ExecOptions.Complete())
			if o.showExample {
				util.CheckErr(o.runShowExample(args))
			} else {
				util.CheckErr(o.connect(args))
			}
		},
	}
	cmd.Flags().StringVarP(&o.PodName, "instance", "i", "", "The instance name to connect.")
	cmd.Flags().BoolVar(&o.showExample, "show-example", false, "Show how to connect to cluster or instance from different client.")
	cmd.Flags().StringVar(&o.clientType, "client", "", "Which client connection example should be output, only valid if --show-example is true.")
	util.CheckErr(cmd.RegisterFlagCompletionFunc("client", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var types []string
		for _, t := range engine.ClientTypes() {
			if strings.HasPrefix(t, toComplete) {
				types = append(types, t)
			}
		}
		return types, cobra.ShellCompDirectiveNoFileComp
	}))
	return cmd
}

func (o *ConnectOptions) runShowExample(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("only support to connect one cluster")
	}

	if len(args) == 0 {
		return fmt.Errorf("cluster name should be specified when --show-example is true")
	}

	o.name = args[0]

	// get connection info
	info, err := o.getConnectionInfo()
	if err != nil {
		return err
	}

	fmt.Fprint(o.Out, o.engine.ConnectExample(info, o.clientType))
	return nil
}

// connect create parameters for connecting cluster and connect
func (o *ConnectOptions) connect(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("only support to connect one cluster")
	}

	if len(args) == 0 && len(o.PodName) == 0 {
		return fmt.Errorf("cluster name or instance name should be specified")
	}

	if len(args) > 0 {
		o.name = args[0]
	}

	// get target pod name, if not specified, find default pod from cluster
	if len(o.PodName) == 0 {
		infos := cluster.GetSimpleInstanceInfos(o.Dynamic, o.name, o.Namespace)
		if infos == nil {
			return fmt.Errorf("failed to find the instance to connect, please check cluster status")
		}

		// first element is the default instance to connect
		o.PodName = infos[0].Name

		// output all instance infos
		var nameRoles = make([]string, len(infos))
		for i, info := range infos {
			if len(info.Role) == 0 {
				nameRoles[i] = info.Name
			} else {
				nameRoles[i] = fmt.Sprintf("%s(%s)", info.Name, info.Role)
			}
		}
		fmt.Fprintf(o.Out, "Connect to instance %s: out of %s\n", o.PodName, strings.Join(nameRoles, ", "))
	}

	// get the pod object
	pod, err := o.Client.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// get the connect command and the target container
	engine, err := getEngineByPod(pod)
	if err != nil {
		return err
	}

	o.Command = engine.ConnectCommand()
	o.ContainerName = engine.EngineName()
	o.Pod = pod
	return o.ExecOptions.Run()
}

func getEngineByPod(pod *corev1.Pod) (engine.Interface, error) {
	typeName, err := cluster.GetClusterTypeByPod(pod)
	if err != nil {
		return nil, err
	}

	engine, err := engine.New(typeName)
	if err != nil {
		return nil, err
	}

	return engine, nil
}

func (o *ConnectOptions) getConnectionInfo() (*engine.ConnectionInfo, error) {
	info := &engine.ConnectionInfo{}
	getter := cluster.ObjectsGetter{
		Client:    o.Client,
		Dynamic:   o.Dynamic,
		Name:      o.name,
		Namespace: o.Namespace,
		GetOptions: cluster.GetOptions{
			WithClusterDef: true,
			WithService:    true,
			WithSecret:     true,
		},
	}

	objs, err := getter.Get()
	if err != nil {
		return nil, err
	}

	// get username and password
	if info.User, info.Password, err = getUserAndPassword(objs.Secrets); err != nil {
		return nil, err
	}

	// get host and port, use external endpoints first, if external endpoints are empty,
	// use internal endpoints
	var private = false
	primaryComponent := cluster.FindClusterComp(objs.Cluster, objs.ClusterDef.Spec.Components[0].TypeName)
	svcs := cluster.GetComponentServices(objs.Services, primaryComponent)
	getEndpoint := func(getIPFn func(*corev1.Service) string) *corev1.Service {
		for _, svc := range svcs {
			var ip = getIPFn(svc)
			if ip != "" && ip != "None" {
				info.Host = ip
				info.Port = fmt.Sprintf("%d", svc.Spec.Ports[0].Port)
				return svc
			}
		}
		return nil
	}

	// if there is public endpoint, use it to build connection info
	svc := getEndpoint(cluster.GetExternalIP)

	// if cluster does not have public endpoint, use local host to connect
	if svc == nil {
		svc = getEndpoint(func(svc *corev1.Service) string {
			return svc.Spec.ClusterIP
		})
		private = true
	}

	// does not find any endpoints
	if svc == nil {
		return nil, fmt.Errorf("failed to find cluster endpoints")
	}

	// if cluster does not have public endpoints, tell user to use port-forward command and
	// connect cluster from local host
	if private {
		fmt.Fprintf(o.Out, "# cluster %s does not have public endpoints, you can run following command and connect cluster from local host\n"+
			"kubectl port-forward service/%s %s:%s\n\n", objs.Cluster.Name, svc.Name, info.Port, info.Port)
		info.Host = "127.0.0.1"
	}

	// get engine
	o.engine, err = engine.New(objs.ClusterDef.Spec.Type)
	if err != nil {
		return nil, err
	}

	return info, nil
}

// get cluster user and password from secrets
func getUserAndPassword(secrets *corev1.SecretList) (string, string, error) {
	var (
		user, password = "", ""
		err            error
	)

	if len(secrets.Items) == 0 {
		return user, password, fmt.Errorf("failed to find the cluster username and password")
	}

	getSecretVal := func(secret *corev1.Secret, key string) (string, error) {
		val, ok := secret.Data[key]
		if !ok {
			return "", fmt.Errorf("failed to find the cluster %s", key)
		}
		return string(val), nil
	}

	// now, we only use the first secret
	secret := secrets.Items[0]
	user, err = getSecretVal(&secret, "username")
	if err != nil {
		return user, password, err
	}

	password, err = getSecretVal(&secret, "password")
	return user, password, err
}
