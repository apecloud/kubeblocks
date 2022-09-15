package dbcluster

import (
	"context"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"
	"os"
)

type CreateOptions struct {
	Namespace     string
	Name          string
	ClusterDefRef string
	AppVersionRef string

	FilePath string

	BuilderArgs []string

	DescriberSettings *describe.DescriberSettings

	client dynamic.Interface
	genericclioptions.IOStreams
}

func NewCreateCmd(f cmdutil.Factory) *cobra.Command {

	o := &CreateOptions{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a database cluster",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.FilePath, "file", "f", "", "Use yaml file to create cluster")
	cmd.Flags().StringVar(&o.Name, "name", "", "DBCluster name")
	cmd.Flags().StringVar(&o.Namespace, "namespace", "default", "DBCluster namespace")
	cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "ClusterDefinition reference")
	cmd.Flags().StringVar(&o.AppVersionRef, "app-version", "", "AppVersion reference")

	return cmd
}

func (o *CreateOptions) Validate() error {
	if len(o.FilePath) > 0 {
		return nil
	}
	if len(o.Name) == 0 {
		return fmt.Errorf("name can not be empty")
	}
	if len(o.ClusterDefRef) == 0 {
		return fmt.Errorf("cluster-definition can not be empty")
	}
	if len(o.AppVersionRef) == 0 {
		return fmt.Errorf("app-version can not be empty")
	}
	return nil
}

func (o *CreateOptions) Complete(f cmdutil.Factory, args []string) error {

	// used to fetch the resource
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.client = client

	return nil
}

func (o *CreateOptions) Run() error {
	clusterObj := unstructured.Unstructured{}
	if len(o.FilePath) > 0 {
		fileByte, err := os.ReadFile(o.FilePath)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(fileByte, &clusterObj); err != nil {
			return nil
		}
	} else {
		clusterYamlByte := []byte(`
apiVersion: dbaas.infracreate.com/v1alpha1
kind: Cluster
metadata:
  name: mysql-cluster-01
  namespace: default
spec:
  clusterDefinitionRef: mysql-cluster-definition
  appVersionRef: appversion-mysql-latest
  components:
  - name: replicasets
    type: replicasets
    roleGroups:
    - name: primary
      type: primary
      replicas: 3
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
    - name: log
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
`)
		if err := yaml.Unmarshal(clusterYamlByte, &clusterObj); err != nil {
			return err
		}
	}
	gvr := schema.GroupVersionResource{Group: "dbaas.infracreate.com", Version: "v1alpha1", Resource: "clusters"}
	_, err := o.client.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), &clusterObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
