package cluster

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

const (
	k = "service.kubernetes.io/apecloud-loadbalancer-type"
	v = "private-ip"
)

type ExposeOptions struct {
	Namespace string
	Name      string
	reverse   bool

	client dynamic.Interface
	genericclioptions.IOStreams
}

func NewExposeCmd(f cmdutil.Factory) *cobra.Command {
	o := &ExposeOptions{}

	cmd := &cobra.Command{
		Use:   "expose",
		Short: "Expose a database cluster",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().BoolVar(&o.reverse, "reverse", o.reverse, "Stop expose a database cluster")

	return cmd
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
	clusterGVR := schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: types.ResourceClusters}
	_, err := o.client.Resource(clusterGVR).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	serviceGVR := schema.GroupVersionResource{Version: "v1", Resource: "services"}
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "app.kubernetes.io/instance", o.Name),
	}
	svcList, err := o.client.Resource(serviceGVR).Namespace(o.Namespace).List(context.TODO(), opts)
	if err != nil {
		return errors.Wrap(err, "Failed to find related services")
	}

	for _, svc := range svcList.Items {
		annotations := svc.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		if !o.reverse {
			annotations[k] = v
		} else {
			delete(annotations, k)
		}
		svc.SetAnnotations(annotations)
		_, err := o.client.Resource(serviceGVR).Namespace(o.Namespace).Update(context.TODO(), &svc, metav1.UpdateOptions{})
		if err != nil {
			return errors.Wrapf(err, "Failed to update service %s/%s", svc.GetNamespace(), svc.GetName())
		}
	}
	return nil
}
