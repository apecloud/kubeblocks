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
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type ExposeOptions struct {
	Namespace string
	Name      string
	Type      string
	Enable    string

	exposeType ExposeType
	enabled    bool
	client     kubernetes.Interface
	genericclioptions.IOStreams
}

var exposeExamples = templates.Examples(`
	# Expose a cluster to vpc
	kbcli cluster expose mycluster --type vpc --enable=true

	# Expose a cluster to internet
	kbcli cluster expose mycluster --type internet --enable=true
	
	# Stop exposing a cluster
	kbcli cluster expose mycluster --type vpc --enable=false
`)

type ExposeType string

const (
	ExposeToVPC      ExposeType = "vpc"
	ExposeToInternet ExposeType = "internet"

	EnableValue  string = "true"
	DisableValue string = "false"
)

const (
	ServiceAnnotationExposeType string = "service.kubernetes.io/kb-expose-type"
)

var ProviderExposeAnnotations = map[util.K8sProvider]map[ExposeType]map[string]string{
	util.EKSProvider: {
		ExposeToVPC: map[string]string{
			ServiceAnnotationExposeType:                             string(ExposeToVPC),
			"service.beta.kubernetes.io/aws-load-balancer-type":     "nlb",
			"service.beta.kubernetes.io/aws-load-balancer-internal": "true",
		},
		ExposeToInternet: map[string]string{
			ServiceAnnotationExposeType:                             string(ExposeToInternet),
			"service.beta.kubernetes.io/aws-load-balancer-type":     "nlb",
			"service.beta.kubernetes.io/aws-load-balancer-internal": "false",
		},
	},
}

func NewExposeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ExposeOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:               "expose NAME",
		Short:             "Expose a cluster",
		Example:           exposeExamples,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Validate(args))
			util.CheckErr(o.Complete(f, args))
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.Type, "type", "", "Expose type, currently supported types are 'vpc', 'internet'")
	_ = cmd.MarkFlagRequired("type")
	util.CheckErr(cmd.RegisterFlagCompletionFunc("type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{string(ExposeToVPC), string(ExposeToInternet)}, cobra.ShellCompDirectiveNoFileComp
	}))
	cmd.Flags().StringVar(&o.Enable, "enable", "", "Enable or disable the expose, values can be true or false")
	util.CheckErr(cmd.RegisterFlagCompletionFunc("enable", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
	}))
	_ = cmd.MarkFlagRequired("enable")

	return cmd
}

func (o *ExposeOptions) Validate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing cluster name")
	}

	switch ExposeType(o.Type) {
	case ExposeToVPC, ExposeToInternet:
	default:
		return fmt.Errorf("invalid expose type %q", o.Type)
	}
	o.exposeType = ExposeType(o.Type)

	switch strings.ToLower(o.Enable) {
	case EnableValue, DisableValue:
	default:
		return fmt.Errorf("invalid value for enable flag: %s", o.Enable)
	}
	o.enabled = o.Enable == EnableValue

	return nil
}

func (o *ExposeOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		o.Name = args[0]
	}

	o.client, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}
	return nil
}

func (o *ExposeOptions) Run() error {
	provider, err := GetK8SProvider(o.client)
	if err != nil {
		return err
	}
	return o.run(provider)
}

func (o *ExposeOptions) run(provider util.K8sProvider) error {
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "app.kubernetes.io/instance", o.Name),
	}
	svcList, err := o.client.CoreV1().Services(o.Namespace).List(context.TODO(), opts)
	if err != nil {
		return err
	}

	var disabledType ExposeType
	for _, svc := range svcList.Items {
		// ignore headless service
		if svc.Spec.ClusterIP == corev1.ClusterIPNone {
			continue
		}

		if o.enabled {
			err = o.EnableExpose(svc, provider)
		} else {
			disabledType, err = o.DisableExpose(svc, provider)
		}
		if err != nil {
			return err
		}
	}

	if o.enabled {
		fmt.Fprintf(o.Out, "Cluster %s is exposed to %s\n", o.Name, o.exposeType)
	} else {
		fmt.Fprintf(o.Out, "Cluster %s stopped exposing to %s\n", o.Name, disabledType)
	}
	return nil
}

func (o *ExposeOptions) EnableExpose(svc corev1.Service, provider util.K8sProvider) error {
	// check if the service is already exposed
	exposeType := svc.GetLabels()[ServiceAnnotationExposeType]
	if ExposeType(exposeType) == o.exposeType {
		_, _ = fmt.Fprintf(o.Out, "service %s is already exposed to %s\n", svc.Name, exposeType)
		return nil
	}

	annotations := svc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// remove current expose annotations
	var (
		kvs map[string]string
		err error
	)
	kvs, err = GetExposeAnnotations(provider, o.exposeType)
	if err != nil {
		return err
	}
	for k := range kvs {
		delete(annotations, k)
	}

	// add new expose annotations
	kvs, err = GetExposeAnnotations(provider, o.exposeType)
	if err != nil {
		return err
	}
	for k, v := range kvs {
		annotations[k] = v
	}

	svc.SetAnnotations(annotations)
	svc.Spec.Type = corev1.ServiceTypeLoadBalancer
	_, err = o.client.CoreV1().Services(o.Namespace).Update(context.TODO(), &svc, metav1.UpdateOptions{})
	return err
}

func (o *ExposeOptions) DisableExpose(svc corev1.Service, provider util.K8sProvider) (ExposeType, error) {
	// check if the service is exposed
	exposeType, ok := svc.GetAnnotations()[ServiceAnnotationExposeType]
	if !ok {
		return "", fmt.Errorf("service %s is not exposed", svc.Name)
	}
	annotations := svc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	kvs, err := GetExposeAnnotations(provider, ExposeType(exposeType))
	if err != nil {
		return "", err
	}
	for k := range kvs {
		delete(annotations, k)
	}

	svc.SetAnnotations(annotations)
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	if _, err = o.client.CoreV1().Services(o.Namespace).Update(context.TODO(), &svc, metav1.UpdateOptions{}); err != nil {
		return "", err
	}
	return ExposeType(exposeType), nil
}

func GetExposeAnnotations(provider util.K8sProvider, exposeType ExposeType) (map[string]string, error) {
	exposeAnnotations, ok := ProviderExposeAnnotations[util.EKSProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
	annotations, ok := exposeAnnotations[exposeType]
	if !ok {
		return nil, fmt.Errorf("unsupported expose type: %s on provider %s", exposeType, provider)
	}
	return annotations, nil
}

func GetK8SProvider(client kubernetes.Interface) (util.K8sProvider, error) {
	versionInfo, err := util.GetVersionInfo(client)
	if err != nil {
		return "", err
	}

	versionErr := fmt.Errorf("failed to get kubernetes version")
	k8sVersionStr, ok := versionInfo[util.KubernetesApp]
	if !ok {
		return "", versionErr
	}
	return util.GetK8sProvider(k8sVersionStr), nil
}
