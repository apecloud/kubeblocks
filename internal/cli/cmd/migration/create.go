/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package migration

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	migrationv1 "github.com/apecloud/kubeblocks/internal/cli/types/migrationapi"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	AllStepsArr = []string{
		migrationv1.CliStepGlobal.String(),
		migrationv1.CliStepPreCheck.String(),
		migrationv1.CliStepCdc.String(),
		migrationv1.CliStepInitStruct.String(),
		migrationv1.CliStepInitData.String(),
	}
)

const (
	StringBoolTrue  = "true"
	StringBoolFalse = "false"
)

type CreateMigrationOptions struct {
	Template             string                   `json:"template"`
	TaskType             string                   `json:"taskType,omitempty"`
	Source               string                   `json:"source"`
	SourceEndpointModel  EndpointModel            `json:"sourceEndpointModel,omitempty"`
	Sink                 string                   `json:"sink"`
	SinkEndpointModel    EndpointModel            `json:"sinkEndpointModel,omitempty"`
	MigrationObject      []string                 `json:"migrationObject"`
	MigrationObjectModel MigrationObjectModel     `json:"migrationObjectModel,omitempty"`
	Steps                []string                 `json:"steps,omitempty"`
	StepsModel           []string                 `json:"stepsModel,omitempty"`
	Tolerations          []string                 `json:"tolerations,omitempty"`
	TolerationModel      map[string][]interface{} `json:"tolerationModel,omitempty"`
	Resources            []string                 `json:"resources,omitempty"`
	ResourceModel        map[string]interface{}   `json:"resourceModel,omitempty"`
	ServerID             uint32                   `json:"serverId,omitempty"`
	create.BaseOptions
}

func NewMigrationCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateMigrationOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:                          "create name",
		Short:                        "Create a migration task.",
		Example:                      CreateTemplate,
		CueTemplateName:              "migration_template.cue",
		ResourceName:                 types.ResourceMigrationTasks,
		Group:                        types.MigrationAPIGroup,
		Version:                      types.MigrationAPIVersion,
		BaseOptionsObj:               &o.BaseOptions,
		Options:                      o,
		Factory:                      f,
		Validate:                     o.Validate,
		ResourceNameGVRForCompletion: types.MigrationTaskGVR(),
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.Template, "template", "", "Specify migration template, run \"kbcli migration templates\" to show all available migration templates")
			cmd.Flags().StringVar(&o.Source, "source", "", "Set the source database information for migration.such as '{username}:{password}@{connection_address}:{connection_port}/[{database}]'")
			cmd.Flags().StringVar(&o.Sink, "sink", "", "Set the sink database information for migration.such as '{username}:{password}@{connection_address}:{connection_port}/[{database}]")
			cmd.Flags().StringSliceVar(&o.MigrationObject, "migration-object", []string{}, "Set the data objects that need to be migrated,such as '\"db1.table1\",\"db2\"'")
			cmd.Flags().StringSliceVar(&o.Steps, "steps", []string{}, "Set up migration steps,such as: precheck=true,init-struct=true,init-data=true,cdc=true")
			cmd.Flags().StringSliceVar(&o.Tolerations, "tolerations", []string{}, "Tolerations for migration, such as '\"key=engineType,value=pg,operator=Equal,effect=NoSchedule\"'")
			cmd.Flags().StringSliceVar(&o.Resources, "resources", []string{}, "Resources limit for migration, such as '\"cpu=3000m,memory=3Gi\"'")

			util.CheckErr(cmd.MarkFlagRequired("template"))
			util.CheckErr(cmd.MarkFlagRequired("source"))
			util.CheckErr(cmd.MarkFlagRequired("sink"))
			util.CheckErr(cmd.MarkFlagRequired("migration-object"))
		},
	}
	return create.BuildCommand(inputs)
}

func (o *CreateMigrationOptions) Validate() error {
	var err error

	_, err = IsMigrationCrdValidWithDynamic(&o.Dynamic)
	if errors.IsNotFound(err) {
		return fmt.Errorf("datamigration crd is not install")
	} else if err != nil {
		return err
	}

	if o.Template == "" {
		return fmt.Errorf("migration template is needed, use \"kbcli migration templates\" to check and special one")
	}

	errMsgArr := make([]string, 0)
	// Source
	o.SourceEndpointModel = EndpointModel{}
	if err = o.SourceEndpointModel.BuildFromStr(&errMsgArr, o.Source); err != nil {
		return err
	}
	// Sink
	o.SinkEndpointModel = EndpointModel{}
	if err = o.SinkEndpointModel.BuildFromStr(&errMsgArr, o.Sink); err != nil {
		return err
	}

	// MigrationObject
	if err = o.MigrationObjectModel.BuildFromStrs(&errMsgArr, o.MigrationObject); err != nil {
		return err
	}

	// Steps & taskType
	if err = o.BuildWithSteps(&errMsgArr); err != nil {
		return err
	}

	// Tolerations
	if err = o.BuildWithTolerations(); err != nil {
		return err
	}

	// Resources
	if err = o.BuildWithResources(); err != nil {
		return err
	}

	// RuntimeParams
	if err = o.BuildWithRuntimeParams(); err != nil {
		return err
	}

	// Log errors if necessary
	if len(errMsgArr) > 0 {
		return fmt.Errorf(strings.Join(errMsgArr, ";\n"))
	}
	return nil
}

func (o *CreateMigrationOptions) BuildWithSteps(errMsgArr *[]string) error {
	taskType := InitializationAndCdc.String()
	validStepMap, validStepKey := CliStepChangeToStructure()
	enableCdc, enablePreCheck, enableInitStruct, enableInitData := StringBoolTrue, StringBoolTrue, StringBoolTrue, StringBoolTrue
	if len(o.Steps) > 0 {
		for _, step := range o.Steps {
			stepArr := strings.Split(step, "=")
			if len(stepArr) != 2 {
				BuildErrorMsg(errMsgArr, fmt.Sprintf("[%s] in steps setting is invalid", step))
				return nil
			}
			stepName := strings.ToLower(strings.TrimSpace(stepArr[0]))
			enable := strings.ToLower(strings.TrimSpace(stepArr[1]))
			if validStepMap[stepName] == "" {
				BuildErrorMsg(errMsgArr, fmt.Sprintf("[%s] in steps settings is invalid, the name should be one of: (%s)", step, validStepKey))
				return nil
			}
			if enable != StringBoolTrue && enable != StringBoolFalse {
				BuildErrorMsg(errMsgArr, fmt.Sprintf("[%s] in steps settings is invalid, the value should be one of: (true false)", step))
				return nil
			}
			switch stepName {
			case migrationv1.CliStepCdc.String():
				enableCdc = enable
			case migrationv1.CliStepPreCheck.String():
				enablePreCheck = enable
			case migrationv1.CliStepInitStruct.String():
				enableInitStruct = enable
			case migrationv1.CliStepInitData.String():
				enableInitData = enable
			}
		}

		if enableInitData != StringBoolTrue {
			BuildErrorMsg(errMsgArr, "step init-data is needed")
			return nil
		}
		if enableCdc == StringBoolTrue {
			taskType = InitializationAndCdc.String()
		} else {
			taskType = Initialization.String()
		}
	}
	o.TaskType = taskType
	o.StepsModel = []string{}
	if enablePreCheck == StringBoolTrue {
		o.StepsModel = append(o.StepsModel, migrationv1.StepPreCheck.String())
	}
	if enableInitStruct == StringBoolTrue {
		o.StepsModel = append(o.StepsModel, migrationv1.StepStructPreFullLoad.String())
	}
	if enableInitData == StringBoolTrue {
		o.StepsModel = append(o.StepsModel, migrationv1.StepFullLoad.String())
	}
	return nil
}

func (o *CreateMigrationOptions) BuildWithTolerations() error {
	o.TolerationModel = o.buildTolerationOrResources(o.Tolerations)
	tmp := make([]interface{}, 0)
	for _, step := range AllStepsArr {
		if o.TolerationModel[step] == nil {
			o.TolerationModel[step] = tmp
		}
	}
	return nil
}

func (o *CreateMigrationOptions) BuildWithResources() error {
	o.ResourceModel = make(map[string]interface{})
	for k, v := range o.buildTolerationOrResources(o.Resources) {
		if len(v) >= 1 {
			o.ResourceModel[k] = v[0]
		}
	}
	for _, step := range AllStepsArr {
		if o.ResourceModel[step] == nil {
			o.ResourceModel[step] = v1.ResourceList{}
		}
	}
	return nil
}

func (o *CreateMigrationOptions) BuildWithRuntimeParams() error {
	template := migrationv1.MigrationTemplate{}
	templateGvr := types.MigrationTemplateGVR()
	if err := APIResource(&o.BaseOptions.Dynamic, &templateGvr, o.Template, "", &template); err != nil {
		return err
	}

	// Generate random serverId for MySQL type database.Possible values are between 10001 and 2^32-10001
	if template.Spec.Source.DBType == migrationv1.MigrationDBTypeMySQL {
		o.ServerID = o.generateRandomMySQLServerID()
	} else {
		o.ServerID = 10001
	}

	return nil
}

func (o *CreateMigrationOptions) buildTolerationOrResources(raws []string) map[string][]interface{} {
	results := make(map[string][]interface{})
	for _, raw := range raws {
		step := migrationv1.CliStepGlobal.String()
		tmpMap := map[string]interface{}{}
	rawLoop:
		for _, entries := range strings.Split(raw, ",") {
			parts := strings.SplitN(entries, "=", 2)
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if k == "step" {
				switch v {
				case migrationv1.CliStepPreCheck.String(), migrationv1.CliStepCdc.String(), migrationv1.CliStepInitStruct.String(), migrationv1.CliStepInitData.String():
					step = v
				}
				continue rawLoop
			}
			tmpMap[k] = v
		}
		results[step] = append(results[step], tmpMap)
	}
	return results
}

func (o *CreateMigrationOptions) generateRandomMySQLServerID() uint32 {
	rand.Seed(time.Now().UnixNano())
	return uint32(rand.Int63nRange(10001, 1<<32-10001))
}
