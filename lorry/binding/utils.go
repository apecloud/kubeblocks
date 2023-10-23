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
	"os"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	ctlruntime "sigs.k8s.io/controller-runtime"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/lorry/component"
	. "github.com/apecloud/kubeblocks/lorry/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type RedisEntry struct {
	Key  string `json:"key"`
	Data []byte `json:"data,omitempty"`
}

type opsMetadata struct {
	Operation OperationKind `json:"operation,omitempty"`
	StartTime string        `json:"startTime,omitempty"`
	EndTime   string        `json:"endTime,omitempty"`
	Extra     string        `json:"extra,omitempty"`
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
type objectParser[T customizedObjType] func(req *ProbeRequest, object *T) error

func ExecuteObject[T customizedObjType](ctx context.Context, ops BaseInternalOps, req *ProbeRequest,
	opsKind OperationKind, sqlTplRend cmdRender[T], msgTplRend cmdRender[T], object T) (OpsResult, error) {
	var (
		result = OpsResult{}
		err    error
	)

	metadata := opsMetadata{Operation: opsKind, StartTime: getAndFormatNow()}

	sql := sqlTplRend(object)
	metadata.Extra = sql
	ops.GetLogger().Info("ExecObject with cmd", "cmd", sql)

	if _, err = ops.InternalExec(ctx, sql); err != nil {
		return opsTerminateOnErr(result, metadata, err)
	}
	return opsTerminateOnSucc(result, metadata, msgTplRend(object))
}

func QueryObject[T customizedObjType](ctx context.Context, ops BaseInternalOps, req *ProbeRequest,
	opsKind OperationKind, sqlTplRend cmdRender[T], dataProcessor resultRender[T], object T) (OpsResult, error) {
	var (
		result = OpsResult{}
		err    error
	)

	metadata := opsMetadata{Operation: opsKind, StartTime: getAndFormatNow()}

	sql := sqlTplRend(object)
	metadata.Extra = sql
	ops.GetLogger().Info("QueryObject() with cmd", "cmd", sql)

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

func ParseObjFromRequest[T customizedObjType](req *ProbeRequest, parse objectParser[T], validator objectValidator[T], object *T) error {
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

func DefaultUserInfoParser(req *ProbeRequest, object *UserInfo) error {
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
	result[RespFieldEvent] = RespEveSucc
	result[RespFieldMessage] = msg
	result[RespTypMeta] = metadata
	return result, nil
}

func opsTerminateOnErr(result OpsResult, metadata opsMetadata, err error) (OpsResult, error) {
	metadata.EndTime = getAndFormatNow()
	result[RespFieldEvent] = RespEveFail
	result[RespFieldMessage] = err.Error()
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

func SentProbeEvent(ctx context.Context, opsResult OpsResult, resp *ProbeResponse, log logr.Logger) {
	log.Info(fmt.Sprintf("send event: %v", opsResult))
	roleUpdateMechanism := workloads.DirectAPIServerEventUpdate
	if viper.IsSet(RSMRoleUpdateMechanismVarName) {
		roleUpdateMechanism = workloads.RoleUpdateMechanism(viper.GetString(RSMRoleUpdateMechanismVarName))
	}
	switch roleUpdateMechanism {
	case workloads.ReadinessProbeEventUpdate:
		resp.Metadata[StatusCode] = OperationFailedHTTPCode
	case workloads.DirectAPIServerEventUpdate:
		event, err := createProbeEvent(opsResult)
		if err != nil {
			log.Error(err, "generate event failed")
			return
		}

		_ = sendEvent(ctx, log, event)
	default:
		log.Info(fmt.Sprintf("no event sent, RoleUpdateMechanism: %s", roleUpdateMechanism))
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
  fieldPath: spec.containers{kb-checkrole}
  kind: Pod
  name: {{ .PodName }}
  namespace: {{ .Namespace }}
reason: RoleChanged
type: Normal
source:
  component: lorry
`

	// get pod object
	podName := os.Getenv(constant.KBEnvPodName)
	podUID := os.Getenv(constant.KBEnvPodUID)
	nodeName := os.Getenv(constant.KBEnvNodeName)
	namespace := os.Getenv(constant.KBEnvNamespace)
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
	event.Reason = string(opsResult["operation"].(OperationKind))
	event.FirstTimestamp = metav1.Now()
	event.LastTimestamp = metav1.Now()
	event.EventTime = metav1.NowMicro()
	event.ReportingController = "lorry"
	event.ReportingInstance = podName
	event.Action = string(opsResult["operation"].(OperationKind))

	return event, nil
}

func sendEvent(ctx context.Context, log logr.Logger, event *corev1.Event) error {
	config, err := ctlruntime.GetConfig()
	if err != nil {
		log.Error(err, "get k8s client config failed")
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "k8s client create failed")
		return err
	}
	namespace := os.Getenv(constant.KBEnvNamespace)
	for i := 0; i < 30; i++ {
		_, err = clientset.CoreV1().Events(namespace).Create(ctx, event, metav1.CreateOptions{})
		if err == nil {
			break
		}
		log.Error(err, "send event failed")
		time.Sleep(10 * time.Second)
	}
	return err
}

func StartupCheckWraper(manager component.DBManager, operation Operation) Operation {
	return func(ctx context.Context, request *ProbeRequest, response *ProbeResponse) (OpsResult, error) {
		if !manager.IsDBStartupReady() {
			opsRes := OpsResult{"event": OperationFailed, "message": "db not ready"}
			return opsRes, nil
		}

		return operation(ctx, request, response)
	}
}
