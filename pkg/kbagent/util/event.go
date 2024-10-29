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
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	ctlruntime "sigs.k8s.io/controller-runtime"
)

const (
	sendQPS       = 30
	sendBurstSize = 25
)

var (
	once      sync.Once
	recorder  record.EventRecorder
	clientSet *kubernetes.Clientset
)

func SendEventWithMessage(logger *logr.Logger, reason, message string) {
	once.Do(func() {
		if err := initEventRecorder(); err != nil && logger != nil {
			logger.Error(err, "Failed to initialize event recorder")
		}
	})

	go func() {
		if err := sendEvent(reason, message); err != nil && logger != nil {
			logger.Error(err, "Failed to send event")
		}
	}()
}

func initEventRecorder() error {
	if err := initializeClientSet(); err != nil {
		return fmt.Errorf("failed to get k8s clientset: %w", err)
	}

	eventBroadcaster := record.NewBroadcasterWithCorrelatorOptions(
		record.CorrelatorOptions{
			QPS:       sendQPS,
			BurstSize: sendBurstSize,
		},
	)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: clientSet.CoreV1().Events(""),
		},
	)

	recorder = eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{
			Component: "kbagent",
			Host:      nodeName(),
		},
	)
	return nil
}

func sendEvent(reason, message string) error {
	pod, err := clientSet.CoreV1().Pods(namespace()).Get(context.TODO(), podName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}
	recorder.Event(pod, corev1.EventTypeNormal, reason, message)
	return nil
}

func initializeClientSet() error {
	var err error
	clientSet, err = kubernetes.NewForConfig(ctlruntime.GetConfigOrDie())
	return err
}
