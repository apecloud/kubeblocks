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

package grpcserver

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

func init() {
	viper.AutomaticEnv()
	viper.SetDefault(constant.KBEnvPodName, "pod-test")
	viper.SetDefault(constant.KBEnvClusterCompName, "cluster-component-test")
	viper.SetDefault(constant.KBEnvNamespace, "namespace-test")
	ctrl.SetLogger(zap.New())
}

func TestVolumeOperations(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GRPC Server. Suite")
}

var _ = BeforeSuite(func() {
	// Init mock dcs store
	InitMockDCSStore()
})

var _ = AfterSuite(func() {
})

func InitMockDCSStore() {
	ctrl := gomock.NewController(GinkgoT())
	mockDCSStore := dcs.NewMockDCS(ctrl)
	mockDCSStore.EXPECT().GetClusterFromCache().Return(&dcs.Cluster{}).AnyTimes()
	mockDCSStore.EXPECT().GetMembers().Return([]dcs.Member{{Name: "pod-test", UID: "123"}}, nil).AnyTimes()
	dcs.SetStore(mockDCSStore)
}
