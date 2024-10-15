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

package util

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	sendQPS       = 30
	sendBurstSize = 25
)

var (
	once      sync.Once
	namespace string
	podName   string
	nodeName  string
	recorder  record.EventRecorder
	clientSet *kubernetes.Clientset
)

func SendEventWithMessage(logger *logr.Logger, reason string, message string) {
	go func() {
		once.Do(func() {
			namespace = os.Getenv(constant.KBEnvNamespace)
			podName = os.Getenv(constant.KBEnvPodName)
			nodeName = os.Getenv(constant.KBEnvNodeName)
			err := initEventRecorder()
			if logger != nil && err != nil {
				logger.Error(err, "init event recorder failed")
			}
		})

		err := sendEvent(reason, message)
		if logger != nil && err != nil {
			logger.Error(err, "send event failed")
		}
	}()
}

func initEventRecorder() error {
	err := getK8sClientSet()
	if err != nil {
		return fmt.Errorf("failed to get k8s clientset: %v", err)
	}

	eventBroadcaster := record.NewBroadcasterWithCorrelatorOptions(
		record.CorrelatorOptions{
			QPS:       sendQPS,
			BurstSize: sendBurstSize,
		})
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: clientSet.CoreV1().Events(""),
		},
	)

	recorder = eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{
			Component: "kbagent",
			Host:      nodeName,
		},
	)
	return nil
}

func sendEvent(reason string, message string) error {
	pod, err := getPodObject()
	if err != nil {
		return err
	}
	recorder.Event(pod, corev1.EventTypeNormal, reason, message)
	return nil
}

func getK8sClientSet() error {
	if clientSet != nil {
		return nil
	}
	restConfig, err := ctlruntime.GetConfig()
	if err != nil {
		return err
	}
	clientSet, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	return nil
}

func getPodObject() (*corev1.Pod, error) {
	if clientSet == nil {
		err := getK8sClientSet()
		if err != nil {
			return nil, fmt.Errorf("failed to get k8s clientset: %v", err)
		}
	}

	pod, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %v", err)
	}
	return pod, nil
}
