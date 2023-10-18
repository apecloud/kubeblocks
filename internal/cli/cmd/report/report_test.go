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

package report

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	kbclischeme "github.com/apecloud/kubeblocks/internal/cli/scheme"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	kbclientset "github.com/apecloud/kubeblocks/pkg/client/clientset/versioned/fake"
)

var _ = Describe("report", func() {
	var (
		namespace = "test"
		streams   genericiooptions.IOStreams
		tf        *cmdtesting.TestFactory
	)

	const (
		jsonFormat = "json"
		yamlFormat = "yaml"
	)

	buildClusterResourceSelectorMap := func(clusterName string) map[string]string {
		// app.kubernetes.io/instance: <clusterName>
		// app.kubernetes.io/managed-by: kubeblocks
		return map[string]string{
			constant.AppInstanceLabelKey:  clusterName,
			constant.AppManagedByLabelKey: constant.AppName,
		}
	}

	BeforeEach(func() {
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		tf.Client = &clientfake.RESTClient{}
		tf.FakeDynamicClient = testing.FakeDynamicClient()
		streams = genericiooptions.NewTestIOStreamsDiscard()
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("report options", func() {
		It("check outputformat", func() {
			o := newReportOptions(streams)
			Expect(o.validate()).Should(HaveOccurred())
			o.outputFormat = jsonFormat
			Expect(o.validate()).ShouldNot(HaveOccurred())
			o.outputFormat = yamlFormat
			Expect(o.validate()).ShouldNot(HaveOccurred())
			o.outputFormat = "text"
			Expect(o.validate()).Should(HaveOccurred())
		})

		It("check toLogOptions", func() {
			o := newReportOptions(streams)
			o.outputFormat = jsonFormat
			logOptions, err := o.toLogOptions()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(logOptions).ShouldNot(BeNil())
			Expect(logOptions.SinceTime).Should(BeNil())
			Expect(logOptions.SinceSeconds).Should(BeNil())
		})

		It("check toLogOptions with since-time", func() {
			o := newReportOptions(streams)
			o.outputFormat = jsonFormat
			o.sinceDuration = time.Hour
			logOptions, err := o.toLogOptions()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(logOptions).ShouldNot(BeNil())
			Expect(logOptions.SinceTime).Should(BeNil())
			Expect(logOptions.SinceSeconds).ShouldNot(BeNil())
			sinceSeconds := logOptions.SinceSeconds
			Expect(*sinceSeconds).Should(Equal(int64(3600)))
		})

		It("check toLogOptions with since", func() {
			o := newReportOptions(streams)
			o.outputFormat = jsonFormat
			o.sinceTime = "2023-05-23T00:00:00Z"
			logOptions, err := o.toLogOptions()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(logOptions).ShouldNot(BeNil())
			Expect(logOptions.SinceTime).ShouldNot(BeNil())
			Expect(logOptions.SinceSeconds).Should(BeNil())

			sicneTime := logOptions.SinceTime
			Expect(sicneTime.Format(time.RFC3339)).Should(Equal("2023-05-23T00:00:00Z"))
		})

		It("validate report options", func() {
			var err error
			o := newReportOptions(streams)
			o.outputFormat = jsonFormat
			Expect(o.validate()).ShouldNot(HaveOccurred())

			// set since-time
			o.sinceTime = "2023-05-23T00:00:00Z"
			// disable log
			o.withLogs = false
			Expect(o.validate()).Should(HaveOccurred())

			// enable log
			o.withLogs = true
			Expect(o.validate()).Should(Succeed())

			// set since
			o.sinceDuration = time.Hour
			err = o.validate()
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(Equal("only one of --since-time / --since may be used"))

			o.withLogs = false
			o.sinceDuration = 0
			o.allContainers = true
			err = o.validate()
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(MatchRegexp("can only be used when --with-logs is set"))

			o.withLogs = true
			err = o.validate()
			Expect(err).Should(Succeed())
		})
	})

	Context("parse printer", func() {
		const charName = "JohnSnow"
		// check default printer
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-secret",
			},
			Data: map[string][]byte{
				"test": []byte(charName),
			},
		}
		configMap := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-configmap",
			},
			Data: map[string]string{
				"test": charName,
			},
		}

		It("use default printer", func() {
			var err error
			var strBuffer bytes.Buffer

			o := newReportOptions(streams)
			o.outputFormat = jsonFormat
			Expect(o.validate()).ShouldNot(HaveOccurred())
			print, err := o.parsePrinter()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(print).ShouldNot(BeNil())

			// test default printer, print secret
			err = print.PrintObj(secret, &strBuffer)
			Expect(err).Should(Succeed())
			copySecret := &corev1.Secret{}
			Expect(json.Unmarshal(strBuffer.Bytes(), &copySecret)).Should(Succeed())
			for _, v := range copySecret.Data {
				Expect(v).Should(Equal([]byte(charName)))
			}
			// test default printer, print config map
			strBuffer.Reset()
			err = print.PrintObj(configMap, &strBuffer)
			Expect(err).Should(Succeed())
			copyConfigMap := &corev1.ConfigMap{}
			Expect(json.Unmarshal(strBuffer.Bytes(), &copyConfigMap)).Should(Succeed())
			for _, v := range copyConfigMap.Data {
				Expect(v).Should(Equal(charName))
			}
		})

		It("use mask printer", func() {
			var err error
			var strBuffer bytes.Buffer

			o := newReportOptions(streams)
			o.outputFormat = jsonFormat
			o.mask = true

			Expect(o.validate()).ShouldNot(HaveOccurred())
			print, err := o.parsePrinter()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(print).ShouldNot(BeNil())

			// test default printer, print secret
			err = print.PrintObj(secret, &strBuffer)
			Expect(err).Should(Succeed())
			copySecret := &corev1.Secret{}
			Expect(json.Unmarshal(strBuffer.Bytes(), &copySecret)).Should(Succeed())
			for _, v := range copySecret.Data {
				Expect(v).Should(Equal([]byte(EncryptedData)))
			}
			// test default printer, print config map
			strBuffer.Reset()
			err = print.PrintObj(configMap, &strBuffer)
			Expect(err).Should(Succeed())
			copyConfigMap := &corev1.ConfigMap{}
			Expect(json.Unmarshal(strBuffer.Bytes(), &copyConfigMap)).Should(Succeed())
			for _, v := range copyConfigMap.Data {
				Expect(v).Should(Equal(EncryptedData))
			}
		})

		It("use mask printer for unstructured data", func() {
			var err error
			var strBuffer bytes.Buffer

			o := newReportOptions(streams)
			o.outputFormat = jsonFormat
			o.mask = true

			Expect(o.validate()).ShouldNot(HaveOccurred())
			print, err := o.parsePrinter()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(print).ShouldNot(BeNil())

			unstructuredSecret, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
			Expect(err).ShouldNot(HaveOccurred())
			unstructuredConfigMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(configMap)
			Expect(err).ShouldNot(HaveOccurred())

			unstructuredSecretObj := &unstructured.Unstructured{Object: unstructuredSecret}
			unstructuredConfigMapObj := &unstructured.Unstructured{Object: unstructuredConfigMap}

			// test default printer, print secret
			strBuffer.Reset()
			err = print.PrintObj(unstructuredSecretObj, &strBuffer)
			Expect(err).Should(Succeed())
			copySecret := &corev1.Secret{}
			Expect(json.Unmarshal(strBuffer.Bytes(), &copySecret)).Should(Succeed())
			for _, v := range copySecret.Data {
				Expect((string)(v)).Should(Equal(EncryptedData))
			}
			// test default printer, print config map
			strBuffer.Reset()
			err = print.PrintObj(unstructuredConfigMapObj, &strBuffer)
			Expect(err).Should(Succeed())
			copyConfigMap := &corev1.ConfigMap{}
			Expect(json.Unmarshal(strBuffer.Bytes(), &copyConfigMap)).Should(Succeed())
			for _, v := range copyConfigMap.Data {
				Expect(v).Should(Equal(EncryptedData))
			}
		})
	})

	Context("report-kubeblocks options", func() {
		const (
			namespace = "test"
		)

		BeforeEach(func() {
			tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
			codec := kbclischeme.Codecs.LegacyCodec(kbclischeme.Scheme.PrioritizedVersionsAllGroups()...)

			// mock a secret for helm chart
			secrets := testing.FakeSecretsWithLabels(namespace, map[string]string{
				"name":  types.KubeBlocksChartName,
				"owner": "helm",
			})

			deploy := testing.FakeKBDeploy("0.5.23")
			deploy.Namespace = namespace
			deploymentList := &appsv1.DeploymentList{}
			deploymentList.Items = []appsv1.Deployment{*deploy}
			// mock events
			events := &corev1.EventList{}
			pods := testing.FakePods(1, namespace, deploy.Name)
			podEvent := testing.FakeEventForObject("test-event-pod", namespace, pods.Items[0].Name)
			events.Items = append(events.Items, *podEvent)

			httpResp := func(obj runtime.Object) *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
			}

			tf.UnstructuredClient = &clientfake.RESTClient{
				GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					urlPrefix := "/api/v1/namespaces/" + namespace
					return map[string]*http.Response{
						urlPrefix + "/deployments": httpResp(deploymentList),
						urlPrefix + "/events":      httpResp(events),
						urlPrefix + "/pods":        httpResp(pods),
						"/api/v1/secrets":          httpResp(secrets),
					}[req.URL.Path], nil
				}),
			}

			tf.Client = tf.UnstructuredClient
			tf.FakeDynamicClient = testing.FakeDynamicClient(deploy, podEvent, &secrets.Items[0])
			streams = genericiooptions.NewTestIOStreamsDiscard()
		})

		It("complete kb-report options", func() {
			o := reportKubeblocksOptions{reportOptions: newReportOptions(streams)}
			o.outputFormat = jsonFormat
			Expect(o.complete(tf)).To(Succeed())
			Expect(o.genericClientSet).ShouldNot(BeNil())
			Expect(o.namespace).Should(Equal(namespace))
			Expect(o.file).Should(MatchRegexp("report-kubeblocks-.*.zip"))
		})

		It("complete kb-report manifest", func() {
			o := reportKubeblocksOptions{reportOptions: newReportOptions(streams)}
			o.outputFormat = jsonFormat
			Expect(o.complete(tf)).To(Succeed())

			By("use fake zip writter to test handleManifests")
			o.reportWritter = &fakeZipWritter{}
			ctx := context.Background()
			Expect(o.handleManifests(ctx)).To(Succeed())
			Expect(o.handleEvents(ctx)).To(Succeed())
			Expect(o.handleLogs(ctx)).To(Succeed())
		})
	})

	Context("report-cluster options", func() {
		const (
			namespace       = "test"
			clusterName     = "test-cluster"
			deployName      = "test-deploy"
			statefulSetName = "test-statefulset"
		)
		var (
			kbfakeclient *kbclientset.Clientset
		)
		BeforeEach(func() {
			clusterLabels := buildClusterResourceSelectorMap(clusterName)

			tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
			codec := kbclischeme.Codecs.LegacyCodec(kbclischeme.Scheme.PrioritizedVersionsAllGroups()...)

			cluster := testing.FakeCluster(clusterName, namespace)
			clusterDef := testing.FakeClusterDef()
			clusterVersion := testing.FakeClusterVersion()

			deploy := testing.FakeDeploy(deployName, namespace, clusterLabels)
			deploymentList := &appsv1.DeploymentList{}
			deploymentList.Items = []appsv1.Deployment{*deploy}

			// mock events
			events := &corev1.EventList{}
			event := testing.FakeEventForObject("test-event-cluster", namespace, clusterName)
			events.Items = []corev1.Event{*event}

			pods := testing.FakePods(1, namespace, clusterName)
			event = testing.FakeEventForObject("test-event-pod", namespace, pods.Items[0].Name)
			events.Items = append(events.Items, *event)

			sts := testing.FakeStatefulSet(statefulSetName, namespace, clusterLabels)
			statefulSetList := &appsv1.StatefulSetList{}
			statefulSetList.Items = []appsv1.StatefulSet{*sts}

			httpResp := func(obj runtime.Object) *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
			}

			tf.UnstructuredClient = &clientfake.RESTClient{
				GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					urlPrefix := "/api/v1/namespaces/" + namespace
					return map[string]*http.Response{
						urlPrefix + "/deployments":  httpResp(deploymentList),
						urlPrefix + "/statefulsets": httpResp(statefulSetList),
						urlPrefix + "/events":       httpResp(events),
						urlPrefix + "/pods":         httpResp(pods),
					}[req.URL.Path], nil
				}),
			}

			tf.Client = tf.UnstructuredClient
			tf.FakeDynamicClient = testing.FakeDynamicClient(deploy, sts, event)
			kbfakeclient = testing.FakeKBClientSet(cluster, clusterDef, clusterVersion)
			streams = genericiooptions.NewTestIOStreamsDiscard()
		})

		It("validate cluster-report options", func() {
			o := reportClusterOptions{reportOptions: newReportOptions(streams)}
			o.outputFormat = jsonFormat
			args := make([]string, 0)
			Expect(o.validate(args)).Should(HaveOccurred())
			args = append(args, "mycluster")
			Expect(o.validate(args)).Should(Succeed())
			args = append(args, "mycluster2")
			Expect(o.validate(args)).Should(HaveOccurred())
		})

		It("complete cluster-report options", func() {
			o := reportClusterOptions{reportOptions: newReportOptions(streams)}
			o.outputFormat = jsonFormat
			args := make([]string, 0)
			args = append(args, "mycluster")
			Expect(o.validate(args)).Should(Succeed())
			Expect(o.complete(tf)).To(Succeed())
			Expect(o.genericClientSet).ShouldNot(BeNil())
			Expect(len(o.namespace)).ShouldNot(Equal(0))
			Expect(o.file).Should(MatchRegexp("report-cluster-.*.zip"))
		})

		It("handle cluster-report manifests", func() {
			o := reportClusterOptions{reportOptions: newReportOptions(streams)}
			o.outputFormat = jsonFormat
			args := make([]string, 0)
			args = append(args, clusterName)
			Expect(o.validate(args)).Should(Succeed())
			Expect(o.complete(tf)).To(Succeed())
			o.genericClientSet.kbClientSet = kbfakeclient

			By("use fake zip writter to test handleManifests")
			o.reportWritter = &fakeZipWritter{}
			ctx := context.Background()
			Expect(o.handleManifests(ctx)).To(Succeed())
			Expect(o.handleEvents(ctx)).To(Succeed())
			Expect(o.handleLogs(ctx)).To(Succeed())
		})
	})
})

type fakeZipWritter struct {
	printer printers.ResourcePrinter
}

var _ reportWritter = &fakeZipWritter{}

func (w *fakeZipWritter) Init(file string, printer printers.ResourcePrinterFunc) error {
	w.printer = printer
	return nil
}

func (w *fakeZipWritter) Close() error {
	return nil
}
func (w *fakeZipWritter) WriteKubeBlocksVersion(fileName string, client kubernetes.Interface) error {
	return nil
}
func (w *fakeZipWritter) WriteObjects(folderName string, objects []*unstructured.UnstructuredList, format string) error {
	return nil
}
func (w *fakeZipWritter) WriteSingleObject(prefix string, kind string, name string, object runtime.Object, format string) error {
	return nil
}
func (w *fakeZipWritter) WriteEvents(folderName string, events map[string][]corev1.Event, format string) error {
	return nil
}

func (w *fakeZipWritter) WriteLogs(folderName string, ctx context.Context, client kubernetes.Interface,
	pods *corev1.PodList, logOptions corev1.PodLogOptions,
	allContainers bool) error {
	return nil
}
