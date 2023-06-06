package ha

import (
	"os"
	"reflect"
	"time"

	"github.com/dapr/kit/logger"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/mysql"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/postgres"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/configuration_store"
	"github.com/apecloud/kubeblocks/cmd/probe/util"
)

type Ha struct {
	ctx      context.Context
	podName  string
	isLeader int64
	//TODO:可重入锁
	dbType   string
	log      logger.Logger
	informer cache.SharedIndexInformer
	cs       *configuration_store.ConfigurationStore
	DB
}

func NewHa() *Ha {
	configs, err := clientcmd.BuildConfigFromFlags("", "/Users/buyanbujuan/.kube/config")
	if err != nil {
		panic(err)
	}

	clientSet, err := kubernetes.NewForConfig(configs)
	if err != nil {
		panic(err)
	}

	sharedInformers := informers.NewSharedInformerFactory(clientSet, 10*time.Second)
	configMapInformer := sharedInformers.Core().V1().ConfigMaps().Informer()

	cs := configuration_store.NewConfigurationStore()

	ha := &Ha{
		ctx:      context.Background(),
		podName:  os.Getenv(util.HostName),
		isLeader: int64(0),
		dbType:   os.Getenv(util.KbServiceCharacterType),
		log:      logger.NewLogger("ha"),
		informer: configMapInformer,
		cs:       cs,
	}

	ha.DB = ha.newDbInterface(ha.log)
	if ha.DB == nil {
		panic("unknown db type")
	}

	return ha
}

func (h *Ha) Init() {
	if !h.DB.IsLeader(h.ctx) {
		return
	}

	sysid, err := h.DB.GetSysID(h.ctx)
	if err != nil {
		h.log.Errorf("can not get sysID, err:%v", err)
		panic(err)
	}

	extra, err := h.DB.GetExtra(h.ctx)
	if err != nil {
		h.log.Errorf("can not get extra, err:%v", err)
		panic(err)
	}

	opTime, err := h.DB.GetOpTime(h.ctx)
	if err != nil {
		h.log.Errorf("can not get op time, err:%v", err)
		panic(err)
	}

	err = h.cs.Init(sysid, extra, opTime)
	if err != nil {
		h.log.Errorf("configuration store init err:%v", err)
		panic(err)
	}
}

func (h *Ha) newDbInterface(logger logger.Logger) DB {
	switch h.dbType {
	case binding.Postgresql:
		return postgres.NewPostgres(logger).(DB)
	case binding.Mysql:
		return mysql.NewMysql(logger).(DB)
	default:
		h.log.Fatalf("unknown db type:%s", h.dbType)
		return nil
	}
}

func (h *Ha) HaControl(stopCh chan struct{}) {
	h.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    h.processSwitchover,
		UpdateFunc: h.clusterControl,
	})

	h.informer.Run(stopCh)
}

// clusterControl 主循环
func (h *Ha) clusterControl(oldObj, newObj interface{}) {
	oldConfigMap := oldObj.(*v1.ConfigMap)
	newConfigMap := newObj.(*v1.ConfigMap)
	if oldConfigMap.GetName() != h.cs.GetClusterCompName()+configuration_store.LeaderSuffix {
		return
	}
	if reflect.DeepEqual(oldConfigMap.Annotations, newConfigMap.Annotations) {
		return
	}

	err := h.cs.GetClusterFromKubernetes()
	if err != nil {
		h.log.Errorf("cluster control get cluster from k8s err:%v", err)
		return
	}

	if !h.cs.GetCluster().HasMember(h.podName) {
		h.touchMember()
	}

	if h.cs.GetCluster().IsLocked() {
		if !h.hasLock() {
			h.setLeader(false)
			return
		}
		err = h.updateLockWithRetry(3)
		if err != nil {
			h.log.Errorf("update lock err,")
		}
		return
	}

	// Process no leader cluster
	if h.isHealthiest() {
		err = h.acquireLeaderLock()
		if err != nil {
			h.log.Errorf("acquire leader lock err:%v", err)
			h.follow()
		}
		err = h.DB.EnforcePrimaryRole(h.ctx, h.podName)
	} else {
		// Give a time to somebody to take the leader lock
		time.Sleep(time.Second * 2)
		h.follow()
	}
}

// Only in processSwitchover, leader can unlock actively， 处理降主，不处理升
func (h *Ha) processSwitchover(obj interface{}) {
	configMap := obj.(*v1.ConfigMap)

	if configMap.Name != h.cs.GetClusterCompName()+configuration_store.SwitchoverSuffix {
		return
	}

	err := h.cs.GetClusterFromKubernetes()
	if err != nil {
		h.log.Errorf("process switchover get cluster from k8s err:%v", err)
		return
	}
	if !h.hasLock() {
		h.log.Infof("db:%s does not have lock", h.podName)
		return
	}

	err = h.updateLockWithRetry(3)
	if err != nil {
		h.log.Errorf("failed to update leader lock")
		if h.DB.IsLeader(h.ctx) {
			_ = h.DB.Demote(h.ctx, h.podName)
		}
	}
}

func (h *Ha) hasLock() bool {
	return h.podName == h.cs.GetCluster().Leader.GetMember().GetName()
}

func (h *Ha) updateLockWithRetry(retryTimes int) error {
	opTime, err := h.DB.GetOpTime(h.ctx)
	if err != nil {
		opTime = 0
	}
	extra, err := h.DB.GetExtra(h.ctx)
	if err != nil {
		extra = map[string]string{}
	}
	for i := 0; i < retryTimes; i++ {
		err = h.cs.UpdateLeader(h.podName, opTime, extra)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second * 10)
	}

	return err
}

func (h *Ha) setLeader(shouldSet bool) {
	if shouldSet {
		h.isLeader = time.Now().Unix() + h.cs.GetCluster().Config.GetData().GetTtl()
	} else {
		h.isLeader = 0
	}
}

// TODO:finish touchMember
func (h *Ha) touchMember() {}

func (h *Ha) isHealthiest() bool {
	if !h.isDBRunning() {
		return false
	}

	if h.DB.IsLeader(h.ctx) {
		dbSysId, err := h.DB.GetSysID(h.ctx)
		if err != nil {
			h.log.Errorf("get db sysid err:%v", err)
			return false
		}
		if dbSysId != h.cs.GetCluster().SysID {
			return false
		}
	}

	return h.DB.IsHealthiest(h.ctx, h.podName)
}

func (h *Ha) acquireLeaderLock() error {
	err := h.cs.AttemptToAcquireLeaderLock(h.podName)
	if err == nil {
		h.setLeader(true)
	}
	return err
}

func (h *Ha) isDBRunning() bool {
	status, err := h.DB.GetStatus(h.ctx)
	if err != nil {
		h.log.Errorf("get db status failed, err:%v", err)
		return false
	}

	return status == util.Running
}

func (h *Ha) follow() {
	// refresh cluster
	err := h.cs.GetClusterFromKubernetes()
	if err != nil {
		h.log.Errorf("get cluster from k8s failed, err:%v")
		return
	}

	if h.DB.IsLeader(h.ctx) {
		h.log.Infof("demoted %s after trying and failing to obtain lock", h.podName)
		err = h.DB.Demote(h.ctx, h.podName)
	}

	_ = h.DB.HandleFollow(h.ctx, h.cs.GetCluster().Leader, h.podName)
}

/*
func (h *Ha) demote() error {
	resp, err := h.cs.ExecCommand(h.db.name, "default", "su -c 'pg_ctl stop -m fast' postgres")
	if err != nil {
		h.log.Errorf("demote err: %v", err)
		return err
	}

	_, err = h.cs.ExecCommand(h.db.name, "default", "touch /postgresql/data/standby.signal")
	if err != nil {
		h.log.Errorf("touch err: %v", err)
		return err
	}

	time.Sleep(5 * time.Second)
	_, err = h.cs.ExecCommand(h.db.name, "default", "su -c 'postgres -D /postgresql/data --config-file=/opt/bitnami/postgresql/conf/postgresql.conf --external_pid_file=/opt/bitnami/postgresql/tmp/postgresql.pid --hba_file=/opt/bitnami/postgresql/conf/pg_hba.conf' postgres &")
	if err != nil {
		h.log.Errorf("start err: %v", err)
		return err
	}
	_, err = h.cs.ExecCommand(h.db.name, "default", "su -c './scripts/on_role_change.sh' postgres")
	if err != nil {
		h.log.Errorf("shell err: %v", err)
		return err
	}

	h.log.Infof("response: ", resp)
	return nil
}*/
