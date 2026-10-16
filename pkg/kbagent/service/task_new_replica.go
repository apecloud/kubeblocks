/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	newReplicaDataDump = "dataDump"
	newReplicaDataLoad = "dataLoad"

	targetPodNameEnv = "KB_TARGET_POD_NAME"
)

type newReplicaTask struct {
	logger        logr.Logger
	actionService *actionService
	task          *proto.NewReplicaTask
}

var _ task = &newReplicaTask{}

func (s *newReplicaTask) run(ctx context.Context) (chan error, error) {
	var pvcClient typedcorev1.PersistentVolumeClaimInterface
	if s.task.DataLoadResult != nil {
		var err error
		pvcClient, err = dataLoadPVCClient(s.task.DataLoadResult)
		if err != nil {
			return nil, err
		}
		loaded, err := dataLoadCompleted(ctx, pvcClient, s.task.DataLoadResult.PVCName)
		if err != nil {
			return nil, err
		}
		if loaded {
			return nil, nil
		}
	}
	if s.actionService == nil {
		return nil, fmt.Errorf("worker action service is required")
	}
	action, ok := s.actionService.actions[newReplicaDataLoad]
	if !ok {
		return nil, fmt.Errorf("%s is not supported", newReplicaDataLoad)
	}

	conn, err := s.handshake(ctx)
	if err != nil {
		return nil, err
	}

	ch, err := nonBlockingCallActionX(ctx, action, s.task.Parameters, nil, &action.TimeoutSeconds, conn, nil, nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	result := make(chan error, 1)
	go func() {
		defer conn.Close()
		err, ok := <-ch
		if !ok {
			err = fmt.Errorf("runtime error: error chan closed unexpectedly")
		}
		if err == nil && pvcClient != nil {
			err = s.saveDataLoadResult(ctx, pvcClient)
		}
		result <- err
	}()
	return result, nil
}

// DataLoadCompleted reads the actual data PVC before allowing the database to start.
func DataLoadCompleted(ctx context.Context, result *proto.DataLoadResult) (bool, error) {
	pvcClient, err := dataLoadPVCClient(result)
	if err != nil {
		return false, err
	}
	return dataLoadCompleted(ctx, pvcClient, result.PVCName)
}

func dataLoadPVCClient(result *proto.DataLoadResult) (typedcorev1.PersistentVolumeClaimInterface, error) {
	if result == nil || result.Namespace == "" || result.PVCName == "" {
		return nil, fmt.Errorf("data load result namespace and PVC name are required")
	}
	config, err := ctlruntime.GetConfig()
	if err != nil {
		return nil, err
	}
	client, err := typedcorev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client.PersistentVolumeClaims(result.Namespace), nil
}

func dataLoadCompleted(ctx context.Context, pvcClient typedcorev1.PersistentVolumeClaimInterface, name string) (bool, error) {
	pvc, err := pvcClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("read data PVC %s: %w", name, err)
	}
	return pvc.Annotations[proto.DataLoadedAnnotationKey] == "true", nil
}

func (s *newReplicaTask) saveDataLoadResult(ctx context.Context, pvcClient typedcorev1.PersistentVolumeClaimInterface) error {
	// Keep the loaded data in this worker while retrying only the result write.
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			pvc, err := pvcClient.Get(ctx, s.task.DataLoadResult.PVCName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if pvc.Annotations[proto.DataLoadedAnnotationKey] == "true" {
				return nil
			}
			if pvc.Annotations == nil {
				pvc.Annotations = make(map[string]string)
			}
			pvc.Annotations[proto.DataLoadedAnnotationKey] = "true"
			_, err = pvcClient.Update(ctx, pvc, metav1.UpdateOptions{})
			return err
		})
		if err != nil {
			s.logger.Error(err, "save data load result failed; retrying without reloading data", "PVC", s.task.DataLoadResult.PVCName)
		}
		return err == nil, nil
	})
}

func (s *newReplicaTask) status(ctx context.Context, event *proto.TaskEvent) {
	// TODO: query the progress
	event.Code = 0
	event.Output = nil
	event.Message = ""
}

func (s *newReplicaTask) handshake(ctx context.Context) (net.Conn, error) {
	conn, err := s.connectToRemote(ctx)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = conn.Close()
		}
	}()

	// reuse the action request as the handshake packet, define a new one when needed
	req := proto.ActionRequest{
		Action:     newReplicaDataDump,
		Parameters: s.task.Parameters,
	}
	if req.Parameters == nil {
		req.Parameters = make(map[string]string)
	}
	req.Parameters[targetPodNameEnv] = util.PodName()
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if len(data) > maxStreamingHandshakePacketSize {
		return nil, fmt.Errorf("handshake packet size is too large: %d", len(data))
	}

	ret, err := conn.Write(data)
	if err != nil {
		return nil, err
	}
	if ret != len(data) {
		return nil, fmt.Errorf("write streaming handshake request to remote error")
	}

	ok = true
	return conn, nil
}

func (s *newReplicaTask) connectToRemote(ctx context.Context) (net.Conn, error) {
	if len(s.task.Remote) == 0 {
		return nil, fmt.Errorf("remote server is required")
	}
	if s.task.Port == 0 {
		return nil, fmt.Errorf("remote port is required")
	}
	dialer := &net.Dialer{
		Timeout: defaultConnectTimeout,
	}
	return dialer.DialContext(ctx, "tcp", net.JoinHostPort(s.task.Remote, strconv.Itoa(int(s.task.Port))))
}
