package controllerutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// // getKubeConfig creates kuberest.Config object based on current environment
// func getKubeConfig(kubeConfigFile, masterURL string) (*rest.Config, error) {
// 	if len(kubeConfigFile) > 0 {
// 		// kube config file specified as CLI flag
// 		return clientcmd.BuildConfigFromFlags(masterURL, kubeConfigFile)
// 	}
// 	if len(os.Getenv("KUBECONFIG")) > 0 {
// 		// kube config file specified as ENV var
// 		return clientcmd.BuildConfigFromFlags(masterURL, os.Getenv("KUBECONFIG"))
// 	}
// 	if conf, err := rest.InClusterConfig(); err == nil {
// 		// in-cluster configuration found
// 		return conf, nil
// 	}
// 	usr, err := user.Current()
// 	if err != nil {
// 		return nil, fmt.Errorf("user not found")
// 	}
// 	// OS user found. Parse ~/.kube/config file
// 	conf, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config"))
// 	if err != nil {
// 		return nil, fmt.Errorf("~/.kube/config not found")
// 	}
// 	// ~/.kube/config found
// 	return conf, nil
// }

// // GetClientset gets k8s API clients - both kube native client and our custom client
// func GetClientset(kubeConfigFile string, masterURL string) *kubernetes.Clientset {
// 	kubeConfig, err := getKubeConfig(kubeConfigFile, masterURL)
// 	if err != nil {
// 		os.Exit(1)
// 	}

// 	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
// 	if err != nil {
// 		os.Exit(1)
// 	}
// 	return kubeClientset
// }

// type Controller struct {
// 	client *kubernetes.Clientset
// }

// func NewController() (*Controller, error) {
// 	c := Controller{}
// 	c.client = GetClientset("", "")
// 	return &c, nil
// }

// func (c *Controller) GetCR(apiVersion string, kind string, namespace string, name string, result any) error {
// 	data, err := c.client.CoreV1().RESTClient().
// 		Get().
// 		AbsPath("/apis/" + apiVersion).
// 		Namespace(namespace).
// 		Resource(kind + "s").
// 		Name(name).
// 		DoRaw(context.TODO())
// 	if err != nil {
// 		return err
// 	}
// 	return json.Unmarshal(data, result)
// }

// func (c *Controller) CreateStatefulSet(statefulset *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
// 	return c.client.AppsV1().StatefulSets(statefulset.Namespace).Create(context.TODO(), statefulset, metav1.CreateOptions{})
// }

// func (c *Controller) GetStatefulSet(statefulset *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
// 	return c.client.AppsV1().StatefulSets(statefulset.Namespace).Get(context.TODO(), statefulset.Name, metav1.GetOptions{})
// }

// APIStatus is exposed by errors that can be converted to an api.Status object
// for finer grained details.
type APIStatus interface {
	Status() metav1.Status
}

// ReasonForError returns the HTTP status for a particular error.
func ReasonForError(err error) metav1.StatusReason {
	switch t := err.(type) {
	case APIStatus:
		return t.Status().Reason
	}
	return metav1.StatusReasonUnknown
}
