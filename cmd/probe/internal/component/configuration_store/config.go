package configuration_store

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/util"
)

type ConfigurationStore struct {
	ctx                  context.Context
	clusterName          string
	clusterCompName      string
	namespace            string
	cluster              *Cluster
	config               *rest.Config
	clientSet            *kubernetes.Clientset
	leaderObservedRecord *LeaderRecord
	LeaderObservedTime   int64
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

	return &ConfigurationStore{
		ctx:             ctx,
		clusterName:     os.Getenv(util.KbClusterName),
		clusterCompName: os.Getenv(util.KbClusterCompName),
		namespace:       os.Getenv(util.KbNamespace),
		config:          config,
		clientSet:       clientSet,
		cluster:         &Cluster{},
	}
}

func (cs *ConfigurationStore) Init(sysID string, extra map[string]string, opTime int64) error {
	var createOpt metav1.CreateOptions

	leaderName := strings.Split(os.Getenv(util.KbPrimaryPodName), ".")[0]
	acquireTime := time.Now().Unix()
	renewTime := acquireTime
	ttl := os.Getenv(util.KbTtl)
	_, err := cs.clientSet.CoreV1().ConfigMaps(cs.namespace).Create(cs.ctx, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cs.clusterCompName + LeaderSuffix,
			Namespace: cs.namespace,
			Annotations: map[string]string{
				binding.LEADER: leaderName,
				AcquireTime:    strconv.FormatInt(acquireTime, 10),
				RenewTime:      strconv.FormatInt(renewTime, 10),
				TTL:            ttl,
				OpTime:         strconv.FormatInt(opTime, 10),
			},
		},
	}, createOpt)

	maxLagOnSwitchover := os.Getenv(MaxLagOnSwitchover)
	if _, err = cs.clientSet.CoreV1().ConfigMaps(cs.namespace).Create(cs.ctx, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cs.clusterCompName + ConfigSuffix,
			Namespace: cs.namespace,
			Annotations: map[string]string{
				SysID:              sysID,
				TTL:                ttl,
				MaxLagOnSwitchover: maxLagOnSwitchover,
			},
		},
	}, createOpt); err != nil {
		return err
	}

	if extra != nil {
		if _, err = cs.clientSet.CoreV1().ConfigMaps(cs.namespace).Create(cs.ctx, &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        cs.clusterCompName + ExtraSuffix,
				Namespace:   cs.namespace,
				Annotations: extra,
			},
		}, createOpt); err != nil {
			return err
		}
	}

	return nil
}

func (cs *ConfigurationStore) GetCluster() *Cluster {
	return cs.cluster
}

func (cs *ConfigurationStore) GetClusterName() string {
	return cs.clusterName
}

func (cs *ConfigurationStore) GetClusterCompName() string {
	return cs.clusterCompName
}

func (cs *ConfigurationStore) GetClusterFromKubernetes() error {
	podList, err := cs.clientSet.CoreV1().Pods(cs.namespace).List(cs.ctx, metav1.ListOptions{})
	if err != nil || podList == nil {
		return err
	}
	configMapList, err := cs.clientSet.CoreV1().ConfigMaps(cs.namespace).List(cs.ctx, metav1.ListOptions{})
	if err != nil || configMapList == nil {
		return err
	}

	pods := make([]*v1.Pod, 0, len(podList.Items))
	for i, pod := range podList.Items {
		pods[i] = &pod
	}

	var config, switchoverConfig, leaderConfig, extraConfig *v1.ConfigMap
	for _, cm := range configMapList.Items {
		switch cm.Name {
		case cs.clusterCompName + ConfigSuffix:
			config = &cm
		case cs.clusterCompName + SwitchoverSuffix:
			switchoverConfig = &cm
		case cs.clusterCompName + LeaderSuffix:
			leaderConfig = &cm
		case cs.clusterCompName + ExtraSuffix:
			extraConfig = &cm
		}
	}

	cs.cluster = cs.loadClusterFromKubernetes(config, switchoverConfig, leaderConfig, extraConfig, pods)

	return nil
}

func (cs *ConfigurationStore) loadClusterFromKubernetes(config, switchoverConfig, leaderConfig, extraConfig *v1.ConfigMap, pods []*v1.Pod) *Cluster {
	var (
		sysID         string
		clusterConfig *ClusterConfig
		leader        *Leader
		opTime        int64
		switchover    *Switchover
		extra         map[string]string
	)

	if config != nil {
		sysID = config.Annotations[SysID]
		clusterConfig = getClusterConfigFromConfigMap(config)
	}

	if leaderConfig != nil {
		leaderRecord := newLeaderRecord(leaderConfig.Annotations)
		if cs.leaderObservedRecord == nil || cs.leaderObservedRecord.renewTime != leaderRecord.renewTime {
			cs.leaderObservedRecord = leaderRecord
			cs.LeaderObservedTime = time.Now().Unix()
		}
		opTime = leaderRecord.opTime

		if cs.LeaderObservedTime+leaderRecord.ttl < time.Now().Unix() {
			leader = nil
		} else {
			leader = newLeader(leaderConfig.ResourceVersion, newMember("-1", leaderConfig.Annotations[binding.LEADER], map[string]string{}))
		}
	}

	members := make([]*Member, 0, len(pods))
	for i, pod := range pods {
		members[i] = getMemberFromPod(pod)
	}

	if switchoverConfig != nil {
		annotations := switchoverConfig.Annotations
		scheduledAt, err := strconv.Atoi(annotations[ScheduledAt])
		if err == nil {
			switchover = newSwitchover(switchoverConfig.ResourceVersion, annotations[binding.LEADER], annotations[binding.CANDIDATE], int64(scheduledAt))
		}
	}

	if extraConfig != nil {
		extra = extraConfig.Annotations
	}

	return &Cluster{
		SysID:      sysID,
		Config:     clusterConfig,
		Leader:     leader,
		OpTime:     opTime,
		Members:    members,
		Switchover: switchover,
		Extra:      extra,
	}
}

func (cs *ConfigurationStore) GetConfigMap(name string) (*v1.ConfigMap, error) {
	return cs.clientSet.CoreV1().ConfigMaps(cs.namespace).Get(cs.ctx, name, metav1.GetOptions{})
}

func (cs *ConfigurationStore) GetPod(name string) (*v1.Pod, error) {
	return cs.clientSet.CoreV1().Pods(cs.namespace).Get(cs.ctx, name, metav1.GetOptions{})
}

func (cs *ConfigurationStore) ListPods() (*v1.PodList, error) {
	return cs.clientSet.CoreV1().Pods(cs.namespace).List(cs.ctx, metav1.ListOptions{})
}

func (cs *ConfigurationStore) UpdateConfigMap(configMap *v1.ConfigMap) (*v1.ConfigMap, error) {
	return cs.clientSet.CoreV1().ConfigMaps(cs.namespace).Update(cs.ctx, configMap, metav1.UpdateOptions{})
}

func (cs *ConfigurationStore) CreateConfigMap(name string, annotations map[string]string) (*v1.ConfigMap, error) {
	configMap, err := cs.clientSet.CoreV1().ConfigMaps(cs.namespace).Create(cs.ctx, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cs.namespace,
			Annotations: annotations,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return configMap, nil
}

func (cs *ConfigurationStore) DeleteConfigMap(name string) error {
	return cs.clientSet.CoreV1().ConfigMaps(cs.namespace).Delete(cs.ctx, name, metav1.DeleteOptions{})
}

func (cs *ConfigurationStore) ExecCmdWithPod(ctx context.Context, podName, cmd, container string) (map[string]string, error) {
	req := cs.clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(cs.namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   []string{"sh", "-c", cmd},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(cs.config, "POST", req.URL())
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	if err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return nil, err
	}

	res := map[string]string{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	return res, nil
}

func (cs *ConfigurationStore) UpdateLeader(podName string, opTime int64) error {
	leaderConfigMap, err := cs.GetConfigMap(cs.clusterCompName + LeaderSuffix)
	if err != nil {
		return err
	}

	leaderRecord := leaderConfigMap.GetAnnotations()
	if leaderRecord[binding.LEADER] != podName {
		return errors.Errorf("lost lock")
	}
	leaderRecord[RenewTime] = strconv.FormatInt(time.Now().Unix(), 10)
	if opTime != 0 {
		leaderRecord[OpTime] = strconv.FormatInt(opTime, 10)
	}

	leaderConfigMap.SetAnnotations(leaderRecord)

	_, err = cs.UpdateConfigMap(leaderConfigMap)
	return err
}

type LeaderRecord struct {
	acquireTime int64
	leader      string
	renewTime   int64
	ttl         int64
	opTime      int64
}

func newLeaderRecord(data map[string]string) *LeaderRecord {
	ttl, err := strconv.Atoi(data[TTL])
	if err != nil {
		ttl = 0
	}

	acquireTime, err := strconv.Atoi(data[AcquireTime])
	if err != nil {
		acquireTime = 0
	}

	renewTime, err := strconv.Atoi(data[RenewTime])
	if err != nil {
		renewTime = 0
	}

	opTime, err := strconv.Atoi(data[OpTime])
	if err != nil {
		opTime = 0
	}

	return &LeaderRecord{
		acquireTime: int64(acquireTime),
		leader:      data[binding.LEADER],
		renewTime:   int64(renewTime),
		ttl:         int64(ttl),
		opTime:      int64(opTime),
	}
}
