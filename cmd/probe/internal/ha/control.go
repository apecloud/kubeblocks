package ha

import (
	"github.com/apecloud/kubeblocks/cmd/probe/util"
	"os"
	"time"

	"github.com/dapr/kit/logger"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/mysql"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/postgres"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/configuration_store"
)

type Ha struct {
	ctx             context.Context
	podName         string
	dbType          string
	replicationMode string
	log             logger.Logger
	Informer        cache.SharedIndexInformer
	cs              *configuration_store.ConfigurationStore
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
	informer := sharedInformers.Core().V1().ConfigMaps().Informer()

	cs := configuration_store.NewConfigurationStore()

	ha := &Ha{
		ctx:      context.Background(),
		podName:  os.Getenv(util.HostName),
		dbType:   os.Getenv(util.KbServiceCharacterType),
		log:      logger.NewLogger("ha"),
		Informer: informer,
		cs:       cs,
	}

	ha.DB = ha.newDbInterface(ha.log)
	if ha.DB == nil {
		panic("unknown db type")
	}

	return ha
}

func (h *Ha) Init() {
	sysid, err := h.DB.GetSysID(h.ctx)
	if err != nil {
		h.log.Errorf("can't get sysID, err:%v", err)
	}

	dbState, err := h.DB.GetState(h.ctx)

	err = h.cs.Init(sysid, h.podName, dbState, h.replicationMode, h.newDbExtra())
	if err != nil {
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

func (h *Ha) newDbExtra() map[string]string {
	switch h.dbType {
	case binding.Postgresql:
		return h.DB.GetExtra(h.ctx)
	default:
		h.log.Fatalf("unknown db type:%s", h.dbType)
		return nil
	}
}

func (h *Ha) HaControl(stopCh chan struct{}) {
	h.Informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: h.UpdateRecall,
	})

	h.Informer.Run(stopCh)
}

func (h *Ha) UpdateRecall(oldObj, newObj interface{}) {
	newConfigMap := newObj.(*corev1.ConfigMap)
	oldConfigMap := oldObj.(*corev1.ConfigMap)
	if oldConfigMap.Name != "test" {
		return
	}

	oldPrimary := oldConfigMap.Data["primary"]
	newPrimary := newConfigMap.Data["primary"]
	if oldPrimary == newPrimary {
		return
	}

	if h.podName == oldPrimary && h.podName != newPrimary {
		_ = h.DB.Demote()
	}

	if h.podName != oldPrimary && h.podName == newPrimary {
		_ = h.DB.Promote()
	}
}

/*
func (h *Ha) promote() error {
	time.Sleep(5 * time.Second)
	resp, err := h.cs.ExecCommand(h.db.name, "default", "su -c 'pg_ctl promote' postgres")
	if err != nil {
		h.log.Errorf("promote err: %v", err)
		return err
	}
	h.log.Infof("response: ", resp)
	return nil
}

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
