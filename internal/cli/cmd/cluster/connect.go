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
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/sqlchannel/engine"
)

var connectExample = templates.Examples(`
		# connect to a specified cluster, default connect to the leader or primary instance
		kbcli cluster connect mycluster

		# connect to cluster as user
		kbcli cluster connect mycluster --as-user myuser

		# connect to a specified instance
		kbcli cluster connect -i mycluster-instance-0

		# connect to a specified component
		kbcli cluster connect mycluster --component mycomponent

		# show cli connection example
		kbcli cluster connect mycluster --show-example --client=cli

		# show java connection example
		kbcli cluster connect mycluster --show-example --client=java

		# show all connection examples
		kbcli cluster connect mycluster --show-example`)

type ConnectOptions struct {
	clusterName   string
	componentName string

	clientType  string
	showExample bool
	engine      engine.Interface

	privateEndPoint bool
	svc             *corev1.Service

	component        *appsv1alpha1.ClusterComponentSpec
	componentDef     *appsv1alpha1.ClusterComponentDefinition
	targetCluster    *appsv1alpha1.Cluster
	targetClusterDef *appsv1alpha1.ClusterDefinition

	// assume user , who has access to the cluster
	userName   string
	userPasswd string

	*exec.ExecOptions
}

// NewConnectCmd return the cmd of connecting a cluster
func NewConnectCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ConnectOptions{ExecOptions: exec.NewExecOptions(f, streams)}
	cmd := &cobra.Command{
		Use:               "connect (NAME | -i INSTANCE-NAME)",
		Short:             "Connect to a cluster or instance.",
		Example:           connectExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.validate(args))
			util.CheckErr(o.complete())
			if o.showExample {
				util.CheckErr(o.runShowExample())
			} else {
				util.CheckErr(o.connect())
			}
		},
	}
	cmd.Flags().StringVarP(&o.PodName, "instance", "i", "", "The instance name to connect.")
	cmd.Flags().StringVar(&o.componentName, "component", "", "The component to connect. If not specified, the first component will be used.")
	cmd.Flags().BoolVar(&o.showExample, "show-example", false, "Show how to connect to cluster or instance from different client.")
	cmd.Flags().StringVar(&o.clientType, "client", "", "Which client connection example should be output, only valid if --show-example is true.")

	cmd.Flags().StringVar(&o.userName, "as-user", "", "Connect to cluster as user")

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

func (o *ConnectOptions) runShowExample() error {
	// get connection info
	info, err := o.getConnectionInfo()
	if err != nil {
		return err
	}
	// make sure engine is initialized
	if o.engine == nil {
		return fmt.Errorf("engine is not initialized yet")
	}

	// if cluster does not have public endpoints, tell user to use port-forward command and
	// connect cluster from local host
	if o.privateEndPoint {
		fmt.Fprintf(o.Out, "# cluster %s does not have public endpoints, you can run following command and connect cluster from local host\n"+
			"kubectl port-forward service/%s %s:%s\n\n", o.clusterName, o.svc.Name, info.Port, info.Port)
		info.Host = "127.0.0.1"
	}

	fmt.Fprint(o.Out, o.engine.ConnectExample(info, o.clientType))
	return nil
}

func (o *ConnectOptions) validate(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("only support to connect one cluster")
	}

	// cluster name and pod instance are mutual exclusive
	if len(o.PodName) > 0 {
		if len(args) > 0 {
			return fmt.Errorf("specify either cluster name or instance name, not both")
		}
		if len(o.componentName) > 0 {
			return fmt.Errorf("component name is valid only when cluster name is specified")
		}
	} else if len(args) == 0 {
		return fmt.Errorf("either cluster name or instance name should be specified")
	}

	// set custer name
	if len(args) > 0 {
		o.clusterName = args[0]
	}

	// validate user name and password
	if len(o.userName) > 0 {
		// read password from stdin
		fmt.Print("Password: ")
		if bytePassword, err := terminal.ReadPassword(int(os.Stdin.Fd())); err != nil {
			return err
		} else {
			o.userPasswd = string(bytePassword)
		}
	}
	return nil
}

func (o *ConnectOptions) complete() error {
	var err error
	if err = o.ExecOptions.Complete(); err != nil {
		return err
	}
	// opt 1. specified pod name
	// 1.1 get pod by name
	if len(o.PodName) > 0 {
		if o.Pod, err = o.Client.CoreV1().Pods(o.Namespace).Get(context.Background(), o.PodName, metav1.GetOptions{}); err != nil {
			return err
		}
		o.clusterName = cluster.GetPodClusterName(o.Pod)
		o.componentName = cluster.GetPodComponentName(o.Pod)
	}

	// cannot infer characterType from pod directly (neither from pod annotation nor pod label)
	// so we have to get cluster definition first to get characterType
	// opt 2. specified cluster name
	// 2.1 get cluster by name
	if o.targetCluster, err = cluster.GetClusterByName(o.Dynamic, o.clusterName, o.Namespace); err != nil {
		return err
	}
	// get cluster def
	if o.targetClusterDef, err = cluster.GetClusterDefByName(o.Dynamic, o.targetCluster.Spec.ClusterDefRef); err != nil {
		return err
	}

	// 2.2 fill component name, use the first component by default
	if len(o.componentName) == 0 {
		o.component = &o.targetCluster.Spec.ComponentSpecs[0]
		o.componentName = o.component.Name
	} else {
		// verify component
		if o.component = o.targetCluster.Spec.GetComponentByName(o.componentName); o.component == nil {
			return fmt.Errorf("failed to get component %s. Check the list of components use: \n\tkbcli cluster list-components %s -n %s", o.componentName, o.clusterName, o.Namespace)
		}
	}

	// 2.3 get character type
	if o.componentDef = o.targetClusterDef.GetComponentDefByName(o.component.ComponentDefRef); o.componentDef == nil {
		return fmt.Errorf("failed to get component def :%s", o.component.ComponentDefRef)
	}

	// 2.4. get pod to connect, make sure o.clusterName, o.componentName are set before this step
	if len(o.PodName) == 0 {
		if err = o.getTargetPod(); err != nil {
			return err
		}
		if o.Pod, err = o.Client.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// connect create parameters for connecting cluster and connect
func (o *ConnectOptions) connect() error {
	if o.componentDef == nil {
		return fmt.Errorf("component def is not initialized")
	}

	var err error

	if o.engine, err = engine.New(o.componentDef.CharacterType); err != nil {
		return err
	}

	var authInfo *engine.AuthInfo
	if len(o.userName) > 0 {
		authInfo = &engine.AuthInfo{}
		authInfo.UserName = o.userName
		authInfo.UserPasswd = o.userPasswd
	} else if authInfo, err = o.getAuthInfo(); err != nil {
		return err
	}

	o.ExecOptions.ContainerName = o.engine.Container()
	o.ExecOptions.Command = o.engine.ConnectCommand(authInfo)
	if klog.V(1).Enabled() {
		fmt.Fprintf(o.Out, "connect with cmd: %s", o.ExecOptions.Command)
	}
	return o.ExecOptions.Run()
}

func (o *ConnectOptions) getAuthInfo() (*engine.AuthInfo, error) {
	// select secrets by labels, prefer admin account
	labels := fmt.Sprintf("%s=%s,%s=%s,%s=%s",
		constant.AppInstanceLabelKey, o.clusterName,
		constant.KBAppComponentLabelKey, o.componentName,
		constant.ClusterAccountLabelKey, (string)(appsv1alpha1.AdminAccount),
	)

	secrets, err := o.Client.CoreV1().Secrets(o.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets for cluster %s, component %s, err %v", o.clusterName, o.componentName, err)
	}
	if len(secrets.Items) == 0 {
		return nil, nil
	}
	return &engine.AuthInfo{
		UserName:   string(secrets.Items[0].Data["username"]),
		UserPasswd: string(secrets.Items[0].Data["password"]),
	}, nil
}

func (o *ConnectOptions) getTargetPod() error {
	// guarantee cluster name and component name are set
	if len(o.clusterName) == 0 {
		return fmt.Errorf("cluster name is not set yet")
	}
	if len(o.componentName) == 0 {
		return fmt.Errorf("component name is not set yet")
	}

	// get instantces for given cluster name and component name
	infos := cluster.GetSimpleInstanceInfosForComponent(o.Dynamic, o.clusterName, o.componentName, o.Namespace)
	if len(infos) == 0 || infos[0].Name == constant.ComponentStatusDefaultPodName {
		return fmt.Errorf("failed to find the instance to connect, please check cluster status")
	}

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
	// make sure component and componentDef are set before this step
	if o.component == nil || o.componentDef == nil {
		return nil, fmt.Errorf("failed to get component or component def")
	}

	info := &engine.ConnectionInfo{}
	getter := cluster.ObjectsGetter{
		Client:    o.Client,
		Dynamic:   o.Dynamic,
		Name:      o.clusterName,
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
	internalSvcs, externalSvcs := cluster.GetComponentServices(objs.Services, o.component)
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

	if o.engine, err = engine.New(o.componentDef.CharacterType); err != nil {
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
			break
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
