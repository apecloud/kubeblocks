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

	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	k8scomponent "github.com/apecloud/kubeblocks/cmd/probe/internal/component/kubernetes"
	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
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
	logger             logger.Logger
}

func NewKubernetesStore(logger logger.Logger) (*KubernetesStore, error) {
	ctx := context.Background()
	clientset, err := k8scomponent.GetClientSet(logger)
	if err != nil {
		err = errors.Wrap(err, "clientset init failed")
		return nil, err
	}
	client, err := k8scomponent.GetRESTClient(logger)
	if err != nil {
		err = errors.Wrap(err, "restclient init failed")
		return nil, err
	}

	clusterName := os.Getenv(constant.KBEnvClusterName)
	if clusterName == "" {
		return nil, errors.New("KB_CLUSTER_NAME must be set")
	}

	componentName := os.Getenv(constant.KBEnvComponentName)
	if componentName == "" {
		return nil, errors.New("KB_CCMP_NAME must be set")
	}

	clusterCompName := os.Getenv(constant.KBEnvClusterCompName)
	if clusterCompName == "" {
		return nil, errors.New("KB_CLUSTER_COMP_NAME must be set")
	}

	currentMemberName := os.Getenv(constant.KBEnvPodName)
	if clusterName == "" {
		return nil, errors.New("KB_POD_NAME must be set")
	}

	namespace := os.Getenv(constant.KBEnvNamespace)
	if namespace == "" {
		return nil, errors.New("KB_NAMESPACE must be set")
	}

	store := &KubernetesStore{
		ctx:               ctx,
		clusterName:       clusterName,
		componentName:     componentName,
		clusterCompName:   clusterCompName,
		currentMemberName: currentMemberName,
		namespace:         namespace,
		client:            client,
		clientset:         clientset,
		logger:            logger,
	}
	dcs = store
	return store, err
}

func (store *KubernetesStore) Initialize(cluster *Cluster) error {
	store.logger.Infof("k8s store initializing")
	_, err := store.GetCluster()
	if err != nil {
		return err
	}

	err = store.CreateHaConfig(cluster)
	if err != nil {
		store.logger.Warnf("Create Ha ConfigMap failed: %s", err)
	}

	err = store.CreateLock()
	if err != nil {
		store.logger.Warnf("Create Leader ConfigMap failed: %s", err)
	}
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
		store.logger.Errorf("k8s get cluster error: %v", err)
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
		store.logger.Errorf("get members error: %v", err)
	}

	leader, err := store.GetLeader()
	if err != nil {
		store.logger.Errorf("get leader error: %v", err)
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
		constant.AppInstanceLabelKey:    store.clusterName,
		constant.AppManagedByLabelKey:   "kubeblocks",
		constant.KBAppComponentLabelKey: store.componentName,
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
		member.Name = pod.Name
		// member.Name = fmt.Sprintf("%s.%s-headless.%s.svc", pod.Name, store.clusterCompName, store.namespace)
		member.Role = pod.Labels["app.kubernetes.io/role"]
		member.PodIP = pod.Status.PodIP
		member.DBPort = getDBPort(&pod)
		member.SQLChannelPort = getSQLChannelPort(&pod)
		member.UID = string(pod.UID)
		member.resource = pod.DeepCopy()
	}

	return members, nil
}

func (store *KubernetesStore) ResetCluser()  {}
func (store *KubernetesStore) DeleteCluser() {}

func (store *KubernetesStore) GetLeaderConfigMap() (*corev1.ConfigMap, error) {
	leaderName := store.getLeaderName()
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
	appCluster, ok := store.cluster.resource.(*appsv1alpha1.Cluster)
	if leaderConfigMap != nil && ok && leaderConfigMap.CreationTimestamp.Before(&appCluster.CreationTimestamp) {
		store.logger.Infof("A previous leader configmap resource exists, delete it %s", leaderConfigMap.Name)
		_ = store.DeleteLeader()
		return false, nil
	}
	return leaderConfigMap != nil, err
}

func (store *KubernetesStore) CreateLock() error {
	isExist, err := store.IsLockExist()
	if isExist || err != nil {
		return err
	}

	leaderConfigMapName := store.getLeaderName()
	leaderName := store.currentMemberName
	now := time.Now().Unix()
	nowStr := strconv.FormatInt(now, 10)
	ttl := viper.GetString(constant.KBEnvTTL)
	leaderConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: leaderConfigMapName,
			Annotations: map[string]string{
				"leader":       leaderName,
				"acquire-time": nowStr,
				"renew-time":   nowStr,
				"ttl":          ttl,
				"extra":        "",
			},
		},
	}

	store.logger.Infof("K8S store initializing, create leader ConfigMap: %s", leaderConfigMapName)
	err = store.createConfigMap(leaderConfigMap)
	if err != nil {
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
		ttl = viper.GetInt("KB_TTL")
	}
	leader := annotations["leader"]
	stateStr, ok := annotations["dbstate"]
	var dbState *DBState
	if ok {
		dbState = new(DBState)
		err = json.Unmarshal([]byte(stateStr), &dbState)
		if err != nil {
			store.logger.Infof("get leader dbstate failed: %v, annotations: %v", err, annotations)
		}
	}

	if ttl > 0 && time.Now().Unix()-renewTime > int64(ttl) {
		store.logger.Infof("lock expired: %v, now: %d", annotations, time.Now().Unix())
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
	leaderName := store.getLeaderName()
	err := store.clientset.CoreV1().ConfigMaps(store.namespace).Delete(store.ctx, leaderName, metav1.DeleteOptions{})
	if err != nil {
		store.logger.Errorf("Delete leader configmap failed: %v", err)
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
		store.logger.Errorf("Acquire lock failed: %v", err)
	} else {
		store.cluster.Leader.Resource = cm
	}

	return err
}

func (store *KubernetesStore) HasLock() bool {
	return store.cluster != nil && store.cluster.Leader != nil && store.cluster.Leader.Name == store.currentMemberName
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
		store.logger.Errorf("release lock failed: %v", err)
	}
	// TODO: if response status code is 409, it means operation conflict.
	return err
}

func (store *KubernetesStore) CreateHaConfig(cluster *Cluster) error {
	haName := store.getHAConfigName()
	haConfig, _ := store.GetHaConfig()
	if haConfig.resource != nil {
		return nil
	}

	store.logger.Infof("Create Ha ConfigMap: %s", haName)
	ttl := viper.GetString(constant.KBEnvTTL)
	maxLag := viper.GetString("KB_MAX_LAG")
	haConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: haName,
			Annotations: map[string]string{
				"ttl":                ttl,
				"MaxLagOnSwitchover": maxLag,
			},
		},
	}

	err := store.createConfigMap(haConfigMap)
	if err != nil {
		store.logger.Infof("Create Ha ConfigMap failed: %v", err)
	}
	return err
}

func (store *KubernetesStore) GetHaConfig() (*HaConfig, error) {
	configmapName := store.getHAConfigName()
	configmap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(context.TODO(), configmapName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			store.logger.Errorf("Get ha configmap [%s] error: %v", configmapName, err)
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
	deleteMembers := make(map[string]MemberToDelete)
	str := annotations["delete-members"]
	if str != "" {
		err := json.Unmarshal([]byte(str), &deleteMembers)
		if err != nil {
			store.logger.Errorf("Get delete members [%s] errors: %v", str, err)
		}
	}

	return &HaConfig{
		index:              configmap.ResourceVersion,
		ttl:                ttl,
		maxLagOnSwitchover: int64(maxLagOnSwitchover),
		DeleteMembers:      deleteMembers,
		resource:           configmap,
	}, err
}

func (store *KubernetesStore) UpdateHaConfig() error {
	haConfig := store.cluster.HaConfig
	if haConfig.resource == nil {
		return errors.New("No HA configmap")
	}

	configMap := haConfig.resource.(*corev1.ConfigMap)
	annotations := configMap.Annotations
	annotations["ttl"] = strconv.Itoa(haConfig.ttl)
	deleteMembers, err := json.Marshal(haConfig.DeleteMembers)
	if err != nil {
		store.logger.Errorf("marsha delete members [%v] errors: %v", haConfig, err)
	}
	annotations["delete-members"] = string(deleteMembers)
	annotations["MaxLagOnSwitchover"] = strconv.Itoa(int(haConfig.maxLagOnSwitchover))

	_, err = store.clientset.CoreV1().ConfigMaps(store.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	return err
}

func (store *KubernetesStore) GetSwitchOverConfigMap() (*corev1.ConfigMap, error) {
	switchoverName := store.getSwitchoverName()
	switchOverConfigMap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(store.ctx, switchoverName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			store.logger.Debugf("no switchOver [%s] setting", switchoverName)
			return nil, nil
		}
		store.logger.Errorf("Get switchOver configmap failed: %v", err)
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
	switchoverName := store.getSwitchoverName()
	switchover, _ := store.GetSwitchover()
	if switchover != nil {
		return fmt.Errorf("there is another switchover %s unfinished", switchoverName)
	}

	store.logger.Infof("Create switchover configmap %s", switchoverName)
	swConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: switchoverName,
			Annotations: map[string]string{
				"leader":    leader,
				"candidate": candidate,
			},
		},
	}

	err := store.createConfigMap(swConfigMap)
	if err != nil {
		store.logger.Infof("Create switchover configmap failed %v", err)
		return err
	}
	return nil
}

func (store *KubernetesStore) DeleteSwitchover() error {
	switchoverName := store.getSwitchoverName()
	err := store.clientset.CoreV1().ConfigMaps(store.namespace).Delete(store.ctx, switchoverName, metav1.DeleteOptions{})
	if err != nil {
		store.logger.Errorf("Delete switchOver configmap failed: %v", err)
	}
	return err
}

func (store *KubernetesStore) getLeaderName() string {
	return store.clusterCompName + "-leader"
}

func (store *KubernetesStore) getHAConfigName() string {
	return store.clusterCompName + "-haconfig"
}

func (store *KubernetesStore) getSwitchoverName() string {
	return store.clusterCompName + "-switchover"
}

func (store *KubernetesStore) createConfigMap(configMap *corev1.ConfigMap) error {
	labelsMap := map[string]string{
		"app.kubernetes.io/instance":        store.clusterName,
		"app.kubernetes.io/managed-by":      "kubeblocks",
		"apps.kubeblocks.io/component-name": store.componentName,
	}

	configMap.Labels = labelsMap
	configMap.Namespace = store.namespace
	configMap.OwnerReferences = []metav1.OwnerReference{getOwnerRef(store.cluster)}
	_, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Create(store.ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}
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

func getOwnerRef(cluster *Cluster) metav1.OwnerReference {
	clusterObj := cluster.resource.(*appsv1alpha1.Cluster)
	gvk, _ := apiutil.GVKForObject(clusterObj, scheme.Scheme)
	ownerRef := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        clusterObj.UID,
		Name:       clusterObj.Name,
	}
	return ownerRef
}
