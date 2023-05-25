package configuration_store

import (
	"bytes"
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"strconv"
	"strings"
	"time"
)

type ConfigurationStore struct {
	ctx                  context.Context
	name                 string
	Cluster              *Cluster
	config               *rest.Config
	ClientSet            *kubernetes.Clientset
	leaderObservedRecord *LeaderRecord
	LeaderObservedTime   time.Time
}

func NewConfig() *ConfigurationStore {
	ctx := context.Background()
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/buyanbujuan/.kube/config")
	if err != nil {
		panic(err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	initConfigmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Data: map[string]string{
			"primary":   "pg-1-pg-replication-0",
			"secondary": "pg-1-pg-replication-1",
		},
	}

	if resp, err := clientSet.CoreV1().ConfigMaps("default").Get(ctx, "test", metav1.GetOptions{}); err != nil || resp == nil {
		_, err = clientSet.CoreV1().ConfigMaps("default").Create(ctx, initConfigmap, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}

	c := &ConfigurationStore{
		ctx:       ctx,
		config:    config,
		ClientSet: clientSet,
		Cluster:   &Cluster{},
	}

	return c
}

func (c *ConfigurationStore) GetCluster() (*Cluster, error) {
	podList, err := c.ClientSet.CoreV1().Pods(Default).List(c.ctx, metav1.ListOptions{})
	if err != nil || podList == nil {
		return nil, err
	}
	configMapList, err := c.ClientSet.CoreV1().ConfigMaps(Default).List(c.ctx, metav1.ListOptions{})
	if err != nil || configMapList == nil {
		return nil, err
	}

	pods := make([]*v1.Pod, 0, len(podList.Items))
	for i, pod := range podList.Items {
		pods[i] = &pod
	}

	var config, leaderConfig, failoverConfig, syncConfig *v1.ConfigMap
	for _, cf := range configMapList.Items {
		switch cf.Name {
		case c.name:
			config = &cf
		case c.name + "-leader":
			leaderConfig = &cf
		case c.name + "-failover":
			failoverConfig = &cf
		case c.name + "-sync":
			syncConfig = &cf
		}
	}

	return c.loadClusterFromKubernetes(config, leaderConfig, failoverConfig, syncConfig, pods, map[string]string{}), nil
}

func (c *ConfigurationStore) loadClusterFromKubernetes(config, leaderConfig, failoverConfig, syncConfig *v1.ConfigMap, pods []*v1.Pod, extra map[string]string) *Cluster {
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

	if leaderConfig != nil {
		c.leaderObservedRecord = newLeaderRecord(leaderConfig.Annotations)
		c.LeaderObservedTime = time.Now()
		leader = newLeader(leaderConfig.ResourceVersion, newMember("-1", leaderConfig.Annotations[LeaderName], map[string]string{}))
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

func (c *ConfigurationStore) GetConfigMap(namespace string, name string) (*v1.ConfigMap, error) {
	return c.ClientSet.CoreV1().ConfigMaps(namespace).Get(c.ctx, name, metav1.GetOptions{})
}

func (c *ConfigurationStore) UpdateConfigMap(namespace string, configMap *v1.ConfigMap) (*v1.ConfigMap, error) {
	return c.ClientSet.CoreV1().ConfigMaps(namespace).Update(c.ctx, configMap, metav1.UpdateOptions{})
}

func (c *ConfigurationStore) ExecCommand(podName string, namespace string, command string) (map[string]string, error) {
	req := c.ClientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: "postgresql",
			Command:   []string{"sh", "-c", command},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	if err = exec.StreamWithContext(c.ctx, remotecommand.StreamOptions{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return nil, err
	}

	res := map[string]string{
		"stdout":   stdout.String(),
		"stderr":   stderr.String(),
		"pod_name": podName,
	}

	return res, nil
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
