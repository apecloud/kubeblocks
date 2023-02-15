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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type ExposeOptions struct {
	Namespace string
	Name      string
	on        bool
	off       bool

	client dynamic.Interface
	genericclioptions.IOStreams
}

func NewExposeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ExposeOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:               "expose NAME",
		Short:             "Expose a database cluster",
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Validate(args))
			util.CheckErr(o.Complete(f, args))
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().BoolVar(&o.on, "on", false, "Expose a database cluster")
	cmd.Flags().BoolVar(&o.off, "off", false, "Stop expose a database cluster")

	return cmd
}

func (o *ExposeOptions) Validate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing cluster name")
	}

	if o.on == o.off {
		return fmt.Errorf("invalid options")
	}
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

	o.client, err = f.DynamicClient()
	return err
}

func (o *ExposeOptions) Run() error {
	_, err := o.client.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	serviceGVR := schema.GroupVersionResource{Version: "v1", Resource: "services"}
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "app.kubernetes.io/instance", o.Name),
	}
	svcList, err := o.client.Resource(serviceGVR).Namespace(o.Namespace).List(context.TODO(), opts)
	if err != nil {
		return errors.Wrap(err, "failed to find related services")
	}

	for _, item := range svcList.Items {

		svc := &corev1.Service{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, svc); err != nil {
			return errors.Wrap(err, "failed to convert service")
		}
		// ignore headless service
		if svc.Spec.ClusterIP == corev1.ClusterIPNone {
			continue
		}

		annotations := item.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		if o.on {
			annotations[types.ServiceLBTypeAnnotationKey] = types.ServiceLBTypeAnnotationValue
		} else if o.off {
			delete(annotations, types.ServiceLBTypeAnnotationKey)
		}
		item.SetAnnotations(annotations)
		_, err = o.client.Resource(serviceGVR).Namespace(o.Namespace).Update(context.TODO(), &item, metav1.UpdateOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to update service %s/%s", item.GetNamespace(), svc.GetName())
		}
	}

	if o.on {
		fmt.Fprintf(o.Out, "Cluster %s is exposed\n", o.Name)
	} else if o.off {
		fmt.Fprintf(o.Out, "Cluster %s stopped exposing\n", o.Name)
	}
	return nil
}
