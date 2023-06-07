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
	viper.Set("KB_ROLE_DETECTION_THRESHOLD", "59")
	buf, _ := json.Marshal(ports)
	viper.Set("KB_CONSENSUS_SET_ACTION_SVC_LIST", string(buf))
}
