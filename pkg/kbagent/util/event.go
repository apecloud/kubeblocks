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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apecloud/kubeblocks/pkg/constant"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"
)

const (
	sendEventMaxAttempts   = 30
	sendEventRetryInterval = 10 * time.Second
)

var (
	once      sync.Once
	counter   int32
	namespace string
	podName   string
	podUID    string
	nodeName  string
)

func SendEventWithMessage(logger *logr.Logger, reason string, message string) {
	go func() {
		once.Do(func() {
			namespace = os.Getenv(constant.KBEnvNamespace)
			podName = os.Getenv(constant.KBEnvPodName)
			podUID = os.Getenv(constant.KBEnvPodUID)
			nodeName = os.Getenv(constant.KBEnvNodeName)
		})
		err := createOrUpdateEvent(reason, message)
		if logger != nil && err != nil {
			logger.Error(err, "send or update event failed")
		}
	}()
}

func createOrUpdateEvent(reason string, message string) error {
	clientSet, err := getK8sClientSet()
	if err != nil {
		return fmt.Errorf("error getting k8s clientset: %v", err)
	}
	event, err := clientSet.CoreV1().Events(namespace).Get(context.TODO(), string(atomic.LoadInt32(&counter)-1), metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting event: %v", err)
		}
		event = newEvent(reason, message)
		return createEvent(clientSet, event)
	}

	return updateEvent(clientSet, event)
}

func newEvent(reason string, message string) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(atomic.AddInt32(&counter, 1)),
			Namespace: namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: namespace,
			Name:      podName,
			UID:       types.UID(podUID),
			FieldPath: "spec.containers{kbagent}",
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: "kbagent",
			Host:      nodeName,
		},
		FirstTimestamp:      metav1.Now(),
		LastTimestamp:       metav1.Now(),
		EventTime:           metav1.NowMicro(),
		ReportingController: "kbagent",
		ReportingInstance:   podName,
		Action:              reason,
		Type:                "Normal",
	}
}

func createEvent(clientSet *kubernetes.Clientset, event *corev1.Event) error {
	for i := 0; i < sendEventMaxAttempts; i++ {
		_, err := clientSet.CoreV1().Events(namespace).Create(context.Background(), event, metav1.CreateOptions{})
		if err == nil {
			return nil
		}
		time.Sleep(sendEventRetryInterval)
	}
	return fmt.Errorf("failed to send event after %d attempts", sendEventMaxAttempts)
}

func updateEvent(clientSet *kubernetes.Clientset, event *corev1.Event) error {
	event.Count += 1
	event.LastTimestamp = metav1.Now()

	_, err := clientSet.CoreV1().Events(namespace).Update(context.Background(), event, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating event: %v", err)
	}
	return nil
}

func getK8sClientSet() (*kubernetes.Clientset, error) {
	restConfig, err := ctlruntime.GetConfig()
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

func hashReasonNMessage(reason, message string) string {
	h := sha256.New()
	h.Write([]byte(reason + message))
	return hex.EncodeToString(h.Sum(nil))
}
