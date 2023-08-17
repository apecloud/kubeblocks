/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package dcs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
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
	ctx                context.Context
	clusterName        string
	componentName      string
	clusterCompName    string
	currentMemberName  string
	namespace          string
	cluster            *Cluster
	client             *rest.RESTClient
	clientset          *kubernetes.Clientset
	LeaderObservedTime int64
	logger             logr.Logger
}

func NewKubernetesStore(logger logr.Logger) (*KubernetesStore, error) {
	ctx := context.Background()
	clientset, err := k8scomponent.GetClientSet(logger)
	if err != nil {
		logger.Error(err, "clientset init error")
	}
	client, err := k8scomponent.GetRESTClient(logger)
	if err != nil {
		logger.Error(err, "restclient init error")
	}

	store := &KubernetesStore{
		ctx:               ctx,
		clusterName:       os.Getenv("KB_CLUSTER_NAME"),
		componentName:     os.Getenv("KB_COMP_NAME"),
		clusterCompName:   os.Getenv("KB_CLUSTER_COMP_NAME"),
		currentMemberName: os.Getenv("KB_POD_NAME"),
		namespace:         os.Getenv("KB_NAMESPACE"),
		client:            client,
		clientset:         clientset,
		logger:            logger,
	}
	dcs = store
	return store, err
}

func (store *KubernetesStore) Initialize() error {
	store.logger.Info("k8s store initializing")
	_, err := store.GetCluster()
	if err != nil {
		return err
	}

	labelsMap := map[string]string{
		"app.kubernetes.io/instance":        store.clusterName,
		"app.kubernetes.io/managed-by":      "kubeblocks",
		"apps.kubeblocks.io/component-name": store.componentName,
	}

	haName := store.clusterCompName + "-haconfig"
	store.logger.Info(fmt.Sprintf("k8s store initializing, create Ha ConfigMap: %s", haName))
	configMap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(store.ctx, haName, metav1.GetOptions{})
	if configMap == nil || err != nil {
		ttl := viper.GetString("KB_TTL")
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

func (store *KubernetesStore) GetClusterName() string {
	return store.clusterName
}

func (store *KubernetesStore) GetClusterFromCache() *Cluster {
	return store.cluster
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
	// store.logger.Debugf("cluster resource: %v", clusterResource)
	if err != nil {
		store.logger.Error(err, "k8s get cluster error")
		return nil, err
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
		store.logger.Error(err, "get members error")
	}

	leader, err := store.GetLeader()
	if err != nil {
		store.logger.Error(err, "get switchover error")
	}

	switchover, err := store.GetSwitchover()
	if err != nil {
		store.logger.Error(err, "get switchover error")
	}

	haConfig, err := store.GetHaConfig()
	if err != nil {
		store.logger.Error(err, "get HaConfig error")
	}

	cluster := &Cluster{
		ClusterCompName: store.clusterCompName,
		Namespace:       store.namespace,
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
	store.logger.Info(fmt.Sprintf("pod selector: %s", selector.String()))
	podList, err := store.clientset.CoreV1().Pods(store.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	store.logger.Info(fmt.Sprintf("podlist: %d", len(podList.Items)))
	members := make([]Member, len(podList.Items))
	for i, pod := range podList.Items {
		member := &members[i]
		member.Name = pod.Name
		// member.Name = fmt.Sprintf("%s.%s-headless.%s.svc", pod.Name, store.clusterCompName, store.namespace)
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
			store.logger.Error(err, fmt.Sprintf("Leader configmap [%s] is not found", leaderName))
			return nil, nil
		}
		store.logger.Error(err, "Get Leader configmap failed")
	}
	return leaderConfigMap, err
}

func (store *KubernetesStore) IsLockExist() (bool, error) {
	leaderConfigMap, err := store.GetLeaderConfigMap()
	appCluster, ok := store.cluster.resource.(*appsv1alpha1.Cluster)
	if leaderConfigMap != nil && ok && leaderConfigMap.CreationTimestamp.Before(&appCluster.CreationTimestamp) {
		store.logger.Info("A previous leader configmap resource exists, delete it", "name", leaderConfigMap.Name)
		_ = store.DeleteLeader()
		return false, nil
	}
	return leaderConfigMap != nil, err
}

func (store *KubernetesStore) CreateLock() error {
	leaderName := store.currentMemberName
	now := time.Now().Unix()
	nowStr := strconv.FormatInt(now, 10)
	ttl := viper.GetString("KB_TTL")
	isExist, err := store.IsLockExist()
	if isExist || err != nil {
		return err
	}

	leaderConfigMapName := store.clusterCompName + "-leader"
	store.logger.Info("K8S store initializing, create leader ConfigMap", "leadername", leaderConfigMapName)
	if _, err = store.clientset.CoreV1().ConfigMaps(store.namespace).Create(store.ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaderConfigMapName,
			Namespace: store.namespace,
			Annotations: map[string]string{
				"leader":       leaderName,
				"acquire-time": nowStr,
				"renew-time":   nowStr,
				"ttl":          ttl,
				"extra":        "",
			},
		},
	}, metav1.CreateOptions{}); err != nil {
		store.logger.Error(err, "Create Leader ConfigMap failed")
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
		ttl = viper.GetInt("KB_TTL")
	}
	leader := annotations["leader"]
	stateStr, ok := annotations["dbstate"]
	var dbState *DBState
	if ok {
		dbState = new(DBState)
		err = json.Unmarshal([]byte(stateStr), &dbState)
		if err != nil {
			store.logger.Error(err, fmt.Sprintf("get leader dbstate failed, annotations: %v", annotations))
		}
	}

	if ttl > 0 && time.Now().Unix()-renewTime > int64(ttl) {
		store.logger.Info(fmt.Sprintf("lock expired: %v, now: %d", annotations, time.Now().Unix()))
		leader = ""
	}

	return &Leader{
		Index:       configmap.ResourceVersion,
		Name:        leader,
		AcquireTime: acquireTime,
		RenewTime:   renewTime,
		TTL:         ttl,
		Resource:    configmap,
		DBState:     dbState,
	}, nil
}

func (store *KubernetesStore) DeleteLeader() error {
	leaderName := store.clusterCompName + "-leader"
	err := store.clientset.CoreV1().ConfigMaps(store.namespace).Delete(store.ctx, leaderName, metav1.DeleteOptions{})
	if err != nil {
		store.logger.Error(err, "Delete leader configmap failed")
	}
	return err
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

	configMap := store.cluster.Leader.Resource.(*corev1.ConfigMap)
	configMap.SetAnnotations(annotation)
	if store.cluster.Leader.DBState != nil {
		str, _ := json.Marshal(store.cluster.Leader.DBState)
		configMap.Annotations["dbstate"] = string(str)
	}
	cm, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		store.logger.Error(err, "Acquire lock failed")
	} else {
		store.cluster.Leader.Resource = cm
	}

	return err
}

func (store *KubernetesStore) HasLock() bool {
	return store.cluster.Leader != nil && store.cluster.Leader.Name == store.currentMemberName
}

func (store *KubernetesStore) UpdateLock() error {
	configMap := store.cluster.Leader.Resource.(*corev1.ConfigMap)

	annotations := configMap.GetAnnotations()
	if annotations["leader"] != store.currentMemberName {
		return errors.Errorf("lost lock")
	}
	ttl := store.cluster.HaConfig.ttl
	annotations["ttl"] = strconv.Itoa(ttl)
	annotations["renew-time"] = strconv.FormatInt(time.Now().Unix(), 10)

	if store.cluster.Leader.DBState != nil {
		str, _ := json.Marshal(store.cluster.Leader.DBState)
		configMap.Annotations["dbstate"] = string(str)
	}
	configMap.SetAnnotations(annotations)

	_, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	return err
}

func (store *KubernetesStore) ReleaseLock() error {
	store.logger.Info("release lock")
	configMap := store.cluster.Leader.Resource.(*corev1.ConfigMap)
	configMap.Annotations["leader"] = ""

	if store.cluster.Leader.DBState != nil {
		str, _ := json.Marshal(store.cluster.Leader.DBState)
		configMap.Annotations["dbstate"] = string(str)
	}
	_, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		store.logger.Error(err, "release lock failed")
	}
	// TODO: if response status code is 409, it means operation conflict.
	return err
}

func (store *KubernetesStore) GetHaConfig() (*HaConfig, error) {
	configmapName := store.clusterCompName + "-haconfig"
	configmap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(context.TODO(), configmapName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			store.logger.Error(err, fmt.Sprintf("Get ha configmap [%s] error", configmapName))
		} else {
			err = nil
		}
		return &HaConfig{
			index:              "",
			ttl:                viper.GetInt("KB_TTL"),
			maxLagOnSwitchover: 1048576,
		}, err
	}

	annotations := configmap.Annotations
	ttl, err := strconv.Atoi(annotations["ttl"])
	if err != nil {
		ttl = viper.GetInt("KB_TTL")
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

func (store *KubernetesStore) GetSwitchOverConfigMap() (*corev1.ConfigMap, error) {
	switchoverName := store.clusterCompName + "-switchover"
	switchOverConfigMap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(store.ctx, switchoverName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			store.logger.Info(fmt.Sprintf("no switchOver [%s] setting", switchoverName))
			return nil, nil
		}
		store.logger.Error(err, "Get switchOver configmap failed")
	}
	return switchOverConfigMap, err
}

func (store *KubernetesStore) GetSwitchover() (*Switchover, error) {
	switchOverConfigMap, _ := store.GetSwitchOverConfigMap()
	if switchOverConfigMap == nil {
		return nil, nil
	}
	annotations := switchOverConfigMap.Annotations
	scheduledAt, _ := strconv.Atoi(annotations["scheduled-at"])
	switchOver := newSwitchover(switchOverConfigMap.ResourceVersion, annotations["leader"], annotations["candidate"], int64(scheduledAt))
	return switchOver, nil
}

func (store *KubernetesStore) CreateSwitchover(leader, candidate string) error {
	switchoverName := store.clusterCompName + "-switchover"
	switchover, _ := store.GetSwitchover()
	if switchover != nil {
		return fmt.Errorf("there is another switchover %s unfinished", switchoverName)
	}

	labelsMap := map[string]string{
		"app.kubernetes.io/instance":        store.clusterName,
		"app.kubernetes.io/managed-by":      "kubeblocks",
		"apps.kubeblocks.io/component-name": store.componentName,
	}

	if _, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Create(store.ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      switchoverName,
			Namespace: store.namespace,
			Labels:    labelsMap,
			Annotations: map[string]string{
				"leader":    leader,
				"candidate": candidate,
			},
			// OwnerReferences: ownerReference,
		},
	}, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

func (store *KubernetesStore) DeleteSwitchover() error {
	switchoverName := store.clusterCompName + "-switchover"
	err := store.clientset.CoreV1().ConfigMaps(store.namespace).Delete(store.ctx, switchoverName, metav1.DeleteOptions{})
	if err != nil {
		store.logger.Error(err, "Delete switchOver configmap failed")
	}
	return err
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
