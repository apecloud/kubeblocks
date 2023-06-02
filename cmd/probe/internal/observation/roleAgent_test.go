package observation

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

	faker := mockFakeController()
	faker.Init()
	if faker.FailedEventReportFrequency != 3600 {
		t.Errorf("faker.FailedEventReportFrequency init failed: %d", faker.FailedEventReportFrequency)
	}
	if faker.RoleDetectionThreshold != 60 {
		t.Errorf("faker.RoleDetectionThreshold init failed: %d", faker.RoleDetectionThreshold)
	}
	if faker.client == nil {
		t.Errorf("faker.client init failed")
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
	if len(faker.DBRoles) == 0 {
		t.Errorf("faker.DBRoles init failed: empty")
	}
}

func TestGetRoleFailed(t *testing.T) {
	var ports = []int{8080}
	initEnv(ports)
	faker := mockFakeController()
	faker.Init()
	ops, b := faker.CheckRoleOps(context.Background())
	event, has := ops["event"]
	if !has || !b || (has && event == OperationFailed) {
		t.Errorf("faker CheckRoleOps failed")
	}
}

func mockFakeController() *fakeRoleAgent {
	roleAgent := NewRoleAgent(os.Stdin, "")
	return &fakeRoleAgent{RoleAgent: roleAgent}
}

func (fakeRoleAgent *fakeRoleAgent) Init() {
	fakeRoleAgent.RoleAgent.Init()
}

func initEnv(ports []int) {
	viper.Set("KB_FAILED_EVENT_REPORT_FREQUENCY", "3601")
	viper.Set("KB_ROLE_DETECTION_THRESHOLD", "59")
	serviceRoles := make(map[string]interface{})
	serviceRoles["leader"] = ReadWrite
	buf, _ := json.Marshal(serviceRoles)
	viper.Set("KB_SERVICE_ROLES", string(buf))
	buf, _ = json.Marshal(ports)
	viper.Set("KB_CONSENSUS_SET_ACTION_SVC_LIST", string(buf))
}
