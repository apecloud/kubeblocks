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

package binding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/dlclark/regexp2"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	mapset "github.com/deckarep/golang-set"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type UserInfo struct {
	UserName string `json:"userName"`
	Password string `json:"password,omitempty"`
	Expired  string `json:"expired,omitempty"`
	RoleName string `json:"roleName,omitempty"`
}

type RedisEntry struct {
	Key  string `json:"key"`
	Data []byte `json:"data,omitempty"`
}

type opsMetadata struct {
	Operation bindings.OperationKind `json:"operation,omitempty"`
	StartTime string                 `json:"startTime,omitempty"`
	EndTime   string                 `json:"endTime,omitempty"`
	Extra     string                 `json:"extra,omitempty"`
}

// UserDefinedObjectType defines the interface for User Defined Objects.
type customizedObjType interface {
	UserInfo | RedisEntry
}

// CmdRender defines the interface to render a statement from given object.
type cmdRender[T customizedObjType] func(object T) string

// resultRender defines the interface to render the data from database.
type resultRender[T customizedObjType] func(interface{}) (interface{}, error)

// objectValidator defines the interface to validate the User Defined Object.
type objectValidator[T customizedObjType] func(object T) error

// objectParser defines the interface to parse the User Defined Object from request.
type objectParser[T customizedObjType] func(req *bindings.InvokeRequest, object *T) error

func ExecuteObject[T customizedObjType](ctx context.Context, ops BaseInternalOps, req *bindings.InvokeRequest,
	opsKind bindings.OperationKind, sqlTplRend cmdRender[T], msgTplRend cmdRender[T], object T) (OpsResult, error) {
	var (
		result = OpsResult{}
		err    error
	)

	metadata := opsMetadata{Operation: opsKind, StartTime: getAndFormatNow()}

	sql := sqlTplRend(object)
	metadata.Extra = sql
	ops.GetLogger().Debugf("ExecObject with cmd: %s", sql)

	if _, err = ops.InternalExec(ctx, sql); err != nil {
		return opsTerminateOnErr(result, metadata, err)
	}
	return opsTerminateOnSucc(result, metadata, msgTplRend(object))
}

func QueryObject[T customizedObjType](ctx context.Context, ops BaseInternalOps, req *bindings.InvokeRequest,
	opsKind bindings.OperationKind, sqlTplRend cmdRender[T], dataProcessor resultRender[T], object T) (OpsResult, error) {
	var (
		result = OpsResult{}
		err    error
	)

	metadata := opsMetadata{Operation: opsKind, StartTime: getAndFormatNow()}

	sql := sqlTplRend(object)
	metadata.Extra = sql
	ops.GetLogger().Debugf("QueryObject() with cmd: %s", sql)

	jsonData, err := ops.InternalQuery(ctx, sql)
	if err != nil {
		return opsTerminateOnErr(result, metadata, err)
	}

	if dataProcessor == nil {
		return opsTerminateOnSucc(result, metadata, string(jsonData))
	}

	if ret, err := dataProcessor(jsonData); err != nil {
		return opsTerminateOnErr(result, metadata, err)
	} else {
		return opsTerminateOnSucc(result, metadata, ret)
	}
}

func ParseObjFromRequest[T customizedObjType](req *bindings.InvokeRequest, parse objectParser[T], validator objectValidator[T], object *T) error {
	if req == nil {
		return fmt.Errorf("no request provided")
	}
	if parse != nil {
		if err := parse(req, object); err != nil {
			return err
		}
	}
	if validator != nil {
		if err := validator(*object); err != nil {
			return err
		}
	}
	return nil
}

func DefaultUserInfoParser(req *bindings.InvokeRequest, object *UserInfo) error {
	if req == nil || req.Metadata == nil {
		return fmt.Errorf("no metadata provided")
	} else if jsonData, err := json.Marshal(req.Metadata); err != nil {
		return err
	} else if err = json.Unmarshal(jsonData, object); err != nil {
		return err
	}
	return nil
}

func UserNameValidator(user UserInfo) error {
	if len(user.UserName) == 0 {
		return ErrNoUserName
	}
	return nil
}

func UserNameAndPasswdValidator(user UserInfo) error {
	if len(user.UserName) == 0 {
		return ErrNoUserName
	}
	if len(user.Password) == 0 {
		return ErrNoPassword
	}
	return nil
}

func UserNameAndRoleValidator(user UserInfo) error {
	if len(user.UserName) == 0 {
		return ErrNoUserName
	}
	if len(user.RoleName) == 0 {
		return ErrNoRoleName
	}
	roles := []RoleType{ReadOnlyRole, ReadWriteRole, SuperUserRole}
	for _, role := range roles {
		if role.EqualTo(user.RoleName) {
			return nil
		}
	}
	return ErrInvalidRoleName
}

func getAndFormatNow() string {
	return time.Now().Format(time.RFC3339Nano)
}

func opsTerminateOnSucc(result OpsResult, metadata opsMetadata, msg interface{}) (OpsResult, error) {
	metadata.EndTime = getAndFormatNow()
	result[RespTypEve] = RespEveSucc
	result[RespTypMsg] = msg
	result[RespTypMeta] = metadata
	return result, nil
}

func opsTerminateOnErr(result OpsResult, metadata opsMetadata, err error) (OpsResult, error) {
	metadata.EndTime = getAndFormatNow()
	result[RespTypEve] = RespEveFail
	result[RespTypMsg] = err.Error()
	result[RespTypMeta] = metadata
	return result, nil
}

func SortRoleByWeight(r1, r2 RoleType) bool {
	return int(r1.GetWeight()) > int(r2.GetWeight())
}

func String2RoleType(roleName string) RoleType {
	if SuperUserRole.EqualTo(roleName) {
		return SuperUserRole
	}
	if ReadWriteRole.EqualTo(roleName) {
		return ReadWriteRole
	}
	if ReadOnlyRole.EqualTo(roleName) {
		return ReadOnlyRole
	}
	if NoPrivileges.EqualTo(roleName) {
		return NoPrivileges
	}
	return CustomizedRole
}

func SentProbeEvent(ctx context.Context, opsResult OpsResult, log logger.Logger) {
	log.Infof("send event: %v", opsResult)
	event, err := createProbeEvent(opsResult)
	if err != nil {
		log.Infof("generate event failed: %v", err)
		return
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Infof("get k8s client config failed: %v", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Infof("k8s client create failed: %v", err)
		return
	}
	namespace := os.Getenv("KB_NAMESPACE")
	for i := 0; i < 3; i++ {
		_, err = clientset.CoreV1().Events(namespace).Create(ctx, event, metav1.CreateOptions{})
		if err == nil {
			break
		}
		log.Infof("send event failed: %v", err)
	}
}

func createProbeEvent(opsResult OpsResult) (*corev1.Event, error) {
	eventTmpl := `
apiVersion: v1
kind: Event
metadata:
  name: {{ .PodName }}.{{ .EventSeq }}
  namespace: {{ .Namespace }}
involvedObject:
  apiVersion: v1
  fieldPath: spec.containers{sqlchannel}
  kind: Pod
  name: {{ .PodName }}
  namespace: {{ .Namespace }}
reason: RoleChanged
type: Normal
source:
  component: sqlchannel
`

	// get pod object
	podName := os.Getenv("KB_POD_NAME")
	podUID := os.Getenv("KB_POD_UID")
	nodeName := os.Getenv("KB_NODENAME")
	namespace := os.Getenv("KB_NAMESPACE")
	msg, _ := json.Marshal(opsResult)
	seq := rand.String(16)
	roleValue := map[string]string{
		"PodName":   podName,
		"Namespace": namespace,
		"EventSeq":  seq,
	}
	tmpl, err := template.New("event-tmpl").Parse(eventTmpl)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, roleValue)
	if err != nil {
		return nil, err
	}

	event := &corev1.Event{}
	_, _, err = scheme.Codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, event)
	if err != nil {
		return nil, err
	}
	event.Message = string(msg)
	event.InvolvedObject.UID = types.UID(podUID)
	event.Source.Host = nodeName
	event.Reason = string(opsResult["operation"].(bindings.OperationKind))
	event.FirstTimestamp = metav1.Now()
	event.LastTimestamp = metav1.Now()

	return event, nil
}

func MaxInt64(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

type PGStandby struct {
	Types   string
	Amount  int
	Members mapset.Set
	HasStar bool
}

func ParsePGSyncStandby(standbyRow string) (*PGStandby, error) {
	pattern := `(?P<first> [fF][iI][rR][sS][tT] )
				|(?P<any> [aA][nN][yY] )
				|(?P<space> \s+ )
				|(?P<ident> [A-Za-z_][A-Za-z_0-9\$]* )
				|(?P<double_quote> " (?: [^"]+ | "" )* " )
				|(?P<star> [*] )
				|(?P<num> \d+ )
				|(?P<comma> , )
				|(?P<parenthesis_start> \( )
				|(?P<parenthesis_end> \) )
				|(?P<JUNK> . ) `
	patterns := []string{
		`(?P<first> [fF][iI][rR][sS][tT]) `,
		`(?P<any> [aA][nN][yY]) `,
		`(?P<space> \s+ )`,
		`(?P<ident> [A-Za-z_][A-Za-z_0-9\$]* )`,
		`(?P<double_quote> "(?:[^"]+|"")*") `,
		`(?P<star> [*] )`,
		`(?P<num> \d+ )`,
		`(?P<comma> , )`,
		`(?P<parenthesis_start> \( )`,
		`(?P<parenthesis_end> \) )`,
		`(?P<JUNK> .) `,
	}
	result := &PGStandby{
		Members: mapset.NewSet(),
	}

	rs := make([]*regexp2.Regexp, len(patterns))
	var patternPrefix string
	for i, p := range patterns {
		if i != 0 {
			patternPrefix += `|`
		}
		patternPrefix += p
		rs[i] = regexp2.MustCompile(patternPrefix, regexp2.IgnorePatternWhitespace+regexp2.RE2)
	}

	r := regexp2.MustCompile(pattern, regexp2.RE2+regexp2.IgnorePatternWhitespace)
	groupNames := r.GetGroupNames()

	match, err := r.FindStringMatch(standbyRow)
	if err != nil {
		return nil, err
	}

	var matches [][]string
	start := 0
	for match != nil {
		num := getMatchLastGroupNumber(rs, standbyRow, match.String(), start)
		if groupNames[num+2] != "space" {
			matches = append(matches, []string{groupNames[num+2], match.String(), strconv.FormatInt(int64(start), 10)})
		}
		start = match.Index + match.Length

		match, err = r.FindNextMatch(match)
		if err != nil {
			return nil, err
		}
	}

	length := len(matches)
	var syncList [][]string
	if matches[0][0] == "any" && matches[1][0] == "num" && matches[2][0] == "parenthesis_start" && matches[length-1][0] == "parenthesis_end" {
		result.Types = "quorum"
		amount, err := strconv.Atoi(matches[1][1])
		if err != nil {
			amount = 0
		}
		result.Amount = amount
		syncList = matches[3 : length-1]
	} else if matches[0][0] == "first" && matches[1][0] == "num" && matches[2][0] == "parenthesis_start" && matches[length-1][0] == "parenthesis_end" {
		result.Types = "priority"
		amount, err := strconv.Atoi(matches[1][1])
		if err != nil {
			amount = 0
		}
		result.Amount = amount
		syncList = matches[3 : length-1]
	} else if matches[0][0] == "num" && matches[1][0] == "parenthesis_start" && matches[length-1][0] == "parenthesis_end" {
		result.Types = "priority"
		amount, err := strconv.Atoi(matches[0][1])
		if err != nil {
			amount = 0
		}
		result.Amount = amount
		syncList = matches[2 : length-1]
	} else {
		result.Types = "priority"
		result.Amount = 1
		syncList = matches
	}

	for i, sync := range syncList {
		if i%2 == 1 { // odd elements are supposed to be commas
			if len(syncList) == i+1 {
				return nil, errors.Errorf("Unparseable synchronous_standby_names value: Unexpected token %s %s at %s", sync[0], sync[1], sync[2])
			} else if sync[0] != "comma" {
				return nil, errors.Errorf("Unparseable synchronous_standby_names value: Got token %s %s while expecting comma at %s", sync[0], sync[1], sync[2])
			}
		} else if slices.Contains([]string{"ident", "first", "any"}, sync[0]) {
			result.Members.Add(sync[1])
		} else if sync[0] == "star" {
			result.Members.Add(sync[1])
			result.HasStar = true
		} else if sync[0] == "double_quote" {
			//TODO:check
			result.Members.Add(strings.Replace(sync[1][1:len(sync)-1], `""`, `"`, -1))
		} else {
			return nil, errors.Errorf("Unparseable synchronous_standby_names value: Unexpected token %s %s at %s", sync[0], sync[1], sync[2])
		}
	}

	return result, nil
}

func getMatchLastGroupNumber(rs []*regexp2.Regexp, str string, substr string, start int) int {
	for i := len(rs) - 1; i >= 0; i-- {
		match, err := rs[i].FindStringMatchStartingAt(str, start)
		if match == nil || err != nil {
			return i
		}
		if match.String() != substr {
			return i
		}
	}

	return -1
}

func ParsePgLsn(str string) int64 {
	list := strings.Split(str, "/")
	prefix, _ := strconv.ParseInt(list[0], 16, 64)
	suffix, _ := strconv.ParseInt(list[1], 16, 64)
	return prefix*0x100000000 + suffix
}
