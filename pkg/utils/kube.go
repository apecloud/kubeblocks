/*
Copyright © 2022 The OpenCli Authors

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

package utils

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/util"
)

func PortForward(svc string, port string) error {
	f := buildFactory()
	restClient, err := f.RESTClient()
	if err != nil {
		return err
	}
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	ns, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil
	}

	builder := f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ContinueOnError().
		NamespaceParam(ns).DefaultNamespace()
	builder.ResourceNames("pods", svc)
	obj, err := builder.Do().Object()
	if err != nil {
		return err
	}

	forwardablePod, err := polymorphichelpers.AttachablePodForObjectFn(f, obj, time.Second*60)
	if err != nil {
		return err
	}
	podName := forwardablePod.Name
	portnum, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	t := obj.(*corev1.Service)
	containerPort, err := util.LookupContainerPortNumberByServicePort(*t, *forwardablePod, int32(portnum))
	if err != nil {
		return err
	}
	ports := fmt.Sprintf("%s:%s", port, strconv.Itoa(int(containerPort)))
	clientset, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}
	podClient := clientset.CoreV1()

	stopChannel := make(chan struct{}, 1)
	readyChannel := make(chan struct{})

	pod, err := podClient.Pods(ns).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if stopChannel != nil {
			close(stopChannel)
		}
	}()

	req := restClient.Post().
		Resource("pods").
		Namespace(ns).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	fw, err := portforward.NewOnAddresses(dialer, []string{"localhost"}, []string{ports}, stopChannel, readyChannel, nil, nil)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func buildFactory() cmdutil.Factory {
	getter := genericclioptions.NewConfigFlags(true)
	if err := apiextv1.AddToScheme(scheme.Scheme); err != nil {
		// This should never happen.
		panic(err)
	}
	if err := apiextv1beta1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	return cmdutil.NewFactory(getter)
}
