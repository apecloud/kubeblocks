package ha

import (
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/config"
	"github.com/dapr/kit/logger"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"time"
)

type Ha struct {
	ctx      context.Context
	db       string
	log      logger.Logger
	Informer cache.SharedIndexInformer
	config   *config.Config
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

	return &Ha{
		ctx: context.Background(),
		//db:       os.Getenv("HOSTNAME"),
		db:       "pg-1-pg-replication-0",
		log:      logger.NewLogger("ha"),
		Informer: informer,
		config:   config.NewConfig(),
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

	if h.db == oldPrimary && h.db != newPrimary {
		_ = h.demote()
	}

	if h.db != oldPrimary && h.db == newPrimary {
		_ = h.promote()
	}
}

func (h *Ha) promote() error {
	time.Sleep(5 * time.Second)
	resp, err := h.config.ExecCommand(h.db, "default", "su -c 'pg_ctl promote' postgres")
	if err != nil {
		h.log.Errorf("promote err: %v", err)
		return err
	}
	h.log.Infof("response: ", resp)
	return nil
}

func (h *Ha) demote() error {
	resp, err := h.config.ExecCommand(h.db, "default", "su -c 'pg_ctl stop -m fast' postgres")
	if err != nil {
		h.log.Errorf("demote err: %v", err)
		return err
	}

	_, err = h.config.ExecCommand(h.db, "default", "touch /postgresql/data/standby.signal")
	if err != nil {
		h.log.Errorf("touch err: %v", err)
		return err
	}

	time.Sleep(5 * time.Second)
	_, err = h.config.ExecCommand(h.db, "default", "su -c 'postgres -D /postgresql/data --config-file=/opt/bitnami/postgresql/conf/postgresql.conf --external_pid_file=/opt/bitnami/postgresql/tmp/postgresql.pid --hba_file=/opt/bitnami/postgresql/conf/pg_hba.conf' postgres &")
	if err != nil {
		h.log.Errorf("start err: %v", err)
		return err
	}
	_, err = h.config.ExecCommand(h.db, "default", "su -c './scripts/on_role_change.sh' postgres")
	if err != nil {
		h.log.Errorf("shell err: %v", err)
		return err
	}

	h.log.Infof("response: ", resp)
	return nil
}
