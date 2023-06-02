package observation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type OpsResult map[string]interface{}
type AccessMode string

type Controller struct {
	CheckRoleFailedCount       int
	RoleUnchangedCount         int
	FailedEventReportFrequency int

	// RoleDetectionThreshold is used to set the report duration of role event after role changed,
	// then event controller can always get rolechanged events to maintain pod label accurately
	// in cases of:
	// 1 rolechanged event lost;
	// 2 pod role label deleted or updated incorrectly.
	RoleDetectionThreshold int
	OriRole                string // 原先的row
	Logger                 log.Logger
	DBRoles                map[string]AccessMode
	actionSvcPorts         *[]int
	client                 *http.Client
}

func init() {
	viper.SetDefault("KB_FAILED_EVENT_REPORT_FREQUENCY", defaultFailedEventReportFrequency)
	viper.SetDefault("KB_ROLE_DETECTION_THRESHOLD", defaultRoleDetectionThreshold)
}

func NewController(logger log.Logger) *Controller {
	return &Controller{
		Logger:         logger,
		actionSvcPorts: &[]int{},
	}
}

func (controller *Controller) Init() error {
	controller.FailedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if controller.FailedEventReportFrequency < 300 {
		controller.FailedEventReportFrequency = 300
	} else if controller.FailedEventReportFrequency > 3600 {
		controller.FailedEventReportFrequency = 3600
	}

	controller.RoleDetectionThreshold = viper.GetInt("KB_ROLE_DETECTION_THRESHOLD")
	if controller.RoleDetectionThreshold < 60 {
		controller.RoleDetectionThreshold = 60
	} else if controller.RoleDetectionThreshold > 300 {
		controller.RoleDetectionThreshold = 300
	}

	val := viper.GetString("KB_SERVICE_ROLES")
	if val != "" {
		if err := json.Unmarshal([]byte(val), &controller.DBRoles); err != nil {
			fmt.Println(errors.Wrap(err, "KB_DB_ROLES env format error").Error())
		}
	}

	actionSvcList := viper.GetString("KB_CONSENSUS_SET_ACTION_SVC_LIST")
	if len(actionSvcList) > 0 {
		err := json.Unmarshal([]byte(actionSvcList), controller.actionSvcPorts)
		if err != nil {
			return err
		}
	}

	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	netTransport := &http.Transport{
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	controller.client = &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}
	return nil
}

func (controller *Controller) ShutDownClient() {
	controller.client.CloseIdleConnections()
}

func (controller *Controller) CheckRoleOps(ctx context.Context) (OpsResult, bool) {
	opsRes := OpsResult{}
	needNotify := false
	role, err := controller.getRole(ctx)

	// OpsResult organized like below:
	// Success: event originalRole role
	// Invalid: event message
	// Failure: event message

	if err != nil {
		controller.Logger.Printf("error executing checkRole: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if controller.CheckRoleFailedCount%controller.FailedEventReportFrequency == 0 {
			controller.Logger.Printf("role checks failed %v times continuously", controller.CheckRoleFailedCount)
		}
		controller.CheckRoleFailedCount++
		return opsRes, true
	}

	controller.CheckRoleFailedCount = 0
	if isValid, message := controller.roleValidate(role); !isValid {
		opsRes["event"] = OperationInvalid
		opsRes["message"] = message
		return opsRes, true
	}

	opsRes["event"] = OperationSuccess
	opsRes["originalRole"] = controller.OriRole
	opsRes["role"] = role

	if controller.OriRole != role {
		controller.OriRole = role
		needNotify = true
	} else {
		controller.RoleUnchangedCount++
	}

	// RoleUnchangedCount is the count of consecutive role unchanged checks.
	// If the role remains unchanged consecutively in RoleDetectionThreshold checks after it has changed,
	// then the roleCheck event will be reported at roleEventReportFrequency so that the event controller
	// can always get relevant roleCheck events in order to maintain the pod label accurately, even in cases
	// of roleChanged events being lost or the pod role label being deleted or updated incorrectly.
	if controller.RoleUnchangedCount < controller.RoleDetectionThreshold && controller.RoleUnchangedCount%roleEventReportFrequency == 0 {
		needNotify = true
		controller.RoleUnchangedCount = 0
	}
	return opsRes, needNotify
}

func (controller *Controller) GetRoleOps(ctx context.Context) OpsResult {
	opsRes := OpsResult{}

	role, err := controller.getRole(ctx)
	if err != nil {
		controller.Logger.Printf("error executing getRole: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if controller.CheckRoleFailedCount%controller.FailedEventReportFrequency == 0 {
			controller.Logger.Printf("getRole failed %v times continuously", controller.CheckRoleFailedCount)
		}
		controller.CheckRoleFailedCount++
		return opsRes
	}

	controller.CheckRoleFailedCount = 0

	opsRes["event"] = OperationSuccess
	opsRes["role"] = role
	return opsRes
}

// Component may have some internal roles that need not be exposed to end user,
// and not configured in cluster definition, e.g. ETCD's Candidate.
// roleValidate is used to filter the internal roles and decrease the number
// of report events to reduce the possibility of event conflicts.
func (controller *Controller) roleValidate(role string) (bool, string) {

	// do not validate when db roles setting is missing
	if len(controller.DBRoles) == 0 {
		return true, ""
	}

	var msg string
	isValid := false
	for r := range controller.DBRoles {
		if strings.EqualFold(r, role) {
			isValid = true
			break
		}
	}
	if !isValid {
		msg = fmt.Sprintf("role %s is not configured in cluster definition %v", role, controller.DBRoles)
	}
	return isValid, msg
}

func (controller *Controller) getRole(ctx context.Context) (string, error) {
	if controller.actionSvcPorts == nil {
		return "", nil
	}

	var (
		lastOutput string
		err        error
	)

	for _, port := range *controller.actionSvcPorts {
		u := fmt.Sprintf("http://127.0.0.1:%d/role?KB_CONSENSUS_SET_LAST_STDOUT=%s", port, url.QueryEscape(lastOutput))
		lastOutput, err = controller.callAction(ctx, u)
		if err != nil {
			return "", err
		}
	}

	return lastOutput, nil
}

func (controller *Controller) callAction(ctx context.Context, url string) (string, error) {
	// compose http request
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// send http request
	resp, err := controller.client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// parse http response
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("received status code %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), err
}
