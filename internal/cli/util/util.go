/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	cmdget "k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

var (
	green = color.New(color.FgHiGreen, color.Bold).SprintFunc()
	red   = color.New(color.FgHiRed, color.Bold).SprintFunc()
)

func init() {
	if _, err := GetCliHomeDir(); err != nil {
		fmt.Println("Failed to create kbcli home dir:", err)
	}
}

// CloseQuietly closes `io.Closer` quietly. Very handy and helpful for code
// quality too.
func CloseQuietly(d io.Closer) {
	_ = d.Close()
}

// GetCliHomeDir return kbcli home dir
func GetCliHomeDir() (string, error) {
	var cliHome string
	if custom := os.Getenv(types.CliHomeEnv); custom != "" {
		cliHome = custom
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cliHome = filepath.Join(home, types.CliDefaultHome)
	}
	if _, err := os.Stat(cliHome); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(cliHome, 0750); err != nil {
			return "", errors.Wrap(err, "error when create kbcli home directory")
		}
	}
	return cliHome, nil
}

// GetKubeconfigDir returns the kubeconfig directory.
func GetKubeconfigDir() string {
	var kubeconfigDir string
	switch runtime.GOOS {
	case types.GoosDarwin, types.GoosLinux:
		kubeconfigDir = filepath.Join(os.Getenv("HOME"), ".kube")
	case types.GoosWindows:
		kubeconfigDir = filepath.Join(os.Getenv("USERPROFILE"), ".kube")
	}
	return kubeconfigDir
}

func ConfigPath(name string) string {
	if len(name) == 0 {
		return ""
	}

	return filepath.Join(GetKubeconfigDir(), name)
}

func RemoveConfig(name string) error {
	if err := os.Remove(ConfigPath(name)); err != nil {
		return err
	}
	return nil
}

func GetPublicIP() (string, error) {
	resp, err := http.Get("https://ifconfig.me")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// MakeSSHKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
func MakeSSHKeyPair(pubKeyPath, privateKeyPath string) error {
	if err := os.MkdirAll(path.Dir(pubKeyPath), os.FileMode(0700)); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Dir(privateKeyPath), os.FileMode(0700)); err != nil {
		return err
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	// generate and write private key as PEM
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer privateKeyFile.Close()

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	return os.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(pub), 0655)
}

func PrintObjYAML(obj *unstructured.Unstructured) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

type RetryOptions struct {
	MaxRetry int
	Delay    time.Duration
}

func DoWithRetry(ctx context.Context, logger logr.Logger, operation func() error, options *RetryOptions) error {
	err := operation()
	for attempt := 0; err != nil && attempt < options.MaxRetry; attempt++ {
		delay := time.Duration(int(math.Pow(2, float64(attempt)))) * time.Second
		if options.Delay != 0 {
			delay = options.Delay
		}
		logger.Info(fmt.Sprintf("Failed, retrying in %s ... (%d/%d). Error: %v", delay, attempt+1, options.MaxRetry, err))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return err
		}
		err = operation()
	}
	return err
}

func GenRequestID() string {
	return uuid.New().String()
}

func PrintGoTemplate(wr io.Writer, tpl string, values interface{}) error {
	tmpl, err := template.New("output").Parse(tpl)
	if err != nil {
		return err
	}

	err = tmpl.Execute(wr, values)
	if err != nil {
		return err
	}
	return nil
}

// SetKubeConfig set KUBECONFIG environment
func SetKubeConfig(cfg string) error {
	return os.Setenv("KUBECONFIG", cfg)
}

func Spinner(w io.Writer, fmtstr string, a ...any) func(result bool) {
	msg := fmt.Sprintf(fmtstr, a...)
	var once sync.Once
	var s *spinner.Spinner

	if runtime.GOOS == types.GoosWindows {
		fmt.Fprintf(w, "%s\n", msg)
		return func(result bool) {}
	} else {
		s = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Writer = w
		_ = s.Color("cyan")
		s.Suffix = fmt.Sprintf("  %s", msg)
		s.Start()
	}

	return func(result bool) {
		once.Do(func() {
			if s != nil {
				s.Stop()
			}
			if result {
				fmt.Fprintf(w, "%s %s\n", msg, green("OK"))
			} else {
				fmt.Fprintf(w, "%s %s\n", msg, red("FAIL"))
			}
		})
	}
}

var addToScheme sync.Once

func NewFactory() cmdutil.Factory {
	getter := genericclioptions.NewConfigFlags(true)

	// Add CRDs to the scheme. They are missing by default.
	addToScheme.Do(func() {
		if err := apiextv1.AddToScheme(scheme.Scheme); err != nil {
			// This should never happen.
			panic(err)
		}
	})
	return cmdutil.NewFactory(getter)
}

func PlaygroundDir() (string, error) {
	cliPath, err := GetCliHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(cliPath, "playground"), nil
}

func GVRToString(gvr schema.GroupVersionResource) string {
	return strings.Join([]string{gvr.Resource, gvr.Version, gvr.Group}, ".")
}

// GetNodeByName choose node by name from a node array
func GetNodeByName(nodes []*corev1.Node, name string) *corev1.Node {
	for _, node := range nodes {
		if node.Name == name {
			return node
		}
	}
	return &corev1.Node{}
}

// ResourceIsEmpty check if resource is empty or not
func ResourceIsEmpty(res *resource.Quantity) bool {
	resStr := res.String()
	if resStr == "0" || resStr == "<nil>" {
		return true
	}
	return false
}

func GetPodStatus(pods []*corev1.Pod) (running, waiting, succeeded, failed int) {
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			running++
		case corev1.PodPending:
			waiting++
		case corev1.PodSucceeded:
			succeeded++
		case corev1.PodFailed:
			failed++
		}
	}
	return
}

// OpenBrowser will open browser by url in different OS system
func OpenBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("cmd", "/C", "start", url).Run()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

func TimeFormat(t *metav1.Time) string {
	const layout = "Jan 02,2006 15:04 UTC-0700"

	if t == nil || t.IsZero() {
		return ""
	}

	return t.Format(layout)
}

// GetHumanReadableDuration returns a succinct representation of the provided startTime and endTime
// with limited precision for consumption by humans.
func GetHumanReadableDuration(startTime metav1.Time, endTime metav1.Time) string {
	if startTime.IsZero() {
		return "<Unknown>"
	}
	if endTime.IsZero() {
		endTime = metav1.NewTime(time.Now())
	}
	d := endTime.Sub(startTime.Time)
	// if the
	if d < time.Second {
		d = time.Second
	}
	return duration.HumanDuration(d)
}

// CheckEmpty check if string is empty, if yes, return <none> for displaying
func CheckEmpty(str string) string {
	if len(str) == 0 {
		return types.None
	}
	return str
}

// BuildLabelSelectorByNames build the label selector by instance names, the label selector is
// like "instance-key in (name1, name2)"
func BuildLabelSelectorByNames(selector string, names []string) string {
	if len(names) == 0 {
		return ""
	}

	label := fmt.Sprintf("%s in (%s)", types.InstanceLabelKey, strings.Join(names, ","))
	if len(selector) == 0 {
		return label
	} else {
		return selector + "," + label
	}
}

// SortEventsByLastTimestamp sorts events by lastTimestamp
func SortEventsByLastTimestamp(events *corev1.EventList, eventType string) *[]apiruntime.Object {
	objs := make([]apiruntime.Object, 0, len(events.Items))
	for i, e := range events.Items {
		if eventType != "" && e.Type != eventType {
			continue
		}
		objs = append(objs, &events.Items[i])
	}
	sorter := cmdget.NewRuntimeSort("{.lastTimestamp}", objs)
	sort.Sort(sorter)
	return &objs
}

func GetEventTimeStr(e *corev1.Event) string {
	t := &e.CreationTimestamp
	if !e.LastTimestamp.Time.IsZero() {
		t = &e.LastTimestamp
	}
	return TimeFormat(t)
}

func GetEventObject(e *corev1.Event) string {
	kind := e.InvolvedObject.Kind
	if kind == "Pod" {
		kind = "Instance"
	}
	return fmt.Sprintf("%s/%s", kind, e.InvolvedObject.Name)
}

// GetConfigTemplateList returns ConfigTemplate list used by the component.
func GetConfigTemplateList(clusterName string, namespace string, cli dynamic.Interface, componentName string, reloadTpl bool) ([]appsv1alpha1.ConfigTemplate, error) {
	var (
		clusterObj        = appsv1alpha1.Cluster{}
		clusterDefObj     = appsv1alpha1.ClusterDefinition{}
		clusterVersionObj = appsv1alpha1.ClusterVersion{}
	)

	if err := GetResourceObjectFromGVR(types.ClusterGVR(), client.ObjectKey{
		Namespace: namespace,
		Name:      clusterName,
	}, cli, &clusterObj); err != nil {
		return nil, err
	}

	clusterDefName := clusterObj.Spec.ClusterDefRef
	clusterVersionName := clusterObj.Spec.ClusterVersionRef
	if err := GetResourceObjectFromGVR(types.ClusterDefGVR(), client.ObjectKey{
		Namespace: "",
		Name:      clusterDefName,
	}, cli, &clusterDefObj); err != nil {
		return nil, err
	}
	if err := GetResourceObjectFromGVR(types.ClusterVersionGVR(), client.ObjectKey{
		Namespace: "",
		Name:      clusterVersionName,
	}, cli, &clusterVersionObj); err != nil {
		return nil, err
	}

	tpls, err := cfgcore.GetConfigTemplatesFromComponent(clusterObj.Spec.ComponentSpecs, clusterDefObj.Spec.ComponentDefs, clusterVersionObj.Spec.ComponentVersions, componentName)
	if err != nil {
		return nil, err
	} else if !reloadTpl {
		return tpls, nil
	}

	validTpls := make([]appsv1alpha1.ConfigTemplate, 0, len(tpls))
	for _, tpl := range tpls {
		if len(tpl.ConfigConstraintRef) > 0 && len(tpl.ConfigTplRef) > 0 {
			validTpls = append(validTpls, tpl)
		}
	}
	return validTpls, nil
}

// GetResourceObjectFromGVR query the resource object using GVR.
func GetResourceObjectFromGVR(gvr schema.GroupVersionResource, key client.ObjectKey, client dynamic.Interface, k8sObj interface{}) error {
	unstructuredObj, err := client.
		Resource(gvr).
		Namespace(key.Namespace).
		Get(context.TODO(), key.Name, metav1.GetOptions{})
	if err != nil {
		return cfgcore.WrapError(err, "failed to get resource[%v]", key)
	}
	return apiruntime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, k8sObj)
}

// GetComponentsFromClusterCR returns name of component.
func GetComponentsFromClusterCR(key client.ObjectKey, cli dynamic.Interface) ([]string, error) {
	clusterObj := appsv1alpha1.Cluster{}
	clusterDefObj := appsv1alpha1.ClusterDefinition{}
	if err := GetResourceObjectFromGVR(types.ClusterGVR(), key, cli, &clusterObj); err != nil {
		return nil, err
	}

	if err := GetResourceObjectFromGVR(types.ClusterDefGVR(), client.ObjectKey{
		Namespace: "",
		Name:      clusterObj.Spec.ClusterDefRef,
	}, cli, &clusterDefObj); err != nil {
		return nil, err
	}

	componentNames := make([]string, 0, len(clusterObj.Spec.ComponentSpecs))
	for _, component := range clusterObj.Spec.ComponentSpecs {
		cdComponent := clusterDefObj.GetComponentDefByName(component.ComponentDefRef)
		if enableReconfiguring(cdComponent) {
			componentNames = append(componentNames, component.Name)
		}
	}
	return componentNames, nil
}

func enableReconfiguring(component *appsv1alpha1.ClusterComponentDefinition) bool {
	if component == nil || component.ConfigSpec == nil {
		return false
	}
	for _, tpl := range component.ConfigSpec.ConfigTemplateRefs {
		if len(tpl.ConfigConstraintRef) > 0 && len(tpl.ConfigTplRef) > 0 {
			return true
		}
	}
	return false
}

// IsSupportConfigureParams check whether all updated parameters belong to config template parameters.
func IsSupportConfigureParams(tpl appsv1alpha1.ConfigTemplate, values map[string]string, cli dynamic.Interface) (bool, error) {
	var (
		err              error
		configConstraint = appsv1alpha1.ConfigConstraint{}
	)

	if err := GetResourceObjectFromGVR(types.ConfigConstraintGVR(), client.ObjectKey{
		Namespace: "",
		Name:      tpl.ConfigConstraintRef,
	}, cli, &configConstraint); err != nil {
		return false, err
	}

	if configConstraint.Spec.ConfigurationSchema == nil {
		return true, nil
	}

	schema := configConstraint.Spec.ConfigurationSchema.DeepCopy()
	if schema.Schema == nil {
		schema.Schema, err = cfgcore.GenerateOpenAPISchema(schema.CUE, configConstraint.Spec.CfgSchemaTopLevelName)
		if err != nil {
			return false, err
		}
	}

	schemaSpec := schema.Schema.Properties["spec"]
	for key := range values {
		if _, ok := schemaSpec.Properties[key]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func getIPLocation() (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://ifconfig.io/country_code", nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	location, err := io.ReadAll(resp.Body)
	if len(location) == 0 || err != nil {
		return "", err
	}

	// remove last "\n"
	return string(location[:len(location)-1]), nil
}

// GetHelmChartRepoURL get helm chart repo, we will choose one from GitHub and GitLab based on the IP location
func GetHelmChartRepoURL() string {
	if types.KubeBlocksChartURL == testing.KubeBlocksChartURL {
		return testing.KubeBlocksChartURL
	}

	location, _ := getIPLocation()
	if location == "CN" {
		return types.GitLabHelmChartRepo
	}
	return types.KubeBlocksChartURL
}
