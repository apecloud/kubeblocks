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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	sendEventMaxAttempts   = 30
	sendEventRetryInterval = 10 * time.Second
)

type eventInfo struct {
	namespace string
	podName   string
	podUID    string
	nodeName  string
}

func SendEventWithMessage(logger *logr.Logger, reason string, message string) {
	go func() {
		eventInfo := eventInfo{
			namespace: os.Getenv(constant.KBEnvNamespace),
			podName:   os.Getenv(constant.KBEnvPodName),
			podUID:    os.Getenv(constant.KBEnvPodUID),
			nodeName:  os.Getenv(constant.KBEnvNodeName),
		}
		// hash reason and message as event name
		suffix := hashReasonNMessage(reason, message)
		eventName := fmt.Sprintf("%s.%s", os.Getenv(constant.KBEnvPodName), suffix)
		err := sendOrUpdateEvent(eventInfo, reason, message, eventName)
		if logger != nil && err != nil {
			logger.Error(err, "send or update event failed")
		}
	}()
}

func sendOrUpdateEvent(eventInfo eventInfo, reason string, message string, eventName string) error {
	clientSet, err := getK8sClientSet()
	if err != nil {
		return fmt.Errorf("error getting k8s clientset: %v", err)
	}
	event, err := clientSet.CoreV1().Events(eventInfo.namespace).Get(context.TODO(), eventName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting event: %v", err)
		}
		event = createEvent(eventInfo, reason, message, eventName)
		return sendEvent(clientSet, event, eventInfo.namespace)
	}
	return updateEvent(clientSet, event, eventInfo.namespace)
}

func createEvent(eventInfo eventInfo, reason string, message string, eventName string) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eventName,
			Namespace: eventInfo.namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: eventInfo.namespace,
			Name:      eventInfo.podName,
			UID:       types.UID(eventInfo.podUID),
			FieldPath: "spec.containers{kbagent}",
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: "kbagent",
			Host:      eventInfo.nodeName,
		},
		FirstTimestamp:      metav1.Now(),
		LastTimestamp:       metav1.Now(),
		EventTime:           metav1.NowMicro(),
		ReportingController: "kbagent",
		ReportingInstance:   eventInfo.podName,
		Action:              reason,
		Type:                "Normal",
	}
}

func sendEvent(clientSet *kubernetes.Clientset, event *corev1.Event, namespace string) error {
	for i := 0; i < sendEventMaxAttempts; i++ {
		_, err := clientSet.CoreV1().Events(namespace).Create(context.Background(), event, metav1.CreateOptions{})
		if err == nil {
			return nil
		}
		time.Sleep(sendEventRetryInterval)
	}
	return fmt.Errorf("failed to send event after %d attempts", sendEventMaxAttempts)
}

func updateEvent(clientSet *kubernetes.Clientset, event *corev1.Event, namespace string) error {
	event.Count += 1
	event.LastTimestamp = metav1.Now()

	_, err := clientSet.CoreV1().Events(namespace).Update(context.TODO(), event, metav1.UpdateOptions{})
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
