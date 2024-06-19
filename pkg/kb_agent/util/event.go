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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

var logger = ctlruntime.Log.WithName("event")
var EventSendMaxAttempts = 30
var EventSendPeriod = 10 * time.Second

func SentEventForProbe(ctx context.Context, msg ActionMessage) error {
	logger.Info(fmt.Sprintf("send event: %v", msg))
	action := msg.GetAction()
	if action == "" {
		return errors.New("action is unset")
	}
	event, err := CreateEvent(action, msg)
	if err != nil {
		logger.Info("create event failed", "error", err.Error())
		return err
	}

	go func() {
		_ = SendEvent(ctx, event)
	}()

	return nil
}

func CreateEvent(reason string, msg ActionMessage) (*corev1.Event, error) {
	// get pod object
	podName := os.Getenv(constant.KBEnvPodName)
	podUID := os.Getenv(constant.KBEnvPodUID)
	nodeName := os.Getenv(constant.KBEnvNodeName)
	namespace := os.Getenv(constant.KBEnvNamespace)
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.%s", podName, rand.String(16)),
			Namespace: namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: namespace,
			Name:      podName,
			UID:       types.UID(podUID),
			FieldPath: "spec.containers{kb-agent}",
		},
		Reason:  reason,
		Message: string(data),
		Source: corev1.EventSource{
			Component: "kb-agent",
			Host:      nodeName,
		},
		FirstTimestamp:      metav1.Now(),
		LastTimestamp:       metav1.Now(),
		EventTime:           metav1.NowMicro(),
		ReportingController: "kb-agent",
		ReportingInstance:   podName,
		Action:              reason,
		Type:                "Normal",
	}
	return event, nil
}

func SendEvent(ctx context.Context, event *corev1.Event) error {
	ctx1 := context.Background()
	config, err := ctlruntime.GetConfig()
	if err != nil {
		logger.Info("get k8s client config failed", "error", err.Error())
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Info("k8s client create failed", "error", err.Error())
		return err
	}
	namespace := os.Getenv(constant.KBEnvNamespace)
	for i := 0; i < EventSendMaxAttempts; i++ {
		_, err = clientset.CoreV1().Events(namespace).Create(ctx1, event, metav1.CreateOptions{})
		if err == nil {
			logger.Info("send event success", "message", event.Message)
			break
		}
		logger.Info("send event failed", "error", err.Error())
		time.Sleep(EventSendPeriod)
	}
	return err
}
