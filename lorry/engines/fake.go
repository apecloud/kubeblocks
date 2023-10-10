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

	"github.com/apecloud/kubeblocks/lorry/dcs"
)

type FakeManager struct {
	DBManagerBase
}

var _ DBManager = &FakeManager{}

func (*FakeManager) IsRunning() bool {
	return true
}

func (*FakeManager) IsDBStartupReady() bool {
	return true
}

func (*FakeManager) InitializeCluster(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}
func (*FakeManager) IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*FakeManager) IsCurrentMemberInCluster(context.Context, *dcs.Cluster) bool {
	return true
}

func (*FakeManager) IsCurrentMemberHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (*FakeManager) IsClusterHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (*FakeManager) IsMemberHealthy(context.Context, *dcs.Cluster, *dcs.Member) bool {
	return true
}

func (*FakeManager) HasOtherHealthyLeader(context.Context, *dcs.Cluster) *dcs.Member {
	return nil
}

func (*FakeManager) HasOtherHealthyMembers(context.Context, *dcs.Cluster, string) []*dcs.Member {
	return nil
}

func (*FakeManager) IsLeader(context.Context, *dcs.Cluster) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*FakeManager) IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*FakeManager) IsFirstMember() bool {
	return true
}

func (*FakeManager) JoinCurrentMemberToCluster(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) LeaveMemberFromCluster(context.Context, *dcs.Cluster, string) error {
	return fmt.Errorf("NotSuppported")
}

func (*FakeManager) Promote(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) IsPromoted(context.Context) bool {
	return true
}

func (*FakeManager) Demote(context.Context) error {
	return fmt.Errorf("NotSuppported")
}

func (*FakeManager) Follow(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) Recover(context.Context) error {
	return nil

}

func (*FakeManager) GetHealthiestMember(*dcs.Cluster, string) *dcs.Member {
	return nil
}

func (*FakeManager) GetMemberAddrs(context.Context, *dcs.Cluster) []string {
	return nil
}

func (*FakeManager) IsRootCreated(context.Context) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*FakeManager) CreateRoot(context.Context) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) Lock(context.Context, string) error {
	return fmt.Errorf("NotSuppported")
}

func (*FakeManager) Unlock(context.Context) error {
	return fmt.Errorf("NotSuppported")
}
