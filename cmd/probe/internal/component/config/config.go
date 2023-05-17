package config

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
	"strings"
)

type Annotations struct {
	Leader string
	Member string
}

type Config struct {
	ctx       context.Context
	Cluster   *Cluster
	config    *rest.Config
	ClientSet *kubernetes.Clientset
}

func NewConfig() *Config {
	ctx := context.Background()
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/buyanbujuan/.kube/config")
	if err != nil {
		panic(err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return &Config{
		ctx:       ctx,
		config:    config,
		ClientSet: clientSet,
		Cluster:   &Cluster{},
	}
}

func (c *Config) GetCluster() {
	clusterEnv, err := c.ClientSet.CoreV1().ConfigMaps("default").Get(c.ctx, "pg-cluster-pg-replication-env", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	members := []*Member{
		{
			name: strings.Split(clusterEnv.Data["KB_PG-REPLICATION_1_HOSTNAME"], ".")[0],
		},
		{
			name: strings.Split(clusterEnv.Data["KB_PG-REPLICATION_2_HOSTNAME"], ".")[0],
		},
	}

	leader := &Leader{
		Member: &Member{
			name: strings.Split(clusterEnv.Data["KB_PRIMARY_POD_NAME"], ".")[0],
		},
	}

	c.Cluster.Members = members
	c.Cluster.Leader = leader
}

/*
func (c *Config) Init() {
	ctx := context.Background()

	configMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgre-ha-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"leader": "pg-cluster-pg-replication-0",
			"member": "pg-cluster-pg-replication-1-0",
		},
	}

	if cm, err := c.ClientSet.CoreV1().ConfigMaps("default").Get(ctx, "postgre-ha-config", metav1.GetOptions{}); err != nil || cm != nil {
		log.Infof("configmap already exist")
	} else {
		_, err = c.ClientSet.CoreV1().ConfigMaps("default").Create(context.Background(), &configMap, metav1.CreateOptions{})
		if err != nil {
			log.Fatal("err:%v", err)
			return
		}
	}
}
*/

func (c *Config) GetConfigMap(namespace string, name string) (*v1.ConfigMap, error) {
	return c.ClientSet.CoreV1().ConfigMaps(namespace).Get(c.ctx, name, metav1.GetOptions{})
}

func (c *Config) UpdateConfigMap(namespace string, configMap *v1.ConfigMap) (*v1.ConfigMap, error) {
	return c.ClientSet.CoreV1().ConfigMaps(namespace).Update(c.ctx, configMap, metav1.UpdateOptions{})
}

func (c *Config) ExecCommand(podName string, namespace string, command string) (map[string]string, error) {
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
