/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package volume

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
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
)

var (
	dbManager     engines.DBManager
	mockDBManager *engines.MockDBManager
	dcsStore      dcs.DCS
	mockDCSStore  *dcs.MockDCS
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
	RunSpecs(t, "Volume Operations. Suite")
}

var _ = BeforeSuite(func() {
	// Init mock db manager
	InitMockDBManager()

	// Init mock dcs store
	InitMockDCSStore()
})

var _ = AfterSuite(func() {
})

func InitMockDBManager() {
	ctrl := gomock.NewController(GinkgoT())
	mockDBManager = engines.NewMockDBManager(ctrl)
	register.SetDBManager(mockDBManager)
	dbManager = mockDBManager
}

func InitMockDCSStore() {
	ctrl := gomock.NewController(GinkgoT())
	mockDCSStore = dcs.NewMockDCS(ctrl)
	mockDCSStore.EXPECT().GetClusterFromCache().Return(&dcs.Cluster{}).AnyTimes()
	dcs.SetStore(mockDCSStore)
	dcsStore = mockDCSStore
}
