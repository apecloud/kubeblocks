package migration

import (
	"context"
	"fmt"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	migrationv1 "github.com/apecloud/kubeblocks/internal/cli/types/migrationapi"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"strings"
)

const (
	MigrationTaskLabel          = "datamigration.apecloud.io/migrationtask"
	MigrationTaskStepAnnotation = "datamigration.apecloud.io/step"
	SerialJobOrderAnnotation    = "common.apecloud.io/serial_job_order"
)

// Endpoint

type EndpointModel struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
	Address  string `json:"address"`
	// +optional
	Database string `json:"databaseName,omitempty"`
}

func (e *EndpointModel) BuildFromStr(msgArr *[]string, endpointStr string) error {
	if endpointStr == "" {
		BuildErrorMsg(msgArr, "endpoint string can not be empty")
		return nil
	}
	endpointStr = strings.TrimSpace(endpointStr)
	accountUrlPair := strings.Split(endpointStr, "@")
	if len(accountUrlPair) != 2 {
		BuildErrorMsg(msgArr, "endpoint maybe does not contain account info")
		return nil
	}
	accountPair := strings.Split(accountUrlPair[0], ":")
	if len(accountPair) != 2 {
		BuildErrorMsg(msgArr, "the account info in endpoint is invalid, should be like \"user:123456\"")
		return nil
	}
	e.UserName = accountPair[0]
	e.Password = accountPair[1]
	if strings.LastIndex(accountUrlPair[1], "/") != -1 {
		addressDatabasePair := strings.Split(accountUrlPair[1], "/")
		e.Address = strings.Join(addressDatabasePair[:len(addressDatabasePair)-1], "/")
		e.Database = addressDatabasePair[len(addressDatabasePair)-1]
	} else {
		e.Address = accountUrlPair[1]
	}
	return nil
}

// Migration Object

type MigrationObjectModel struct {
	WhiteList []DbObjectExpress `json:"whiteList"`
}

type DbObjectExpress struct {
	SchemaName string `json:"schemaName"`
	// +optional
	IsAll bool `json:"isAll"`
	// +optional
	TableList []TableObjectExpress `json:"tableList"`
}

type TableObjectExpress struct {
	TableName string `json:"tableName"`
	// +optional
	IsAll bool `json:"isAll"`
}

func (m *MigrationObjectModel) BuildFromStrs(errMsgArr *[]string, objStrs []string) error {
	if len(objStrs) <= 0 {
		BuildErrorMsg(errMsgArr, "migration object can not be empty")
		return nil
	}
	for _, str := range objStrs {
		msg := ""
		if str == "" {
			msg = "the database or database.table in migration object can not be empty"
		}
		dbTablePair := strings.Split(str, ".")
		if len(dbTablePair) > 2 {
			msg = fmt.Sprintf("[%s] is not a valid database or database.table", str)
		}
		if msg != "" {
			BuildErrorMsg(errMsgArr, msg)
			return nil
		}
		if len(dbTablePair) == 1 {
			m.WhiteList = append(m.WhiteList, DbObjectExpress{
				SchemaName: str,
				IsAll:      true,
			})
		} else {
			dbObjPoint, err := m.ContainSchema(dbTablePair[0])
			if err != nil {
				return err
			}
			if dbObjPoint != nil {
				dbObjPoint.TableList = append(dbObjPoint.TableList, TableObjectExpress{
					TableName: dbTablePair[1],
					IsAll:     true,
				})
			} else {
				m.WhiteList = append(m.WhiteList, DbObjectExpress{
					SchemaName: dbTablePair[0],
					TableList: []TableObjectExpress{{
						TableName: dbTablePair[1],
						IsAll:     true,
					}},
				})
			}
		}
	}
	return nil
}

func (m *MigrationObjectModel) ContainSchema(schemaName string) (*DbObjectExpress, error) {
	for i := 0; i < len(m.WhiteList); i++ {
		if m.WhiteList[i].SchemaName == schemaName {
			return &m.WhiteList[i], nil
		}
	}
	return nil, nil
}

func CliStepChangeToStructure() (map[string]string, []string) {
	validStepMap := map[string]string{
		migrationv1.CliStepPreCheck.String():   migrationv1.CliStepPreCheck.String(),
		migrationv1.CliStepInitStruct.String(): migrationv1.CliStepInitStruct.String(),
		migrationv1.CliStepInitData.String():   migrationv1.CliStepInitData.String(),
		migrationv1.CliStepCdc.String():        migrationv1.CliStepCdc.String(),
	}
	validStepKey := make([]string, 0)
	for k := range validStepMap {
		validStepKey = append(validStepKey, k)
	}
	return validStepMap, validStepKey
}

type TaskTypeEnum string

const (
	Initialization       TaskTypeEnum = "initialization"
	InitializationAndCdc TaskTypeEnum = "initialization-and-cdc" // default value
)

func (s TaskTypeEnum) String() string {
	return string(s)
}

func IsMigrationCrdValidWithDynamic(dynamic *dynamic.Interface) (bool, error) {
	resource := types.CustomResourceDefinitionGVR()
	if err := ApiResource(dynamic, &resource, "migrationtasks.datamigration.apecloud.io", "", nil); err != nil {
		return false, err
	}
	if err := ApiResource(dynamic, &resource, "migrationtemplates.datamigration.apecloud.io", "", nil); err != nil {
		return false, err
	}
	if err := ApiResource(dynamic, &resource, "serialjobs.common.apecloud.io", "", nil); err != nil {
		return false, err
	}
	return true, nil
}

func IsMigrationCrdValidWithFactory(factory *cmdutil.Factory) (bool, error) {
	dymatic, err := (*factory).DynamicClient()
	if err != nil {
		return false, err
	}
	return IsMigrationCrdValidWithDynamic(&dymatic)
}

func ApiResource(dynamic *dynamic.Interface, resource *schema.GroupVersionResource, name string, namespace string, res interface{}) error {
	obj, err := (*dynamic).Resource(*resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{}, "")
	if err != nil {
		return err
	}
	if res != nil {
		return runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, res)
	}
	return nil
}

func TimeFormat(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return util.TimeTimeFormat(t.Time)
}

func BuildErrorMsg(msgArr *[]string, msg string) {
	if *msgArr == nil {
		*msgArr = make([]string, 1)
	}
	*msgArr = append(*msgArr, msg)
}

func BuildInitializationStepsOrder(task *migrationv1.MigrationTask, template *migrationv1.MigrationTemplate) []string {
	stepMap := make(map[string]string)
	for _, taskStep := range task.Spec.Initialization.Steps {
		stepMap[taskStep.String()] = taskStep.String()
	}
	resultArr := make([]string, 0)
	for _, stepModel := range template.Spec.Initialization.Steps {
		if stepMap[stepModel.Step.String()] != "" {
			resultArr = append(resultArr, stepModel.Step.CliString())
		}
	}
	return resultArr
}
