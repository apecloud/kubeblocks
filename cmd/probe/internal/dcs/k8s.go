package dcs

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	k8scomponent "github.com/apecloud/kubeblocks/cmd/probe/internal/component/kubernetes"
)

type KubernetesStore struct {
	ctx               context.Context
	clusterName       string
	componentName     string
	clusterCompName   string
	currentMemberName string
	namespace         string
	cluster           *Cluster
	client            *rest.RESTClient
	clientset         *kubernetes.Clientset
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

	store := &KubernetesStore{
		ctx:               ctx,
		clusterName:       os.Getenv("KB_CLUSTER_NAME"),
		componentName:     os.Getenv("KB_COMP_NAME"),
		clusterCompName:   os.Getenv("KB_CLUSTER_COMP_NAME"),
		currentMemberName: os.Getenv("KB_POD_FQDN"),
		namespace:         os.Getenv("KB_NAMESPACE"),
		client:            client,
		clientset:         clientset,
		logger:            logger,
	}
	cluster, err := store.GetCluster()
	store.cluster = cluster
	return store, err
}

func (store *KubernetesStore) Initialize() error {
	store.logger.Infof("k8s store initializing")
	labelsMap := map[string]string{
		"app.kubernetes.io/instance":        store.clusterName,
		"app.kubernetes.io/managed-by":      "kubeblocks",
		"apps.kubeblocks.io/component-name": store.componentName,
	}

	haName := store.clusterCompName + "-haconfig"
	store.logger.Infof("k8s store initializing, create Ha ConfigMap: %s", haName)
	configMap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(store.ctx, haName, metav1.GetOptions{})
	if configMap == nil || err != nil {
		ttl := os.Getenv("KB_TTL")
		if _, err = store.clientset.CoreV1().ConfigMaps(store.namespace).Create(store.ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      haName,
				Namespace: store.namespace,
				Labels:    labelsMap,
				Annotations: map[string]string{
					"ttl":                ttl,
					"MaxLagOnSwitchover": "0",
				},
				// OwnerReferences: ownerReference,
			},
		}, metav1.CreateOptions{}); err != nil {
			return err
		}
	}
	err = store.CreateLock()
	return err
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
	store.logger.Infof("cluster resource: %v", clusterResource)
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

	leader, err := store.GetLeader()
	if err != nil {
		store.logger.Errorf("get switchover error: %v", err)
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
		Leader:          leader,
		Switchover:      switchover,
		HaConfig:        haConfig,
		resource:        clusterResource,
	}

	store.cluster = cluster
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
		member.Name = fmt.Sprintf("%s.%s-headless.%s.svc", pod.Name, store.clusterCompName, store.namespace)
		member.Role = pod.Labels["app.kubernetes.io/role"]
		member.PodIP = pod.Status.PodIP
		member.DBPort = getDBPort(&pod)
		member.SQLChannelPort = getSQLChannelPort(&pod)
	}

	return members, nil
}

func (store *KubernetesStore) ResetCluser()  {}
func (store *KubernetesStore) DeleteCluser() {}
func (store *KubernetesStore) GetLeaderConfigMap() (*corev1.ConfigMap, error) {
	leaderName := store.clusterCompName + "-leader"
	leaderConfigMap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(store.ctx, leaderName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			store.logger.Errorf("Leader configmap [%s] is not found", leaderName)
			return nil, nil
		}
		store.logger.Errorf("Get Leader configmap failed: %v", err)
	}
	return leaderConfigMap, err
}

func (store *KubernetesStore) IsLockExist() (bool, error) {
	leaderConfigMap, err := store.GetLeaderConfigMap()
	return leaderConfigMap != nil, err
}

func (store *KubernetesStore) CreateLock() error {
	leaderName := store.currentMemberName
	now := time.Now().Unix()
	nowStr := strconv.FormatInt(now, 10)
	ttl := store.cluster.HaConfig.ttl
	isExist, err := store.IsLockExist()
	if isExist || err != nil {
		return err
	}

	store.logger.Infof("k8s store initializing, create leader ConfigMap: %s", leaderName)
	if _, err = store.clientset.CoreV1().ConfigMaps(store.namespace).Create(store.ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      store.clusterCompName + "-leader",
			Namespace: store.namespace,
			Annotations: map[string]string{
				"leader":       leaderName,
				"acquire-time": nowStr,
				"renew-time":   nowStr,
				"ttl":          strconv.Itoa(ttl),
				"extra":        "",
			},
		},
	}, metav1.CreateOptions{}); err != nil {
		store.logger.Errorf("Create Leader ConfigMap failed: %v", err)
		return err
	}
	return nil
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
		acquireTime: acquireTime,
		renewTime:   renewTime,
		ttl:         ttl,
		resource:    configmap,
	}, nil
}

func (store *KubernetesStore) AttempAcquireLock() error {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	ttl := store.cluster.HaConfig.ttl
	leaderName := store.currentMemberName
	annotation := map[string]string{
		"leader":       leaderName,
		"ttl":          strconv.Itoa(ttl),
		"renew-time":   now,
		"acquire-time": now,
	}

	configMap := store.cluster.Leader.resource.(*corev1.ConfigMap)
	configMap.SetAnnotations(annotation)
	_, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})

	return err
}

func (store *KubernetesStore) HasLock() bool {
	return store.cluster.Leader != nil && store.cluster.Leader.name == store.currentMemberName
}

func (store *KubernetesStore) UpdateLock() error {
	configMap := store.cluster.Leader.resource.(*corev1.ConfigMap)

	annotations := configMap.GetAnnotations()
	if annotations["leader"] != store.currentMemberName {
		return errors.Errorf("lost lock")
	}
	annotations["renew-time"] = strconv.FormatInt(time.Now().Unix(), 10)
	configMap.SetAnnotations(annotations)

	_, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	return err
}

func (store *KubernetesStore) ReleaseLock() error {
	configMap := store.cluster.Leader.resource.(*corev1.ConfigMap)
	configMap.Annotations["leader"] = ""
	_, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	// TODO: if response status code is 409, it means operation conflict.
	return err
}

func (store *KubernetesStore) GetHaConfig() (*HaConfig, error) {
	configmapName := store.clusterCompName + "-haconfig"
	configmap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(context.TODO(), configmapName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			store.logger.Errorf("Get ha configmap [%s] error: %v", configmapName, err)
		} else {
			err = nil
		}
		return &HaConfig{
			index:              "",
			ttl:                0,
			maxLagOnSwitchover: 1048576,
		}, err
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

func (store *KubernetesStore) SetSwitchover() error {
	return nil
}

func (store *KubernetesStore) AddCurrentMember() error {
	return nil
}

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
