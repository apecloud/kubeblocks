/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package container

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/configuration/container/mocks"
)

var zapLog, _ = zap.NewDevelopment()

func TestNewContainerKiller(t *testing.T) {
	zaplog, _ := zap.NewProduction()

	type args struct {
		criType    CRIType
		socketPath string
	}
	tests := []struct {
		name    string
		args    args
		want    ContainerKiller
		wantErr bool
	}{{
		name: "test1",
		args: args{
			criType: "xxxx",
		},
		wantErr: true,
	}, {
		name: "test2",
		args: args{
			criType:    ContainerdType,
			socketPath: "for_test",
		},
		wantErr: false,
		want: &containerdContainer{
			runtimeEndpoint: "for_test",
			logger:          zaplog.Sugar(),
		},
	}, {
		name: "test3",
		args: args{
			criType: DockerType,
		},
		wantErr: false,
		want: &dockerContainer{
			logger: zaplog.Sugar(),
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewContainerKiller(tt.args.criType, tt.args.socketPath, zaplog.Sugar())
			if (err != nil) != tt.wantErr {
				t.Errorf("NewContainerKiller() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewContainerKiller() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDockerContainerKill(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mocks.NewMockContainerAPIClient(mockCtrl)

	docker := &dockerContainer{
		logger: zapLog.Sugar(),
		dc:     cli,
	}

	// mock ContainerList failed
	cli.EXPECT().ContainerList(gomock.Any(), gomock.Any()).
		Return(nil, cfgcore.MakeError("docker service not ready!"))

	// mock container is not exist
	cli.EXPECT().ContainerList(gomock.Any(), gomock.Any()).
		Return([]types.Container{
			testContainer("docker", "e5a00fc1653e196287576abccb70bac7411f553d09096a16fc2e0d8a66e03a8e", ""),
		}, nil)

	// mock container is existed
	cli.EXPECT().ContainerList(gomock.Any(), gomock.Any()).
		Return([]types.Container{
			testContainer("docker", "e5a00fc1653e196287576abccb70bac7411f553d09096a16fc2e0d8a66e03a8e", ""),
			testContainer("docker", "76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55", "exited"),
		}, nil)

	// mock container is running
	cli.EXPECT().ContainerList(gomock.Any(), gomock.Any()).
		Return([]types.Container{
			testContainer("docker", "76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55", "running"),
			testContainer("docker", "754d7342de7feb16be79462bc9d72b9c37306ec08374e914ae0a8f8377d0d855", "running"),
		}, nil).AnyTimes()

	// mock ContainerKill failed
	cli.EXPECT().ContainerKill(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(cfgcore.MakeError("failed to kill docker container!"))
	// mock ContainerKill success
	cli.EXPECT().ContainerKill(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	require.ErrorContains(t,
		docker.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil),
		"docker service not ready!")
	require.Nil(t, docker.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil))
	require.Nil(t, docker.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil))
	require.ErrorContains(t,
		docker.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil),
		"failed to kill docker container")
	require.Nil(t, docker.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil))
}

func TestContainerdContainerKill(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	criCli := mocks.NewMockRuntimeServiceClient(mockCtrl)

	containerd := containerdContainer{
		logger:         zapLog.Sugar(),
		backendRuntime: criCli,
	}

	// ctx context.Context, in *ListContainersRequest, opts ...grpc.CallOption
	// mock ListContainers failed
	criCli.EXPECT().ListContainers(gomock.Any(), gomock.Any()).
		Return(nil, cfgcore.MakeError("failed to list containers!"))

	// mock not exist
	criCli.EXPECT().ListContainers(gomock.Any(), gomock.Any()).
		Return(nil, nil)

	// mock exited
	criCli.EXPECT().ListContainers(gomock.Any(), gomock.Any()).
		Return(&runtimeapi.ListContainersResponse{Containers: []*runtimeapi.Container{{
			Id:    "76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55",
			State: runtimeapi.ContainerState_CONTAINER_EXITED,
		}}}, nil)

	// mock running
	criCli.EXPECT().ListContainers(gomock.Any(), gomock.Any()).
		Return(&runtimeapi.ListContainersResponse{Containers: []*runtimeapi.Container{{
			Id:    "76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55",
			State: runtimeapi.ContainerState_CONTAINER_RUNNING,
		}}}, nil).AnyTimes()

	// mock stop failed
	criCli.EXPECT().StopContainer(gomock.Any(), gomock.Any()).
		Return(nil, cfgcore.MakeError("failed to stop container!"))
	criCli.EXPECT().StopContainer(gomock.Any(), gomock.Any()).
		Return(&runtimeapi.StopContainerResponse{}, nil)

	require.ErrorContains(t,
		containerd.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil),
		"failed to list containers!")

	require.Nil(t, containerd.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil))
	require.Nil(t, containerd.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil))

	require.ErrorContains(t,
		containerd.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil),
		"failed to stop container!")
	require.Nil(t, containerd.Kill(context.Background(), []string{"76f9c2ae8cf47bfa43b97626e3c95045cb3b82c50019ab759801ab52e3acff55"}, "", nil))

}

func TestAutoCheckCRIType(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "SocketFileTest-")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	var (
		testFile1  = filepath.Join(tmpDir, "file1.sock")
		testFile2  = filepath.Join(tmpDir, "file2.sock")
		testFile3  = filepath.Join(tmpDir, "file3.sock")
		dockerFile = filepath.Join(tmpDir, "docker.sock")
	)

	require.Equal(t, autoCheckCRIType([]string{testFile1, testFile2, testFile3}, dockerFile, zapLog.Sugar()), CRIType(""))

	l, err := net.Listen("unix", dockerFile)
	if err != nil {
		t.Errorf("failed to  create socket file: %s", dockerFile)
	}
	defer l.Close()
	require.Equal(t, autoCheckCRIType([]string{testFile1, testFile2, testFile3}, dockerFile, zapLog.Sugar()), DockerType)

	// mock grpc
	{
		listen, err := net.Listen("unix", testFile3)
		if err != nil {
			t.Fatalf("Error while listening. Err: %v", err)
		}
		defer listen.Close()
		listenDone := make(chan struct{})
		dialDone := make(chan struct{})
		// mock grpc connection
		go func() {
			defer close(listenDone)
			conn, err := listen.Accept()
			if err != nil {
				t.Errorf("failed to accepting. Err: %v", err)
				return
			}
			framer := http2.NewFramer(conn, conn)
			if err := framer.WriteSettings(http2.Setting{}); err != nil {
				t.Errorf("failed to writing settings. Err: %v", err)
				return
			}
			<-dialDone // wait close conn only after dial returns.
			conn.Close()
		}()

		// for test
		require.Equal(t, autoCheckCRIType([]string{testFile1, testFile2, testFile3}, dockerFile, zapLog.Sugar()), ContainerdType)
		// close grpc mock
		close(dialDone)

		// wait grpc listen close
		timeout := time.After(1 * time.Second)
		select {
		case <-timeout:
			t.Fatal("timed out waiting for server to finish")
		case <-listenDone:
		}
	}
}
