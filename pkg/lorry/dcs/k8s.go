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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	k8s "github.com/apecloud/kubeblocks/pkg/lorry/util/kubernetes"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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

func NewKubernetesStore() (*KubernetesStore, error) {
	ctx := context.Background()
	logger := ctrl.Log.WithName("DCS-K8S")
	clientset, err := k8s.GetClientSet()
	if err != nil {
		err = errors.Wrap(err, "clientset init failed")
		return nil, err
	}
	client, err := k8s.GetRESTClientForKB()
	if err != nil {
		err = errors.Wrap(err, "restClient init failed")
		return nil, err
	}

	clusterName := os.Getenv(constant.KBEnvClusterName)
	if clusterName == "" {
		return nil, errors.New(fmt.Sprintf("%s must be set", constant.KBEnvClusterName))
	}

	componentName := os.Getenv(constant.KBEnvCompName)
	if componentName == "" {
		return nil, errors.New(fmt.Sprintf("%s must be set", constant.KBEnvCompName))
	}

	clusterCompName := os.Getenv(constant.KBEnvClusterCompName)
	if clusterCompName == "" {
		clusterCompName = clusterName + "-" + componentName
	}

	currentMemberName := os.Getenv(constant.KBEnvPodName)
	if clusterName == "" {
		return nil, errors.New(fmt.Sprintf("%s must be set", constant.KBEnvPodName))
	}

	namespace := os.Getenv(constant.KBEnvNamespace)
	if namespace == "" {
		return nil, errors.New(fmt.Sprintf("%s must be set", constant.KBEnvNamespace))
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
	return store, err
}

func (store *KubernetesStore) Initialize() error {
	store.logger.Info("k8s store initializing")
	_, err := store.GetCluster()
	if err != nil {
		return err
	}

	err = store.CreateHaConfig()
	if err != nil {
		store.logger.Error(err, "Create Ha ConfigMap failed")
	}

	err = store.CreateLease()
	if err != nil {
		store.logger.Error(err, "Create Leader ConfigMap failed")
	}
	return err
}

func (store *KubernetesStore) GetClusterName() string {
	return store.clusterName
}

func (store *KubernetesStore) SetCompName(componentName string) {
	store.componentName = componentName
	store.clusterCompName = store.clusterName + "-" + componentName
}

func (store *KubernetesStore) GetClusterFromCache() *Cluster {
	if store.cluster != nil {
		return store.cluster
	}
	cluster, _ := store.GetCluster()
	return cluster
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

	var members []Member
	if store.cluster != nil && int(replicas) == len(store.cluster.Members) {
		members = store.cluster.Members
	} else {
		members, err = store.GetMembers()
		if err != nil {
			return nil, err
		}
	}

	leader, err := store.GetLeader()
	if err != nil {
		store.logger.Info("get leader failed", "error", err)
	}

	switchover, err := store.GetSwitchover()
	if err != nil {
		store.logger.Info("get switchover failed", "error", err)
	}

	haConfig, err := store.GetHaConfig()
	if err != nil {
		store.logger.Info("get HaConfig failed", "error", err)
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
		member.Role = pod.Labels[constant.RoleLabelKey]
		member.PodIP = pod.Status.PodIP
		member.DBPort = getDBPort(&pod)
		member.LorryPort = getLorryPort(&pod)
		member.UID = string(pod.UID)
		member.resource = pod.DeepCopy()
	}

	return members, nil
}

func (store *KubernetesStore) ResetCluster()  {}
func (store *KubernetesStore) DeleteCluster() {}

func (store *KubernetesStore) GetLeaderConfigMap() (*corev1.ConfigMap, error) {
	leaderName := store.getLeaderName()
	leaderConfigMap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(store.ctx, leaderName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			store.logger.Info("Leader configmap is not found", "configmap", leaderName)
			return nil, nil
		}
		store.logger.Error(err, "Get Leader configmap failed")
	}
	return leaderConfigMap, err
}

func (store *KubernetesStore) IsLeaseExist() (bool, error) {
	leaderConfigMap, err := store.GetLeaderConfigMap()
	appCluster, ok := store.cluster.resource.(*appsv1alpha1.Cluster)
	if leaderConfigMap != nil && ok && leaderConfigMap.CreationTimestamp.Before(&appCluster.CreationTimestamp) {
		store.logger.Info("A previous leader configmap resource exists, delete it", "name", leaderConfigMap.Name)
		_ = store.DeleteLeader()
		return false, nil
	}
	return leaderConfigMap != nil, err
}

func (store *KubernetesStore) CreateLease() error {
	isExist, err := store.IsLeaseExist()
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

	store.logger.Info(fmt.Sprintf("K8S store initializing, create leader ConfigMap: %s", leaderConfigMapName))
	err = store.createConfigMap(leaderConfigMap)
	if err != nil {
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
		ttl = viper.GetInt(constant.KBEnvTTL)
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
	leaderName := store.getLeaderName()
	err := store.clientset.CoreV1().ConfigMaps(store.namespace).Delete(store.ctx, leaderName, metav1.DeleteOptions{})
	if err != nil {
		store.logger.Error(err, "Delete leader configmap failed")
	}
	return err
}

func (store *KubernetesStore) AttemptAcquireLease() error {
	timestamp := time.Now().Unix()
	now := strconv.FormatInt(timestamp, 10)
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
		store.logger.Error(err, "Acquire lease failed")
		return err
	}

	store.cluster.Leader.Resource = cm
	store.cluster.Leader.AcquireTime = timestamp
	store.cluster.Leader.RenewTime = timestamp
	return nil
}

func (store *KubernetesStore) HasLease() bool {
	return store.cluster != nil && store.cluster.Leader != nil && store.cluster.Leader.Name == store.currentMemberName
}

func (store *KubernetesStore) UpdateLease() error {
	configMap := store.cluster.Leader.Resource.(*corev1.ConfigMap)

	annotations := configMap.GetAnnotations()
	if annotations["leader"] != store.currentMemberName {
		return errors.Errorf("lost lease")
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

func (store *KubernetesStore) ReleaseLease() error {
	store.logger.Info("release lease")
	configMap := store.cluster.Leader.Resource.(*corev1.ConfigMap)
	configMap.Annotations["leader"] = ""
	store.cluster.Leader.Name = ""

	if store.cluster.Leader.DBState != nil {
		str, _ := json.Marshal(store.cluster.Leader.DBState)
		configMap.Annotations["dbstate"] = string(str)
	}
	_, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		store.logger.Error(err, "release lease failed")
	}
	// TODO: if response status code is 409, it means operation conflict.
	return err
}

func (store *KubernetesStore) CreateHaConfig() error {
	haName := store.getHAConfigName()
	haConfig, _ := store.GetHaConfig()
	if haConfig.resource != nil {
		return nil
	}

	store.logger.Info(fmt.Sprintf("Create Ha ConfigMap: %s", haName))
	ttl := viper.GetString(constant.KBEnvTTL)
	maxLag := viper.GetString(constant.KBEnvMaxLag)
	enableHA := viper.GetString(constant.KBEnvEnableHA)
	if enableHA == "" {
		// enable HA by default
		enableHA = "true"
	}
	haConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: haName,
			Annotations: map[string]string{
				"ttl":                ttl,
				"enable":             enableHA,
				"MaxLagOnSwitchover": maxLag,
			},
		},
	}

	err := store.createConfigMap(haConfigMap)
	if err != nil {
		store.logger.Error(err, "Create Ha ConfigMap failed")
	}
	return err
}

func (store *KubernetesStore) GetHaConfig() (*HaConfig, error) {
	configmapName := store.getHAConfigName()
	deleteMembers := make(map[string]MemberToDelete)
	configmap, err := store.clientset.CoreV1().ConfigMaps(store.namespace).Get(context.TODO(), configmapName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			store.logger.Error(err, fmt.Sprintf("Get ha configmap [%s] error", configmapName))
		} else {
			err = nil
		}
		return &HaConfig{
			index:              "",
			ttl:                viper.GetInt(constant.KBEnvTTL),
			maxLagOnSwitchover: 1048576,
			DeleteMembers:      deleteMembers,
		}, err
	}

	annotations := configmap.Annotations
	ttl, err := strconv.Atoi(annotations["ttl"])
	if err != nil {
		ttl = viper.GetInt(constant.KBEnvTTL)
	}
	maxLagOnSwitchover, err := strconv.Atoi(annotations["MaxLagOnSwitchover"])
	if err != nil {
		maxLagOnSwitchover = 1048576
	}

	enable := false
	enableStr := annotations["enable"]
	if enableStr != "" {
		enable, err = strconv.ParseBool(enableStr)
	}

	str := annotations["delete-members"]
	if str != "" {
		err := json.Unmarshal([]byte(str), &deleteMembers)
		if err != nil {
			store.logger.Error(err, fmt.Sprintf("Get delete members [%s] error", str))
		}
	}

	return &HaConfig{
		index:              configmap.ResourceVersion,
		ttl:                ttl,
		enable:             enable,
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
		store.logger.Error(err, fmt.Sprintf("marsha delete members [%v]", haConfig))
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
	switchoverName := store.getSwitchoverName()
	switchover, _ := store.GetSwitchover()
	if switchover != nil {
		return fmt.Errorf("there is another switchover %s unfinished", switchoverName)
	}

	store.logger.Info(fmt.Sprintf("Create switchover configmap %s", switchoverName))
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
		store.logger.Error(err, "Create switchover configmap failed")
		return err
	}
	return nil
}

func (store *KubernetesStore) DeleteSwitchover() error {
	switchoverName := store.getSwitchoverName()
	err := store.clientset.CoreV1().ConfigMaps(store.namespace).Delete(store.ctx, switchoverName, metav1.DeleteOptions{})
	if err != nil {
		store.logger.Error(err, "Delete switchOver configmap failed")
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
		constant.AppInstanceLabelKey:    store.clusterName,
		constant.AppManagedByLabelKey:   "kubeblocks",
		constant.KBAppComponentLabelKey: store.componentName,
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

func getLorryPort(pod *corev1.Pod) string {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == constant.LorryHTTPPortName {
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
