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

package internal

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/viper"
)

type fakeRoleAgent struct {
	*RoleAgent
}

func TestInit(t *testing.T) {

	var ports = []int{1, 2, 3}
	initEnv(ports)

	faker := mockFakeAgent()
	err := faker.Init()
	if err != nil {
		t.Errorf("faker init failed ,err = %v", err)
	}
	if faker.FailedEventReportFrequency != 3600 {
		t.Errorf("faker.FailedEventReportFrequency init failed: %d", faker.FailedEventReportFrequency)
	}
	if faker.RoleObservationThreshold != 60 {
		t.Errorf("faker.RoleObservationThreshold init failed: %d", faker.RoleObservationThreshold)
	}
	if faker.client == nil {
		t.Error("faker.client init failed")
	}
	if len(*faker.actionSvcPorts) != len(ports) {
		t.Errorf("faker.actionSvcPorts init failed: len = %d", len(*faker.actionSvcPorts))
	} else {
		for i := 0; i < len(ports); i++ {
			if (*faker.actionSvcPorts)[i] != ports[i] {
				t.Errorf("faker.actionSvcPorts init failed: port%d shouldn't be %d", i, (*faker.actionSvcPorts)[i])
			}
		}
	}
}

func TestGetRoleFailed(t *testing.T) {
	var ports = []int{8080}
	initEnv(ports)
	faker := mockFakeAgent()
	err := faker.Init()
	if err != nil {
		t.Errorf("faker init failed ,err = %v", err)
	}
	ops, b := faker.CheckRole(context.Background())
	event, has := ops["event"]
	if !has || !b || (has && event == OperationSuccess) {
		t.Error("faker CheckRole failed")
	}
}

func mockFakeAgent() *fakeRoleAgent {
	roleAgent := NewRoleAgent(os.Stdin, "")
	return &fakeRoleAgent{RoleAgent: roleAgent}
}

func (fakeRoleAgent *fakeRoleAgent) Init() error {
	err := fakeRoleAgent.RoleAgent.Init()
	if err != nil {
		return err
	}
	return nil
}

func initEnv(ports []int) {
	viper.Set("KB_FAILED_EVENT_REPORT_FREQUENCY", "3601")
	viper.Set("KB_ROLE_OBSERVATION_THRESHOLD", "59")
	buf, _ := json.Marshal(ports)
	viper.Set("KB_CONSENSUS_SET_ACTION_SVC_LIST", string(buf))
}
