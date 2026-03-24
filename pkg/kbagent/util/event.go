/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"hash/fnv"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	defaultRetryInterval    = 10 * time.Second
	defaultMaxRetryAttempts = 30

	syncRetryInterval    = 200 * time.Millisecond
	syncMaxRetryAttempts = 3
)

func SendEventWithMessage(logger *logr.Logger, reason string, message string, sync bool) error {
	send := func(retryInterval time.Duration, retryAttempts int32) error {
		err := createOrUpdateEvent(reason, message, retryInterval, retryAttempts)
		if err != nil && logger != nil {
			logger.Error(err, "failed to send event", "reason", reason, "message", message)
		}
		return err
	}
	if sync {
		return send(syncRetryInterval, syncMaxRetryAttempts)
	}
	go func() {
		_ = send(defaultRetryInterval, defaultMaxRetryAttempts)
	}()
	return nil
}

func newEvent(reason string, message string) *corev1.Event {
	now := metav1.Now()
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateEventName(reason, message),
			Namespace: namespace(),
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: namespace(),
			Name:      podName(),
			UID:       types.UID(podUID()),
			FieldPath: proto.ProbeEventFieldPath,
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: proto.ProbeEventSourceComponent,
			Host:      nodeName(),
		},
		FirstTimestamp:      now,
		LastTimestamp:       now,
		EventTime:           metav1.NowMicro(),
		ReportingController: proto.ProbeEventReportingController,
		ReportingInstance:   podName(),
		Action:              reason,
		Type:                "Normal",
		Count:               1,
	}
}

func createOrUpdateEvent(reason, message string, retryInterval time.Duration, retryAttempts int32) error {
	clientSet, err := getK8sClientSet()
	if err != nil {
		return err
	}
	eventsClient := clientSet.CoreV1().Events(namespace())
	eventName := generateEventName(reason, message)

	var event *corev1.Event
	attempts := max(retryAttempts, 1)
	for i := int32(0); i < attempts; i++ {
		event, err = eventsClient.Get(context.Background(), eventName, metav1.GetOptions{})
		if err == nil {
			event.Count++
			// the granularity of lastTimestamp is second and it is not enough for the event.
			// there may multiple events in the same second, so we need to use EventTime here.
			// event.LastTimestamp = metav1.Now()
			event.EventTime = metav1.NowMicro()
			_, err = eventsClient.Update(context.Background(), event, metav1.UpdateOptions{})
		} else if k8serrors.IsNotFound(err) {
			event = newEvent(reason, message)
			_, err = eventsClient.Create(context.Background(), event, metav1.CreateOptions{})
		}
		if err == nil {
			return nil
		}
		if retryInterval > 0 && i < attempts-1 {
			time.Sleep(retryInterval)
		}
	}
	return errors.Wrapf(err, "failed to create or update event after %d attempts", retryAttempts)
}

func getK8sClientSet() (*kubernetes.Clientset, error) {
	restConfig, err := ctlruntime.GetConfig()
	if err != nil {
		return nil, errors.Wrap(err, "get kubeConfig failed")
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

func generateEventName(reason, message string) string {
	hash := fnv.New32a()
	fmt.Fprintf(hash, "%s.%s.%s", podUID(), reason, message)
	return fmt.Sprintf("%s.%x", podName(), hash.Sum32())
}
