/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package util

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cmdget "k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
)

// CloseQuietly closes `io.Closer` quietly. Very handy and helpful for code
// quality too.
func CloseQuietly(d io.Closer) {
	_ = d.Close()
}

// GetCliHomeDir returns kbcli home dir
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

// MakeSSHKeyPair makes a pair of public and private keys for SSH access.
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

// SetKubeConfig sets KUBECONFIG environment
func SetKubeConfig(cfg string) error {
	return os.Setenv("KUBECONFIG", cfg)
}

var addToScheme sync.Once

func NewFactory() cmdutil.Factory {
	configFlags := NewConfigFlagNoWarnings()
	// Add CRDs to the scheme. They are missing by default.
	addToScheme.Do(func() {
		if err := apiextv1.AddToScheme(scheme.Scheme); err != nil {
			// This should never happen.
			panic(err)
		}
	})
	return cmdutil.NewFactory(configFlags)
}

// NewConfigFlagNoWarnings returns a ConfigFlags that disables warnings.
func NewConfigFlagNoWarnings() *genericclioptions.ConfigFlags {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.WrapConfigFn = func(c *rest.Config) *rest.Config {
		c.WarningHandler = rest.NoWarnings{}
		return c
	}
	return configFlags
}

func GVRToString(gvr schema.GroupVersionResource) string {
	return strings.Join([]string{gvr.Resource, gvr.Version, gvr.Group}, ".")
}

// GetNodeByName chooses node by name from a node array
func GetNodeByName(nodes []*corev1.Node, name string) *corev1.Node {
	for _, node := range nodes {
		if node.Name == name {
			return node
		}
	}
	return nil
}

// ResourceIsEmpty checks if resource is empty or not
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

// OpenBrowser opens browser with url in different OS system
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
	return TimeFormatWithDuration(t, time.Minute)
}

// TimeFormatWithDuration formats time with specified precision
func TimeFormatWithDuration(t *metav1.Time, duration time.Duration) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return TimeTimeFormatWithDuration(t.Time, duration)
}

func TimeTimeFormat(t time.Time) string {
	const layout = "Jan 02,2006 15:04 UTC-0700"
	return t.Format(layout)
}

func timeLayout(precision time.Duration) string {
	layout := "Jan 02,2006 15:04 UTC-0700"
	switch precision {
	case time.Second:
		layout = "Jan 02,2006 15:04:05 UTC-0700"
	case time.Millisecond:
		layout = "Jan 02,2006 15:04:05.000 UTC-0700"
	}
	return layout
}

func TimeTimeFormatWithDuration(t time.Time, precision time.Duration) string {
	layout := timeLayout(precision)
	return t.Format(layout)
}

func TimeParse(t string, precision time.Duration) (time.Time, error) {
	layout := timeLayout(precision)
	return time.Parse(layout, t)
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

// CheckEmpty checks if string is empty, if yes, returns <none> for displaying
func CheckEmpty(str string) string {
	if len(str) == 0 {
		return types.None
	}
	return str
}

// BuildLabelSelectorByNames builds the label selector by instance names, the label selector is
// like "instance-key in (name1, name2)"
func BuildLabelSelectorByNames(selector string, names []string) string {
	if len(names) == 0 {
		return selector
	}

	label := fmt.Sprintf("%s in (%s)", constant.AppInstanceLabelKey, strings.Join(names, ","))
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
func GetConfigTemplateList(clusterName string, namespace string, cli dynamic.Interface, componentName string, reloadTpl bool) ([]appsv1alpha1.ComponentConfigSpec, error) {
	var (
		clusterObj        = appsv1alpha1.Cluster{}
		clusterDefObj     = appsv1alpha1.ClusterDefinition{}
		clusterVersionObj = appsv1alpha1.ClusterVersion{}
	)

	clusterKey := client.ObjectKey{
		Namespace: namespace,
		Name:      clusterName,
	}
	if err := GetResourceObjectFromGVR(types.ClusterGVR(), clusterKey, cli, &clusterObj); err != nil {
		return nil, err
	}
	clusterDefKey := client.ObjectKey{
		Namespace: "",
		Name:      clusterObj.Spec.ClusterDefRef,
	}
	if err := GetResourceObjectFromGVR(types.ClusterDefGVR(), clusterDefKey, cli, &clusterDefObj); err != nil {
		return nil, err
	}
	clusterVerKey := client.ObjectKey{
		Namespace: "",
		Name:      clusterObj.Spec.ClusterVersionRef,
	}
	if clusterVerKey.Name != "" {
		if err := GetResourceObjectFromGVR(types.ClusterVersionGVR(), clusterVerKey, cli, &clusterVersionObj); err != nil {
			return nil, err
		}
	}
	return GetConfigTemplateListWithResource(clusterObj.Spec.ComponentSpecs, clusterDefObj.Spec.ComponentDefs, clusterVersionObj.Spec.ComponentVersions, componentName, reloadTpl)
}

func GetConfigTemplateListWithResource(cComponents []appsv1alpha1.ClusterComponentSpec,
	dComponents []appsv1alpha1.ClusterComponentDefinition,
	vComponents []appsv1alpha1.ClusterComponentVersion,
	componentName string,
	reloadTpl bool) ([]appsv1alpha1.ComponentConfigSpec, error) {

	configSpecs, err := cfgcore.GetConfigTemplatesFromComponent(cComponents, dComponents, vComponents, componentName)
	if err != nil {
		return nil, err
	}
	if !reloadTpl {
		return configSpecs, nil
	}

	validConfigSpecs := make([]appsv1alpha1.ComponentConfigSpec, 0, len(configSpecs))
	for _, configSpec := range configSpecs {
		if configSpec.ConfigConstraintRef != "" && configSpec.TemplateRef != "" {
			validConfigSpecs = append(validConfigSpecs, configSpec)
		}
	}
	return validConfigSpecs, nil
}

// GetResourceObjectFromGVR queries the resource object using GVR.
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

// GetComponentsFromClusterName returns name of component.
func GetComponentsFromClusterName(key client.ObjectKey, cli dynamic.Interface) ([]string, error) {
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

	return GetComponentsFromResource(clusterObj.Spec.ComponentSpecs, &clusterDefObj)
}

// GetComponentsFromResource returns name of component.
func GetComponentsFromResource(componentSpecs []appsv1alpha1.ClusterComponentSpec, clusterDefObj *appsv1alpha1.ClusterDefinition) ([]string, error) {
	componentNames := make([]string, 0, len(componentSpecs))
	for _, component := range componentSpecs {
		cdComponent := clusterDefObj.GetComponentDefByName(component.ComponentDefRef)
		if enableReconfiguring(cdComponent) {
			componentNames = append(componentNames, component.Name)
		}
	}
	return componentNames, nil
}

func enableReconfiguring(component *appsv1alpha1.ClusterComponentDefinition) bool {
	if component == nil {
		return false
	}
	for _, tpl := range component.ConfigSpecs {
		if len(tpl.ConfigConstraintRef) > 0 && len(tpl.TemplateRef) > 0 {
			return true
		}
	}
	return false
}

// IsSupportReconfigureParams checks whether all updated parameters belong to config template parameters.
func IsSupportReconfigureParams(tpl appsv1alpha1.ComponentConfigSpec, values map[string]string, cli dynamic.Interface) (bool, error) {
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
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://ifconfig.io/country_code", nil)
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

// GetHelmChartRepoURL gets helm chart repo, chooses one from GitHub and GitLab based on the IP location
func GetHelmChartRepoURL() string {
	if types.KubeBlocksChartURL == testing.KubeBlocksChartURL {
		return testing.KubeBlocksChartURL
	}

	location, _ := getIPLocation()
	// if location is CN, or we can not get location, use GitLab helm chart repo
	if location == "CN" || location == "" {
		return types.GitLabHelmChartRepo
	}
	return types.KubeBlocksChartURL
}

// GetKubeBlocksNamespace gets namespace of KubeBlocks installation, infer namespace from helm secrets
func GetKubeBlocksNamespace(client kubernetes.Interface) (string, error) {
	secrets, err := client.CoreV1().Secrets(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{LabelSelector: types.KubeBlocksHelmLabel})
	// if KubeBlocks is upgraded, there will be multiple secrets
	if err == nil && len(secrets.Items) >= 1 {
		return secrets.Items[0].Namespace, nil
	}
	return "", errors.New("failed to get KubeBlocks installation namespace")
}

type ExposeType string

const (
	ExposeToVPC      ExposeType = "vpc"
	ExposeToInternet ExposeType = "internet"

	EnableValue  string = "true"
	DisableValue string = "false"
)

var ProviderExposeAnnotations = map[K8sProvider]map[ExposeType]map[string]string{
	EKSProvider: {
		ExposeToVPC: map[string]string{
			"service.beta.kubernetes.io/aws-load-balancer-type":     "nlb",
			"service.beta.kubernetes.io/aws-load-balancer-internal": "true",
		},
		ExposeToInternet: map[string]string{
			"service.beta.kubernetes.io/aws-load-balancer-type":     "nlb",
			"service.beta.kubernetes.io/aws-load-balancer-internal": "false",
		},
	},
	GKEProvider: {
		ExposeToVPC: map[string]string{
			"networking.gke.io/load-balancer-type": "Internal",
		},
		ExposeToInternet: map[string]string{},
	},
	AKSProvider: {
		ExposeToVPC: map[string]string{
			"service.beta.kubernetes.io/azure-load-balancer-internal": "true",
		},
		ExposeToInternet: map[string]string{
			"service.beta.kubernetes.io/azure-load-balancer-internal": "false",
		},
	},
	ACKProvider: {
		ExposeToVPC: map[string]string{
			"service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type": "intranet",
		},
		ExposeToInternet: map[string]string{
			"service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type": "internet",
		},
	},
	// TKE VPC LoadBalancer needs the subnet id, it's difficult for KB to get it, so we just support the internet on TKE now.
	// reference: https://cloud.tencent.com/document/product/457/45487
	TKEProvider: {
		ExposeToInternet: map[string]string{},
	},
}

func GetExposeAnnotations(provider K8sProvider, exposeType ExposeType) (map[string]string, error) {
	exposeAnnotations, ok := ProviderExposeAnnotations[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
	annotations, ok := exposeAnnotations[exposeType]
	if !ok {
		return nil, fmt.Errorf("unsupported expose type: %s on provider %s", exposeType, provider)
	}
	return annotations, nil
}

// BuildAddonReleaseName returns the release name of addon, its f
func BuildAddonReleaseName(addon string) string {
	return fmt.Sprintf("%s-%s", types.AddonReleasePrefix, addon)
}

// CombineLabels combines labels into a string
func CombineLabels(labels map[string]string) string {
	var labelStr []string
	for k, v := range labels {
		labelStr = append(labelStr, fmt.Sprintf("%s=%s", k, v))
	}

	// sort labelStr to make sure the order is stable
	sort.Strings(labelStr)

	return strings.Join(labelStr, ",")
}

func BuildComponentNameLabels(prefix string, names []string) string {
	return buildLabelSelectors(prefix, constant.KBAppComponentLabelKey, names)
}

// buildLabelSelectors builds the label selector by given label key, the label selector is
// like "label-key in (name1, name2)"
func buildLabelSelectors(prefix string, key string, names []string) string {
	if len(names) == 0 {
		return prefix
	}

	label := fmt.Sprintf("%s in (%s)", key, strings.Join(names, ","))
	if len(prefix) == 0 {
		return label
	} else {
		return prefix + "," + label
	}
}

// NewOpsRequestForReconfiguring returns a new common OpsRequest for Reconfiguring operation
func NewOpsRequestForReconfiguring(opsName, namespace, clusterName string) *appsv1alpha1.OpsRequest {
	return &appsv1alpha1.OpsRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.AppsAPIGroup, types.AppsAPIVersion),
			Kind:       types.KindOps,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsName,
			Namespace: namespace,
		},
		Spec: appsv1alpha1.OpsRequestSpec{
			ClusterRef:  clusterName,
			Type:        appsv1alpha1.ReconfiguringType,
			Reconfigure: &appsv1alpha1.Reconfigure{},
		},
	}
}
func ConvertObjToUnstructured(obj any) (*unstructured.Unstructured, error) {
	var (
		contentBytes    []byte
		err             error
		unstructuredObj = &unstructured.Unstructured{}
	)

	if contentBytes, err = json.Marshal(obj); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(contentBytes, unstructuredObj); err != nil {
		return nil, err
	}
	return unstructuredObj, nil
}

func CreateResourceIfAbsent(
	dynamic dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespace string,
	unstructuredObj *unstructured.Unstructured) error {
	objectName, isFound, err := unstructured.NestedString(unstructuredObj.Object, "metadata", "name")
	if !isFound || err != nil {
		return err
	}
	objectByte, err := json.Marshal(unstructuredObj)
	if err != nil {
		return err
	}
	if _, err = dynamic.Resource(gvr).Namespace(namespace).Patch(
		context.TODO(), objectName, k8sapitypes.MergePatchType,
		objectByte, metav1.PatchOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			if _, err = dynamic.Resource(gvr).Namespace(namespace).Create(
				context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func BuildClusterDefinitionRefLable(prefix string, clusterDef []string) string {
	return buildLabelSelectors(prefix, constant.AppNameLabelKey, clusterDef)
}

// IsWindows returns true if the kbcli runtime situation is windows
func IsWindows() bool {
	return runtime.GOOS == types.GoosWindows
}
