package configuration_store

import (
	"context"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"os"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/apecloud/kubeblocks/cmd/probe/util"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

type ConfigurationStore struct {
	ctx                  context.Context
	clusterName          string
	clusterCompName      string
	namespace            string
	Cluster              *Cluster
	config               *rest.Config
	ClientSet            *kubernetes.Clientset
	DynamicClient        *dynamic.DynamicClient
	leaderObservedRecord *LeaderRecord
	LeaderObservedTime   time.Time
}

func NewConfigurationStore() *ConfigurationStore {
	ctx := context.Background()
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/buyanbujuan/.kube/config")
	if err != nil {
		panic(err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return &ConfigurationStore{
		ctx:             ctx,
		clusterName:     os.Getenv(util.KbClusterName),
		clusterCompName: os.Getenv(util.KbClusterCompName),
		namespace:       os.Getenv(util.KbNamespace),
		config:          config,
		ClientSet:       clientSet,
		DynamicClient:   dynamicClient,
		Cluster:         &Cluster{},
	}
}

func (cs *ConfigurationStore) Init(sysID string) {

}

func (cs *ConfigurationStore) GetCluster() (*Cluster, error) {
	podList, err := cs.ClientSet.CoreV1().Pods(Default).List(cs.ctx, metav1.ListOptions{})
	if err != nil || podList == nil {
		return nil, err
	}
	configMapList, err := cs.ClientSet.CoreV1().ConfigMaps(Default).List(cs.ctx, metav1.ListOptions{})
	if err != nil || configMapList == nil {
		return nil, err
	}
	clusterObj := &appsv1alpha1.Cluster{}
	if err = cluster.GetK8SClientObject(cs.DynamicClient, clusterObj, types.ClusterGVR(), cs.namespace, cs.clusterName); err != nil {
		return nil, err
	}

	pods := make([]*v1.Pod, 0, len(podList.Items))
	for i, pod := range podList.Items {
		pods[i] = &pod
	}

	var config, failoverConfig, syncConfig *v1.ConfigMap
	for _, cf := range configMapList.Items {
		switch cf.Name {
		case cs.clusterCompName:
			config = &cf
		case cs.clusterCompName + "-failover":
			failoverConfig = &cf
		case cs.clusterCompName + "-sync":
			syncConfig = &cf
		}
	}

	return cs.loadClusterFromKubernetes(config, failoverConfig, syncConfig, pods, clusterObj, map[string]string{}), nil
}

func (cs *ConfigurationStore) loadClusterFromKubernetes(config, failoverConfig, syncConfig *v1.ConfigMap, pods []*v1.Pod, clusterObj *appsv1alpha1.Cluster, extra map[string]string) *Cluster {
	var (
		sysID         string
		clusterConfig *ClusterConfig
		leader        *Leader
		failover      *Failover
		sync          *SyncState
	)

	if config != nil {
		sysID = config.Annotations[SysID]
		clusterConfig = getClusterConfigFromConfigMap(config)
	}

	if clusterObj != nil {
		cs.leaderObservedRecord = newLeaderRecord(clusterObj.Annotations)
		cs.LeaderObservedTime = time.Now()
		leader = newLeader(clusterObj.ResourceVersion, newMember("-1", clusterObj.Annotations[LeaderName], map[string]string{}))
	}

	members := make([]*Member, 0, len(pods))
	for i, pod := range pods {
		members[i] = getMemberFromPod(pod)
	}

	if failover != nil {
		annotations := failoverConfig.Annotations
		scheduledAt, err := time.Parse(time.RFC3339Nano, annotations[ScheduledAt])
		if err == nil {
			failover = newFailover(failoverConfig.ResourceVersion, annotations[LeaderName], annotations[Candidate], scheduledAt)
		}
	}

	if syncConfig != nil {
		annotations := syncConfig.Annotations
		sync = newSyncState(syncConfig.ResourceVersion, annotations[LeaderName], annotations[SyncStandby])
	}

	return &Cluster{
		sysID:    sysID,
		Config:   clusterConfig,
		Leader:   leader,
		Members:  members,
		FailOver: failover,
		Sync:     sync,
		Extra:    extra,
	}
}

func (cs *ConfigurationStore) GetConfigMap(namespace string, name string) (*v1.ConfigMap, error) {
	return cs.ClientSet.CoreV1().ConfigMaps(namespace).Get(cs.ctx, name, metav1.GetOptions{})
}

func (cs *ConfigurationStore) UpdateConfigMap(namespace string, configMap *v1.ConfigMap) (*v1.ConfigMap, error) {
	return cs.ClientSet.CoreV1().ConfigMaps(namespace).Update(cs.ctx, configMap, metav1.UpdateOptions{})
}

type LeaderRecord struct {
	acquireTime string
	leader      string
	renewTime   string
	transitions string
	ttl         int64
}

func newLeaderRecord(data map[string]string) *LeaderRecord {
	ttl, err := strconv.Atoi(data[TTL])
	if err != nil {
		ttl = 0
	}

	return &LeaderRecord{
		acquireTime: data[AcquireTime],
		leader:      data[LeaderName],
		renewTime:   data[RenewTime],
		transitions: data[Transitions],
		ttl:         int64(ttl),
	}
}
