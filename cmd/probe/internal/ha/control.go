package ha

import (
	"os"
	"time"

	"github.com/dapr/kit/logger"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/mysql"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/postgres"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/configuration_store"
	"github.com/apecloud/kubeblocks/cmd/probe/util"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

type Ha struct {
	ctx               context.Context
	podName           string
	dbType            string
	log               logger.Logger
	configMapInformer cache.SharedIndexInformer
	clusterInformer   cache.SharedInformer
	cs                *configuration_store.ConfigurationStore
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
	dynamicClient, err := dynamic.NewForConfig(configs)
	if err != nil {
		panic(err)
	}

	sharedInformers := informers.NewSharedInformerFactory(clientSet, 10*time.Second)
	configMapInformer := sharedInformers.Core().V1().ConfigMaps().Informer()

	resourceInterface := dynamicClient.Resource(types.ClusterGVR())
	listWatch := cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return resourceInterface.List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return resourceInterface.Watch(context.Background(), options)
		},
		DisableChunking: false,
	}
	obj := unstructured.Unstructured{}
	clusterInformer := cache.NewSharedInformer(&listWatch, &obj, 10*time.Second)

	cs := configuration_store.NewConfigurationStore()

	ha := &Ha{
		ctx:               context.Background(),
		podName:           os.Getenv(util.HostName),
		dbType:            os.Getenv(util.KbServiceCharacterType),
		log:               logger.NewLogger("ha"),
		configMapInformer: configMapInformer,
		clusterInformer:   clusterInformer,
		cs:                cs,
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
		h.log.Errorf("can not get sysID, err:%v", err)
		panic(err)
	}

	err = h.cs.Init(sysid, h.newDbExtra())
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
	h.configMapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: h.UpdateRecall,
	})

	h.clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: h.clusterUpdateRecall,
	})

	h.configMapInformer.Run(stopCh)
	h.clusterInformer.Run(stopCh)
}

func (h *Ha) clusterUpdateRecall(oldObj, newObj interface{}) {
	oldCluster := oldObj.(*unstructured.Unstructured)
	newCluster := newObj.(*unstructured.Unstructured)
	if oldCluster.GetName() != h.cs.GetClusterName() {
		return
	}

	oldLeaderName := oldCluster.GetAnnotations()[binding.LEADER]
	newLeaderName := newCluster.GetAnnotations()[binding.LEADER]
	if oldLeaderName == newLeaderName {

		return
	}

	if oldLeaderName == h.podName && newLeaderName != h.podName {
		err := h.DB.Demote(h.podName)
		if err != nil {
			h.log.Errorf("demote failed, err:%v", err)
		}
	}
	if oldLeaderName != h.podName && newLeaderName == h.podName {
		err := h.DB.Promote(h.podName)
		if err != nil {
			h.log.Errorf("promote failed, err:%v", err)
		}
	}

}

func (h *Ha) UpdateRecall(oldObj, newObj interface{}) {
	return
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
