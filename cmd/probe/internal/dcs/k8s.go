package dcs

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/dapr/kit/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	k8scomponent "github.com/apecloud/kubeblocks/cmd/probe/internal/component/kubernetes"
	v1 "github.com/apecloud/kubeblocks/cmd/probe/vendor/k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

type KubernetesStore struct {
	ctx             context.Context
	clusterName     string
	componentName   string
	clusterCompName string
	namespace       string
	cluster         *Cluster
	client          *rest.RESTClient
	clientset       *kubernetes.Clientset
	//LeaderObservedRecord *LeaderRecord
	LeaderObservedTime int64
	logger             logger.Logger
}

func NewKubernetesStore(logger logger.Logger) (*KubernetesStore, error) {
	ctx := context.Background()
	clientset, err := k8scomponent.GetClientSet()
	if err != nil {
		logger.Errorf("clientset init error: %v", err)
	}
	client, err := k8scomponent.GetRESTClient()
	if err != nil {
		logger.Errorf("restclient init error: %v", err)
	}

	clusterName := os.Getenv("KB_CLUSTER_NAME")
	componentName := os.Getenv("KB_COMP_NAME")
	clusterCompName := os.Getenv("KB_CLUSTER_COMP_NAME")
	namespace := os.Getenv("KB_NAMESPACE")
	labelsMap := map[string]string{
		"app.kubernetes.io/instance":        clusterName,
		"app.kubernetes.io/managed-by":      "kubeblocks",
		"apps.kubeblocks.io/component-name": componentName,
	}

	selector := labels.SelectorFromSet(labelsMap)
	logger.Infof("pod selector: %s", selector.String())
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, clusterCompName+"-haconfig", metav1.GetOptions{})
	if configMap == nil || err != nil {
		ttl := os.Getenv("KB_TTL")
		if _, err = clientset.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterCompName + "-haconfig",
				Namespace: namespace,
				Labels:    labelsMap,
				Annotations: map[string]string{
					"ttl":                ttl,
					"MaxLagOnSwitchover": "0",
				},
				// OwnerReferences: ownerReference,
			},
		}, metav1.CreateOptions{}); err != nil {
			return nil, err
		}
	}
	return &KubernetesStore{
		ctx:             ctx,
		clusterName:     os.Getenv("KB_CLUSTER_NAME"),
		componentName:   os.Getenv("KB_COMP_NAME"),
		clusterCompName: os.Getenv("KB_CLUSTER_COMP_NAME"),
		namespace:       os.Getenv("KB_NAMESPACE"),
		client:          client,
		clientset:       clientset,
		logger:          logger,
	}, nil
}

func (store *KubernetesStore) Initialize() error {
}
func (store *KubernetesStore) GetCluster() (*Cluster, error) {
	clusterResource := &appsv1alpha1.Cluster{}
	err := store.client.Get().
		Namespace(store.namespace).
		Resource("clusters").
		Name(store.clusterName).
		VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
		Do(store.ctx).
		Into(clusterResource)
	store.logger.Infof("cluster: %v", clusterResource)
	if err != nil {
		store.logger.Errorf("k8s get cluster error: %v", err)
	}

	var replicas int32
	for _, component := range clusterResource.Spec.ComponentSpecs {
		if component.Name == store.componentName {
			replicas = component.Replicas
			break
		}
	}

	members, err := store.GetMembers()
	if err != nil {
		store.logger.Errorf("get members error: %v", err)
	}

	switchover, err := store.GetSwitchover()
	if err != nil {
		store.logger.Errorf("get switchover error: %v", err)
	}

	haConfig, err := store.GetHaConfig()
	if err != nil {
		store.logger.Errorf("get HaConfig error: %v", err)
	}

	cluster := &Cluster{
		ClusterCompName: store.clusterCompName,
		Replicas:        replicas,
		Members:         members,
		Switchover:      switchover,
		HaConfig:        haConfig,
		clusterResource: clusterResource,
	}

	return cluster, nil
}

func (store *KubernetesStore) GetMembers() ([]Member, error) {
	labelsMap := map[string]string{
		"app.kubernetes.io/instance":        store.clusterName,
		"app.kubernetes.io/managed-by":      "kubeblocks",
		"apps.kubeblocks.io/component-name": store.componentName,
	}

	selector := labels.SelectorFromSet(labelsMap)
	store.logger.Infof("pod selector: %s", selector.String())
	podList, err := store.clientset.CoreV1().Pods(store.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	store.logger.Debugf("podlist: %d", len(podList.Items))
	members := make([]Member, len(podList.Items))
	for i, pod := range podList.Items {
		member := &members[i]
		member.name = pod.Name
		member.role = pod.Labels["app.kubernetes.io/role"]
		member.PodIP = pod.Status.PodIP
		member.DBPort = getDBPort(&pod)
		member.SQLChannelPort = getSQLChannelPort(&pod)
	}

	return members, nil
}

func (store *KubernetesStore) ResetCluser()  {}
func (store *KubernetesStore) DeleteCluser() {}
func (store *KubernetesStore) GetLeaderConfigMap() (*corev1.ConfigMap, error) {
	leaderConfigMap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(store.ctx, store.clusterCompName+"-leader", metav1.GetOptions{})
	if err != nil {
		store.logger.Errorf("Get Leader configmap failed: %v", err)
	}
	return leaderConfigMap, err
}

func (store *KubernetesStore) IsLeaderExist() (bool, error) {
	leaderConfigMap, err := store.GetLeaderConfigMap()
	return leaderConfigMap != nil, err
}

func (store *KubernetesStore) CreateLeader() error {
	leaderName := os.Getenv("KB_POD_FQDN")
	acquireTime := time.Now().Unix()
	renewTime := acquireTime
	ttl := store.haConfig.TTL
	isExist, err := store.IsLeaderExist()
	if isExist || err != nil {
		store.logger.Errorf("Get Leader failed: %v", err)
		return err
	}

	if _, err = store.clientSet.CoreV1().ConfigMaps(store.namespace).Create(store.ctx, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      store.clusterCompName + "-leader",
			Namespace: store.namespace,
			Annotations: map[string]string{
				"leader":       leaderName,
				"acquire-time": strconv.FormatInt(acquireTime, 10),
				"renew-time":   strconv.FormatInt(renewTime, 10),
				"ttl":          ttl,
				"extra":        "",
			},
		},
	}, metav1.CreateOptions{}); err != nil {
		return err
	}
}

func (store *KubernetesStore) GetLeader() (*Leader, error) {
	configmap, err := store.GetLeaderConfigMap()
	if err != nil {
		return nil, err
	}

	if configmap == nil {
		return nil, nil
	}

	annotations := configmap.Annotations
	acquireTime, err := strconv.ParseInt(annotations["acquire-time"], 10, 64)
	if err != nil {
		acquireTime = 0
	}
	renewTime, err := strconv.ParseInt(annotations["renew-time"], 10, 64)
	if err != nil {
		renewTime = 0
	}
	ttl, err := strconv.Atoi(annotations["ttl"])
	if err != nil {
		ttl = 0
	}
	return &Leader{
		index:       configmap.ResourceVersion,
		name:        annotations["leader"],
		acquireTime: accquireTime,
		renewTime:   renewTime,
		ttl:         ttl,
	}, nil

}

func (store *KubernetesStore) AttempAcquireLock() {}
func (store *KubernetesStore) HasLock()           {}
func (store *KubernetesStore) ReleaseLock()       {}

func (store *KubernetesStore) GetHaConfig() (*HaConfig, error) {
	configmapName := store.clusterCompName + "-haconfig"
	configmap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(context.TODO(), configmapName, metav1.GetOptions{})
	if err != nil {
		store.logger.Errorf("get configmap error: %v", err)
	}

	annotations := configmap.Annotations
	ttl, err := strconv.Atoi(annotations["ttl"])
	if err != nil {
		ttl = 0
	}
	maxLagOnSwitchover, err := strconv.Atoi(annotations["MaxLagOnSwitchover"])
	if err != nil {
		maxLagOnSwitchover = 1048576
	}

	return &HaConfig{
		index:              configmap.ResourceVersion,
		ttl:                ttl,
		maxLagOnSwitchover: int64(maxLagOnSwitchover),
	}, err
}

func (store *KubernetesStore) GetSwitchover() (*Switchover, error) {
	return nil, nil
}

func (store *KubernetesStore) SetSwitchover() {}
func (store *KubernetesStore) AddThisMember() {}

// TODO: Use the database instance's character type to determine its port number more precisely
func getDBPort(pod *corev1.Pod) string {
	mainContainer := pod.Spec.Containers[0]
	port := mainContainer.Ports[0]
	dbPort := port.ContainerPort
	return strconv.Itoa(int(dbPort))
}

func getSQLChannelPort(pod *corev1.Pod) string {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "probe-http-port" {
				return strconv.Itoa(int(port.ContainerPort))
			}
		}
	}
	return ""
}
