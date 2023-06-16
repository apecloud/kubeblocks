package dcs

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	k8scomponent "github.com/apecloud/kubeblocks/cmd/probe/internal/component/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

type KubernetesStore struct {
	ctx             context.Context
	clusterName     string
	clusterCompName string
	namespace       string
	cluster         *Cluster
	client          *rest.RESTClient
	clientset       *kubernetes.Clientset
	//LeaderObservedRecord *LeaderRecord
	LeaderObservedTime int64
}

func NewKubernetesStore() *KubernetesStore {
	ctx := context.Background()
	clientset, err := k8scomponent.GetClientSet()
	if err != nil {
		panic(err)
	}
	client, err := k8scomponent.GetRESTClient()
	if err != nil {
		panic(err)
	}
	cluster := &appsv1alpha1.Cluster{}
	err = client.Get().
		Namespace("default").
		Resource("clusters").
		Name("mongo-cluster").
		VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
		Do(ctx).
		Into(cluster)
	fmt.Printf("cluster: %v\n", cluster)
	if err != nil {
		panic(err)
	}

	return &KubernetesStore{
		ctx:             ctx,
		clusterName:     os.Getenv("KB_CLUSTER_NAME"),
		clusterCompName: os.Getenv("KB_CLUSTER_COMP_NAME"),
		namespace:       os.Getenv("KB_NAMESPACE"),
		client:          client,
		clientset:       clientset,
	}
}

func (store *KubernetesStore) GetCluster() error {
	return nil
}
