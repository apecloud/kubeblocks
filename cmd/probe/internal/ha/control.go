package ha

import (
	"os"
	"time"

	"cuelang.org/go/pkg/strconv"
	"github.com/avast/retry-go"
	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/kit/logger"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
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
	//TODO:锁
	dbType   string
	log      logger.Logger
	informer cache.SharedIndexInformer
	cs       *configuration_store.ConfigurationStore
	DB
}

func NewHa() *Ha {
	configs, err := restclient.InClusterConfig()
	if err != nil {
		configs, err = clientcmd.BuildConfigFromFlags("", "/Users/buyanbujuan/.kube/config")
		if err != nil {
			panic(err)
		}
	}

	clientSet, err := kubernetes.NewForConfig(configs)
	if err != nil {
		panic(err)
	}

	cs := configuration_store.NewConfigurationStore()

	sharedInformers := informers.NewSharedInformerFactoryWithOptions(clientSet, 10*time.Second, informers.WithNamespace(cs.GetNamespace()))
	configMapInformer := sharedInformers.Core().V1().ConfigMaps().Informer()

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

	props := map[string]map[string]string{
		binding.Postgresql: {
			"url": "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=10",
		},
		binding.Mysql: {
			"url":          "root:@tcp(127.0.0.1:3306)/mysql?multiStatements=true",
			"maxOpenConns": "5",
		},
	}
	err = ha.DB.Init(bindings.Metadata{
		Base: metadata.Base{Properties: props[ha.dbType]},
	})
	if err != nil {
		panic(err)
	}

	err = retry.Do(
		ha.DB.InitDelay,
		retry.Attempts(3),
		retry.Delay(time.Second*2),
		retry.DelayType(retry.BackOffDelay),
	)
	if err != nil {
		panic(err)
	}

	return ha
}

func (h *Ha) Init() {
	var (
		isLeader bool
		sysid    string
		opTime   int64
		extra    map[string]string
		err      error
	)

	err = retry.Do(
		func() error {
			isLeader, err = h.DB.IsLeader(h.ctx)
			return err
		},
		retry.Attempts(3),
		retry.Delay(time.Second*2),
		retry.DelayType(retry.BackOffDelay),
	)
	if err != nil {
		panic(err)
	}

	if !isLeader {
		err = h.cs.Init(false, "", nil, 0, h.podName)
		if err != nil {
			panic(err)
		}
		return
	}

	sysid, err = h.DB.GetSysID(h.ctx)
	if err != nil {
		h.log.Errorf("can not get sysID, err:%v", err)
		panic(err)
	}

	extra, err = h.DB.GetExtra(h.ctx)
	if err != nil {
		h.log.Errorf("can not get extra, err:%v", err)
		panic(err)
	}

	opTime, err = h.DB.GetOpTime(h.ctx)
	if err != nil {
		h.log.Errorf("can not get op time, err:%v", err)
		panic(err)
	}

	err = h.cs.Init(isLeader, sysid, extra, opTime, h.podName)
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
	h.informer.AddEventHandlerWithResyncPeriod(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			configMap := obj.(*v1.ConfigMap)
			return configMap.Name == h.cs.GetClusterCompName()+configuration_store.LeaderSuffix
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				configMap := obj.(*v1.ConfigMap)
				configMap.Annotations[configuration_store.RenewTime] = strconv.FormatInt(time.Now().Unix()+1, 10)
				_, _ = h.cs.UpdateConfigMap(configMap)
			},
			UpdateFunc: h.clusterControl,
		},
	}, time.Second*10)

	h.informer.Run(stopCh)
}

func (h *Ha) clusterControl(oldObj, newObj interface{}) {
	oldConfigMap := oldObj.(*v1.ConfigMap)
	newConfigMap := newObj.(*v1.ConfigMap)
	if oldConfigMap.ResourceVersion != newConfigMap.ResourceVersion {
		return
	}

	err := h.cs.GetClusterFromKubernetes()
	if err != nil {
		h.log.Errorf("cluster control get cluster from k8s err:%v", err)
		return
	}
	err = h.DB.RefreshCluster()
	if err != nil {
		h.log.Errorf("Refresh cluster failed, err:%v", err)
		return
	}

	if !h.DB.IsRunning(h.ctx, h.podName) {
		h.log.Warnf("in control loop, db is not running now")
		err = h.DB.Start(h.ctx, h.podName)
		if err != nil {
			h.log.Errorf("db start failed, err:%v", err)
		}
		return
	}

	if !h.cs.GetCluster().HasMember(h.podName) {
		h.touchMember()
	}

	if h.cs.GetCluster().IsLocked() {
		h.processClusterWithLeader()
	} else {
		h.processClusterWithoutLeader()
	}
}

func (h *Ha) hasLock() bool {
	return h.podName == h.cs.GetCluster().Leader.GetMember().GetName()
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
	if !h.DB.IsRunning(h.ctx, h.podName) {
		return false
	}

	if isLeader, err := h.DB.IsLeader(h.ctx); isLeader && err == nil {
		dbSysId, err := h.DB.GetSysID(h.ctx)
		if err != nil {
			h.log.Errorf("get db sysid err:%v", err)
			return false
		}
		return dbSysId == h.cs.GetCluster().SysID
	}

	if h.cs.GetCluster().Switchover != nil {
		return h.DB.ProcessManualSwitchoverFromNoLeader(h.ctx, h.podName)
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

func (h *Ha) follow() error {
	// refresh cluster
	err := h.cs.GetClusterFromKubernetes()
	if err != nil {
		h.log.Errorf("get cluster from k8s failed, err:%v")
		return err
	}
	err = h.DB.RefreshCluster()
	if err != nil {
		h.log.Errorf("Refresh cluster failed, err:%v", err)
		return err
	}

	if isLeader, err := h.DB.IsLeader(h.ctx); isLeader && err == nil {
		h.log.Infof("demoted %s after trying and failing to obtain lock", h.podName)
		return h.DB.Demote(h.ctx, h.podName)
	}

	if h.cs.GetCluster().Leader == nil {
		h.log.Warnf("no action coz cluster still has no leader")
		return nil
	}

	return h.DB.HandleFollow(h.ctx, h.cs.GetCluster().Leader, h.podName)
}

func (h *Ha) processClusterWithLeader() {
	if !h.hasLock() {
		h.setLeader(false)
		err := h.follow()
		if err != nil {
			h.log.Errorf("failed to follow with leader but no lock, err%v", err)
		}
		return
	}

	var opTime int64
	var extra map[string]string
	err := retry.Do(
		func() error {
			opTime, _ = h.DB.GetOpTime(h.ctx)
			extra, _ = h.DB.GetExtra(h.ctx)

			return h.cs.UpdateLeader(h.podName, opTime, extra)
		},
		retry.Attempts(3),
		retry.Delay(time.Second*2),
		retry.DelayType(retry.BackOffDelay),
	)
	if err != nil {
		h.log.Warnf("update lock err,")
		if isLeader, err := h.DB.IsLeader(h.ctx); isLeader && err == nil {
			_ = h.DB.Demote(h.ctx, h.podName)
		}
	}

	done, err := h.DB.ProcessManualSwitchoverFromLeader(h.ctx, h.podName)
	if err != nil {
		h.log.Errorf("process manual switchover failed, err:%v", err)
	}
	if done {
		return
	}

	err = h.DB.EnforcePrimaryRole(h.ctx, h.podName)
	return
}

func (h *Ha) processClusterWithoutLeader() {
	h.log.Warnf("cluster has no leader now")

	if h.isHealthiest() {
		err := h.acquireLeaderLock()
		if err != nil {
			h.log.Errorf("acquire leader lock err:%v", err)
			_ = h.follow()
		}

		if h.cs.GetCluster().Switchover != nil {
			err = h.cs.DeleteConfigMap(h.cs.GetClusterCompName() + configuration_store.SwitchoverSuffix)
			if err != nil {
				return
			}
		}

		err = h.DB.EnforcePrimaryRole(h.ctx, h.podName)
	} else {
		// Give a time to somebody to take the leader lock
		time.Sleep(time.Second * 2)
		_ = h.follow()
	}
}
