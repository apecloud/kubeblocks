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

type RoleAgent struct {
	CheckRoleFailedCount       int
	RoleUnchangedCount         int
	FailedEventReportFrequency int

	// RoleDetectionThreshold is used to set the report duration of role event after role changed,
	// then event controller can always get rolechanged events to maintain pod label accurately
	// in cases of:
	// 1 rolechanged event lost;
	// 2 pod role label deleted or updated incorrectly.
	RoleDetectionThreshold int
	OriRole                string
	Logger                 log.Logger
	DBRoles                map[string]AccessMode
	actionSvcPorts         *[]int
	client                 *http.Client
}

func init() {
	viper.SetDefault("KB_FAILED_EVENT_REPORT_FREQUENCY", defaultFailedEventReportFrequency)
	viper.SetDefault("KB_ROLE_DETECTION_THRESHOLD", defaultRoleDetectionThreshold)
}

func NewRoleAgent(writer io.Writer, prefix string) *RoleAgent {
	return &RoleAgent{
		Logger:         *log.New(writer, prefix, log.LstdFlags),
		actionSvcPorts: &[]int{},
	}
}

func (roleAgent *RoleAgent) Init() error {
	roleAgent.FailedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if roleAgent.FailedEventReportFrequency < 300 {
		roleAgent.FailedEventReportFrequency = 300
	} else if roleAgent.FailedEventReportFrequency > 3600 {
		roleAgent.FailedEventReportFrequency = 3600
	}

	roleAgent.RoleDetectionThreshold = viper.GetInt("KB_ROLE_DETECTION_THRESHOLD")
	if roleAgent.RoleDetectionThreshold < 60 {
		roleAgent.RoleDetectionThreshold = 60
	} else if roleAgent.RoleDetectionThreshold > 300 {
		roleAgent.RoleDetectionThreshold = 300
	}

	val := viper.GetString("KB_SERVICE_ROLES")
	if val != "" {
		if err := json.Unmarshal([]byte(val), &roleAgent.DBRoles); err != nil {
			fmt.Println(errors.Wrap(err, "KB_DB_ROLES env format error").Error())
		}
	}

	actionSvcList := viper.GetString("KB_CONSENSUS_SET_ACTION_SVC_LIST")
	if len(actionSvcList) > 0 {
		err := json.Unmarshal([]byte(actionSvcList), roleAgent.actionSvcPorts)
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
	roleAgent.client = &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}
	return nil
}

func (roleAgent *RoleAgent) ShutDownClient() {
	roleAgent.client.CloseIdleConnections()
}

func (roleAgent *RoleAgent) CheckRoleOps(ctx context.Context) (OpsResult, bool) {
	opsRes := OpsResult{}
	needNotify := false
	role, err := roleAgent.getRole(ctx)

	// OpsResult organized like below:
	// Success: event originalRole role
	// Invalid: event message
	// Failure: event message

	if err != nil {
		roleAgent.Logger.Printf("error executing checkRole: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if roleAgent.CheckRoleFailedCount%roleAgent.FailedEventReportFrequency == 0 {
			roleAgent.Logger.Printf("role checks failed %v times continuously", roleAgent.CheckRoleFailedCount)
		}
		roleAgent.CheckRoleFailedCount++
		return opsRes, true
	}

	roleAgent.CheckRoleFailedCount = 0
	if isValid, message := roleAgent.roleValidate(role); !isValid {
		opsRes["event"] = OperationInvalid
		opsRes["message"] = message
		return opsRes, true
	}

	opsRes["event"] = OperationSuccess
	opsRes["originalRole"] = roleAgent.OriRole
	opsRes["role"] = role

	if roleAgent.OriRole != role {
		roleAgent.OriRole = role
		needNotify = true
	} else {
		roleAgent.RoleUnchangedCount++
	}

	// RoleUnchangedCount is the count of consecutive role unchanged checks.
	// If the role remains unchanged consecutively in RoleDetectionThreshold checks after it has changed,
	// then the roleCheck event will be reported at roleEventReportFrequency so that the event controller
	// can always get relevant roleCheck events in order to maintain the pod label accurately, even in cases
	// of roleChanged events being lost or the pod role label being deleted or updated incorrectly.
	if roleAgent.RoleUnchangedCount < roleAgent.RoleDetectionThreshold && roleAgent.RoleUnchangedCount%roleEventReportFrequency == 0 {
		needNotify = true
		roleAgent.RoleUnchangedCount = 0
	}
	return opsRes, needNotify
}

func (roleAgent *RoleAgent) GetRoleOps(ctx context.Context) OpsResult {
	opsRes := OpsResult{}

	role, err := roleAgent.getRole(ctx)
	if err != nil {
		roleAgent.Logger.Printf("error executing getRole: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if roleAgent.CheckRoleFailedCount%roleAgent.FailedEventReportFrequency == 0 {
			roleAgent.Logger.Printf("getRole failed %v times continuously", roleAgent.CheckRoleFailedCount)
		}
		roleAgent.CheckRoleFailedCount++
		return opsRes
	}

	roleAgent.CheckRoleFailedCount = 0

	opsRes["event"] = OperationSuccess
	opsRes["role"] = role
	return opsRes
}

// Component may have some internal roles that need not be exposed to end user,
// and not configured in cluster definition, e.g. ETCD's Candidate.
// roleValidate is used to filter the internal roles and decrease the number
// of report events to reduce the possibility of event conflicts.
func (roleAgent *RoleAgent) roleValidate(role string) (bool, string) {

	// do not validate when db roles setting is missing
	if len(roleAgent.DBRoles) == 0 {
		return true, ""
	}

	var msg string
	isValid := false
	for r := range roleAgent.DBRoles {
		if strings.EqualFold(r, role) {
			isValid = true
			break
		}
	}
	if !isValid {
		msg = fmt.Sprintf("role %s is not configured in cluster definition %v", role, roleAgent.DBRoles)
	}
	return isValid, msg
}

func (roleAgent *RoleAgent) getRole(ctx context.Context) (string, error) {
	if roleAgent.actionSvcPorts == nil {
		return "", nil
	}

	var (
		lastOutput string
		err        error
	)

	for _, port := range *roleAgent.actionSvcPorts {
		u := fmt.Sprintf("http://127.0.0.1:%d/role?KB_CONSENSUS_SET_LAST_STDOUT=%s", port, url.QueryEscape(lastOutput))
		lastOutput, err = roleAgent.callAction(ctx, u)
		if err != nil {
			return "", err
		}
	}

	return lastOutput, nil
}

func (roleAgent *RoleAgent) callAction(ctx context.Context, url string) (string, error) {
	// compose http request
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// send http request
	resp, err := roleAgent.client.Do(request)
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
