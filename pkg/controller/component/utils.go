/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package component

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	ctlruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func inDataContext() *multicluster.ClientOption {
	return multicluster.InDataContext()
}

func ValidateCompDefRegexp(compDefPattern string) error {
	_, err := regexp.Compile(compDefPattern)
	return err
}

func CompDefMatched(compDef, compDefPattern string) bool {
	if strings.HasPrefix(compDef, compDefPattern) {
		return true
	}

	isRegexpPattern := func(pattern string) bool {
		escapedPattern := regexp.QuoteMeta(pattern)
		return escapedPattern != pattern
	}

	isRegex := false
	regex, err := regexp.Compile(compDefPattern)
	if err == nil {
		// distinguishing between regular expressions and ordinary strings.
		if isRegexpPattern(compDefPattern) {
			isRegex = true
		}
	}
	if !isRegex {
		return false
	}
	return regex.MatchString(compDef)
}

func IsHostNetworkEnabled(synthesizedComp *SynthesizedComponent) bool {
	if !hasHostNetworkCapability(synthesizedComp, nil) {
		return false
	}
	// legacy definition, ignore the cluster annotations
	if synthesizedComp.PodSpec.HostNetwork {
		return true
	}
	return hasHostNetworkEnabled(synthesizedComp.Annotations, synthesizedComp.Name)
}

func isHostNetworkEnabled(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, compName string) (bool, error) {
	// fast path: refer to self
	if compName == synthesizedComp.Name {
		return IsHostNetworkEnabled(synthesizedComp), nil
	}

	// check the component object that whether the host-network is enabled
	compKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, compName),
	}
	comp := &appsv1.Component{}
	if err := cli.Get(ctx, compKey, comp, inDataContext()); err != nil {
		return false, err
	}
	if !hasHostNetworkEnabled(comp.Annotations, compName) {
		return false, nil
	}

	// check the component definition that whether it has the host-network capability
	if len(comp.Spec.CompDef) > 0 {
		compDef := &appsv1.ComponentDefinition{}
		if err := cli.Get(ctx, types.NamespacedName{Name: comp.Spec.CompDef}, compDef); err != nil {
			return false, err
		}
		if hasHostNetworkCapability(nil, compDef) {
			return true, nil
		}
	}
	return false, nil
}

func hasHostNetworkCapability(synthesizedComp *SynthesizedComponent, compDef *appsv1.ComponentDefinition) bool {
	switch {
	case synthesizedComp != nil:
		return synthesizedComp.HostNetwork != nil
	case compDef != nil:
		return compDef.Spec.HostNetwork != nil
	}
	return false
}

func hasHostNetworkEnabled(annotations map[string]string, compName string) bool {
	if annotations == nil {
		return false
	}
	comps, ok := annotations[constant.HostNetworkAnnotationKey]
	if !ok {
		return false
	}
	return slices.Index(strings.Split(comps, ","), compName) >= 0
}

func getHostNetworkPort(ctx context.Context, _ client.Reader, clusterName, compName, cName, pName string) (int32, error) {
	key := intctrlutil.BuildHostPortName(clusterName, compName, cName, pName)
	if v, ok := ctx.Value(mockHostNetworkPortManagerKey{}).(map[string]int32); ok {
		if p, okk := v[key]; okk {
			return p, nil
		}
		return 0, nil
	}
	pm := intctrlutil.GetPortManager()
	if pm == nil {
		return 0, nil
	}
	return pm.GetPort(key)
}

func mockHostNetworkPort(ctx context.Context, _ client.Reader, clusterName, compName, cName, pName string, port int32) context.Context {
	key := intctrlutil.BuildHostPortName(clusterName, compName, cName, pName)
	mockHostNetworkPortManager[key] = port
	return context.WithValue(ctx, mockHostNetworkPortManagerKey{}, mockHostNetworkPortManager)
}

var (
	mockHostNetworkPortManager = map[string]int32{}
)

type mockHostNetworkPortManagerKey struct{}

type PortForwardManager struct {
	pfs []*portforward.PortForwarder
	mu  sync.Mutex
}

func NewPortForwardManager() *PortForwardManager {
	return &PortForwardManager{
		pfs: make([]*portforward.PortForwarder, 0),
		mu:  sync.Mutex{},
	}
}

func (pfm *PortForwardManager) NewPortForwarder(namespace, podName string, podPort int32) (int32, chan error, error) {
	clientSet, restConfig, err := getK8sClientSet()
	if err != nil {
		return -1, nil, err
	}
	return pfm.setupPortForward(restConfig, clientSet, namespace, podName, podPort)
}

func (pfm *PortForwardManager) setupPortForward(restConfig *rest.Config, clientset *kubernetes.Clientset, namespace, podName string, podPort int32) (int32, chan error, error) {
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return -1, nil, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	localPort := "0"
	ports := []string{fmt.Sprintf("%s:%d", localPort, podPort)}

	pf, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return -1, nil, err
	}

	pfm.addPortForwarder(pf)

	errChan := make(chan error, 1)
	go func(errChan chan error) {
		if err := pf.ForwardPorts(); err != nil {
			errChan <- err
		}
	}(errChan)

	select {
	case <-readyChan:
		portList, err := pf.GetPorts()
		if err != nil || len(portList) == 0 {
			return -1, nil, fmt.Errorf("get local port error")
		}
		return int32(portList[0].Local), errChan, nil
	case <-time.After(10 * time.Second):
		close(stopChan)
		return -1, nil, fmt.Errorf("port-forward timeout")
	}
}

func (pfm *PortForwardManager) addPortForwarder(pf *portforward.PortForwarder) {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()
	pfm.pfs = append(pfm.pfs, pf)
}

func (pfm *PortForwardManager) CloseAll() {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()
	for _, p := range pfm.pfs {
		p.Close()
	}
	pfm.pfs = nil
}

func getK8sClientSet() (*kubernetes.Clientset, *rest.Config, error) {
	restConfig, err := ctlruntime.GetConfig()
	if err != nil {
		return nil, nil, err
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	return clientSet, restConfig, nil
}
