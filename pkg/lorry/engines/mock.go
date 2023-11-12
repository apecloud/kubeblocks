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

package engines

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

type MockManager struct {
	DBManagerBase
}

var _ DBManager = &MockManager{}

func NewMockManager(properties Properties) (DBManager, error) {
	logger := ctrl.Log.WithName("MockManager")

	managerBase, err := NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	Mgr := &MockManager{
		DBManagerBase: *managerBase,
	}

	return Mgr, nil
}
func (*MockManager) IsRunning() bool {
	return true
}

func (*MockManager) IsDBStartupReady() bool {
	return true
}

func (*MockManager) InitializeCluster(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}
func (*MockManager) IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*MockManager) IsCurrentMemberInCluster(context.Context, *dcs.Cluster) bool {
	return true
}

func (*MockManager) IsCurrentMemberHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (*MockManager) IsClusterHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (*MockManager) IsMemberHealthy(context.Context, *dcs.Cluster, *dcs.Member) bool {
	return true
}

func (*MockManager) HasOtherHealthyLeader(context.Context, *dcs.Cluster) *dcs.Member {
	return nil
}

func (*MockManager) HasOtherHealthyMembers(context.Context, *dcs.Cluster, string) []*dcs.Member {
	return nil
}

func (*MockManager) IsLeader(context.Context, *dcs.Cluster) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*MockManager) IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*MockManager) IsFirstMember() bool {
	return true
}

func (*MockManager) JoinCurrentMemberToCluster(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*MockManager) LeaveMemberFromCluster(context.Context, *dcs.Cluster, string) error {
	return fmt.Errorf("NotSupported")
}

func (*MockManager) Promote(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*MockManager) IsPromoted(context.Context) bool {
	return true
}

func (*MockManager) Demote(context.Context) error {
	return fmt.Errorf("NotSupported")
}

func (*MockManager) Follow(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*MockManager) Recover(context.Context) error {
	return nil

}

func (*MockManager) GetHealthiestMember(*dcs.Cluster, string) *dcs.Member {
	return nil
}

func (*MockManager) GetMemberAddrs(context.Context, *dcs.Cluster) []string {
	return nil
}

func (*MockManager) IsRootCreated(context.Context) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*MockManager) CreateRoot(context.Context) error {
	return fmt.Errorf("NotSupported")
}

func (*MockManager) Lock(context.Context, string) error {
	return fmt.Errorf("NotSupported")
}

func (*MockManager) Unlock(context.Context) error {
	return fmt.Errorf("NotSupported")
}
