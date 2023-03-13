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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/engine"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
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

	privateEndPoint bool
	svc             *corev1.Service

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

	// if cluster does not have public endpoints, tell user to use port-forward command and
	// connect cluster from local host
	if o.privateEndPoint {
		fmt.Fprintf(o.Out, "# cluster %s does not have public endpoints, you can run following command and connect cluster from local host\n"+
			"kubectl port-forward service/%s %s:%s\n\n", o.name, o.svc.Name, info.Port, info.Port)
		info.Host = "127.0.0.1"
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
		if err := o.getTargetPod(); err != nil {
			return err
		}
	}

	// get the pod object
	pod, err := o.Client.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// cluster name is not specified, get from pod label
	if o.name == "" {
		if name, ok := pod.Annotations[constant.AppInstanceLabelKey]; !ok {
			return fmt.Errorf("failed to find the cluster to which the instance belongs")
		} else {
			o.name = name
		}
	}

	info, err := o.getConnectionInfo()
	if err != nil {
		return err
	}

	o.Command = buildCommand(info)
	o.Pod = pod
	return o.ExecOptions.Run()
}

func (o *ConnectOptions) getTargetPod() error {
	infos := cluster.GetSimpleInstanceInfos(o.Dynamic, o.name, o.Namespace)
	if infos == nil {
		return fmt.Errorf("failed to find the instance to connect, please check cluster status")
	}

	// first element is the default instance to connect
	o.PodName = infos[0].Name

	// print instance info that we connect
	if len(infos) == 1 {
		fmt.Fprintf(o.Out, "Connect to instance %s\n", o.PodName)
		return nil
	}

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
	return nil
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
	if info.User, info.Password, err = getUserAndPassword(objs.ClusterDef, objs.Secrets); err != nil {
		return nil, err
	}

	// get host and port, use external endpoints first, if external endpoints are empty,
	// use internal endpoints

	// TODO: now the primary component is the first component, that may not be correct,
	// maybe show all components connection info in the future.
	primaryCompDef := objs.ClusterDef.Spec.ComponentDefs[0]
	primaryComp := cluster.FindClusterComp(objs.Cluster, primaryCompDef.Name)
	internalSvcs, externalSvcs := cluster.GetComponentServices(objs.Services, primaryComp)
	switch {
	case len(externalSvcs) > 0:
		// cluster has public endpoint
		o.svc = externalSvcs[0]
		info.Host = cluster.GetExternalAddr(o.svc)
		info.Port = fmt.Sprintf("%d", o.svc.Spec.Ports[0].Port)
	case len(internalSvcs) > 0:
		// cluster does not have public endpoint
		o.svc = internalSvcs[0]
		info.Host = o.svc.Spec.ClusterIP
		info.Port = fmt.Sprintf("%d", o.svc.Spec.Ports[0].Port)
		o.privateEndPoint = true
	default:
		// does not find any endpoints
		return nil, fmt.Errorf("failed to find any cluster endpoints")
	}

	info.Command, info.Args, err = getCompCommandArgs(&primaryCompDef)
	if err != nil {
		return nil, err
	}

	// get engine
	o.engine, err = engine.New(objs.ClusterDef.Spec.ComponentDefs[0].CharacterType)
	if err != nil {
		return nil, err
	}

	return info, nil
}

// get cluster user and password from secrets
func getUserAndPassword(clusterDef *appsv1alpha1.ClusterDefinition, secrets *corev1.SecretList) (string, string, error) {
	var (
		user, password = "", ""
		err            error
	)

	if len(secrets.Items) == 0 {
		return user, password, fmt.Errorf("failed to find the cluster username and password")
	}

	getPasswordKey := func(connectionCredential map[string]string) string {
		for k := range connectionCredential {
			if strings.Contains(k, "password") {
				return k
			}
		}
		return "password"
	}

	getSecretVal := func(secret *corev1.Secret, key string) (string, error) {
		val, ok := secret.Data[key]
		if !ok {
			return "", fmt.Errorf("failed to find the cluster %s", key)
		}
		return string(val), nil
	}

	// now, we only use the first secret
	var secret corev1.Secret
	for i, s := range secrets.Items {
		if strings.Contains(s.Name, "conn-credential") {
			secret = secrets.Items[i]
		}
	}
	user, err = getSecretVal(&secret, "username")
	if err != nil {
		return user, password, err
	}

	passwordKey := getPasswordKey(clusterDef.Spec.ConnectionCredential)
	password, err = getSecretVal(&secret, passwordKey)
	return user, password, err
}

func getCompCommandArgs(compDef *appsv1alpha1.ClusterComponentDefinition) ([]string, []string, error) {
	failErr := fmt.Errorf("failed to find the connection command")
	if compDef == nil || compDef.SystemAccounts == nil ||
		compDef.SystemAccounts.CmdExecutorConfig == nil {
		return nil, nil, failErr
	}

	execCfg := compDef.SystemAccounts.CmdExecutorConfig
	command := execCfg.Command
	if len(command) == 0 {
		return nil, nil, failErr
	}
	return command, execCfg.Args, nil
}

// buildCommand build connection command by SystemAccounts.CmdExecutorConfig.
// CLI should not be coupled to a specific engine, so read command info from
// clusterDefinition, but now these information is used to create system
// accounts, we need to do some special handling.
//
// TODO: Refactoring using command channel
func buildCommand(info *engine.ConnectionInfo) []string {
	args := make([]string, 0)
	for _, arg := range info.Args {
		// KB_ACCOUNT_STATEMENT is used to create system accounts, ignore it
		// replace KB_ACCOUNT_ENDPOINT with local host IP
		if strings.Contains(arg, "$(KB_ACCOUNT_ENDPOINT)") && strings.Contains(arg, "$(KB_ACCOUNT_STATEMENT)") {
			arg = strings.Replace(arg, "$(KB_ACCOUNT_ENDPOINT)", "127.0.0.1", 1)
			arg = strings.Replace(arg, "$(KB_ACCOUNT_STATEMENT)", "", 1)
			args = append(args, arg)
			continue
		}
		if strings.Contains(arg, "$(KB_ACCOUNT_ENDPOINT)") {
			args = append(args, strings.Replace(arg, "$(KB_ACCOUNT_ENDPOINT)", "127.0.0.1", 1))
			continue
		}
		if strings.Contains(arg, "$(KB_ACCOUNT_STATEMENT)") {
			continue
		}
		args = append(args, strings.Replace(strings.Replace(arg, "(", "", 1), ")", "", 1))
	}
	return append(info.Command, strings.Join(args, " "))
}
