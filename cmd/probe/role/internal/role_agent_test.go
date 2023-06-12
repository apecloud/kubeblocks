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
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
)

type fakeRoleAgent struct {
	*RoleAgent
}

func TestRoleAgent_Init(t *testing.T) {

	var ports = []int{1, 2, 3}
	initEnv(ports)

	faker := mockFakeAgent("role")
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

func TestGetRole(t *testing.T) {

	t.Run("Success", func(t *testing.T) {
		var ports = []int{9723}
		initEnv(ports)

		path := "role_agent_success"
		faker := mockFakeAgent(path)
		err := faker.Init()
		if err != nil {
			t.Errorf("faker init failed ,err = %v", err)
		}

		roleExpected := "leader"
		initHTTP(roleExpected, path, 9723, true)
		time.Sleep(1 * time.Second) // wait for http listen

		ops, notify := faker.CheckRole(context.Background())

		if len(ops) == 0 {
			t.Error("CheckRole returns empty ...")
		}
		event, has := ops["event"]
		if !has || event != OperationSuccess || !notify {
			t.Error("faker CheckRole failed")
		}

	})

	t.Run("Failure", func(t *testing.T) {
		var ports = []int{7171}
		initEnv(ports)
		path := "role_agent_failure"
		faker := mockFakeAgent(path)
		err := faker.Init()
		if err != nil {
			t.Errorf("faker init failed ,err = %v", err)
		}

		initHTTP("", path, 7171, false)
		time.Sleep(1 * time.Second)

		ops, notify := faker.CheckRole(context.Background())
		event, has := ops["event"]
		if !has || event != OperationFailed || !notify {
			t.Error("faker CheckRole failed")
		}
	})

}

func mockFakeAgent(path string) *fakeRoleAgent {
	return &fakeRoleAgent{RoleAgent: NewRoleAgent(os.Stdin, "", path)}
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
	viper.Set("KB_SRS_ACTION_SVC_LIST", string(buf))
}

func initHTTP(roleExpected, path string, port int, good bool) {
	path = "/" + path
	http.HandleFunc(path, func(writer http.ResponseWriter, request *http.Request) {
		if good {
			_, err := io.WriteString(writer, roleExpected)
			if err != nil {
				return
			}
		} else {
			writer.WriteHeader(http.StatusNotFound)
		}
	})
	go func() {
		err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
		if err != nil {
			return
		}
	}()
}
