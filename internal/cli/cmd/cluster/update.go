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

package cluster

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/configuration"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

var clusterUpdateExample = templates.Examples(`
	# update cluster mycluster termination policy to Delete
	kbcli cluster update mycluster --termination-policy=Delete

	# enable cluster monitor
	kbcli cluster update mycluster --monitor=true

    # enable all logs
	kbcli cluster update mycluster --enable-all-logs=true

    # update cluster topology keys and affinity
	kbcli cluster update mycluster --topology-keys=kubernetes.io/hostname --pod-anti-affinity=Required

	# update cluster tolerations
	kbcli cluster update mycluster --tolerations='"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'

	# edit cluster
	kbcli cluster update mycluster --edit

	# enable cluster monitor and edit
    # kbcli cluster update mycluster --monitor=true --edit

	# enable cluster auto backup
	kbcli cluster update mycluster --backup-enabled=true
	
	# update cluster backup retention period
	kbcli cluster update mycluster --backup-retention-period=1d

	# update cluster backup method
	kbcli cluster update mycluster --backup-method=snapshot

	# update cluster backup cron expression
	kbcli cluster update mycluster --backup-cron-expression="0 0 * * *"

	# update cluster backup starting deadline minutes
	kbcli cluster update mycluster --backup-starting-deadline-minutes=10

	# update cluster backup repo name
	kbcli cluster update mycluster --backup-repo-name=repo1

	# update cluster backup pitr enabled
	kbcli cluster update mycluster --pitr-enabled=true
`)

type updateOptions struct {
	namespace string
	dynamic   dynamic.Interface
	cluster   *appsv1alpha1.Cluster

	UpdatableFlags
	*patch.Options
}

func NewUpdateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &updateOptions{Options: patch.NewOptions(f, streams, types.ClusterGVR())}
	o.Options.OutputOperation = func(didPatch bool) string {
		if didPatch {
			return "updated"
		}
		return "updated (no change)"
	}

	cmd := &cobra.Command{
		Use:               "update NAME",
		Short:             "Update the cluster settings, such as enable or disable monitor or log.",
		Example:           clusterUpdateExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
			util.CheckErr(o.Run(cmd))
		},
	}
	o.UpdatableFlags.addFlags(cmd)
	o.Options.AddFlags(cmd)

	return cmd
}

func (o *updateOptions) complete(cmd *cobra.Command, args []string) error {
	var err error
	if len(args) == 0 {
		return makeMissingClusterNameErr()
	}
	if len(args) > 1 {
		return fmt.Errorf("only support to update one cluster")
	}
	o.Names = args

	// record the flags that been set by user
	var flags []*pflag.Flag
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		flags = append(flags, flag)
	})

	// nothing to do
	if len(flags) == 0 {
		return nil
	}

	if o.namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}
	if o.dynamic, err = o.Factory.DynamicClient(); err != nil {
		return err
	}
	return o.buildPatch(flags)
}

func (o *updateOptions) buildPatch(flags []*pflag.Flag) error {
	var err error
	type buildFn func(obj map[string]interface{}, v pflag.Value, field string) error

	buildFlagObj := func(obj map[string]interface{}, v pflag.Value, field string) error {
		var val interface{}
		switch v.Type() {
		case "string":
			val = v.String()
		case "stringArray", "stringSlice":
			val = v.(pflag.SliceValue).GetSlice()
		case "stringToString":
			valMap := make(map[string]interface{}, 0)
			vStr := strings.Trim(v.String(), "[]")
			if len(vStr) > 0 {
				r := csv.NewReader(strings.NewReader(vStr))
				ss, err := r.Read()
				if err != nil {
					return err
				}
				for _, pair := range ss {
					kv := strings.SplitN(pair, "=", 2)
					if len(kv) != 2 {
						return fmt.Errorf("%s must be formatted as key=value", pair)
					}
					valMap[kv[0]] = kv[1]
				}
			}
			val = valMap
		}
		return unstructured.SetNestedField(obj, val, field)
	}

	buildTolObj := func(obj map[string]interface{}, v pflag.Value, field string) error {
		tolerations, err := util.BuildTolerations(o.TolerationsRaw)
		if err != nil {
			return err
		}
		return unstructured.SetNestedField(obj, tolerations, field)
	}

	buildComps := func(obj map[string]interface{}, v pflag.Value, field string) error {
		return o.buildComponents(field, v.String())
	}

	buildBackup := func(obj map[string]interface{}, v pflag.Value, field string) error {
		return o.buildBackup(field, v.String())
	}

	spec := map[string]interface{}{}
	affinity := map[string]interface{}{}
	type filedObj struct {
		field string
		obj   map[string]interface{}
		fn    buildFn
	}

	flagFieldMapping := map[string]*filedObj{
		"termination-policy": {field: "terminationPolicy", obj: spec, fn: buildFlagObj},
		"pod-anti-affinity":  {field: "podAntiAffinity", obj: affinity, fn: buildFlagObj},
		"topology-keys":      {field: "topologyKeys", obj: affinity, fn: buildFlagObj},
		"node-labels":        {field: "nodeLabels", obj: affinity, fn: buildFlagObj},
		"tenancy":            {field: "tenancy", obj: affinity, fn: buildFlagObj},

		// tolerations
		"tolerations": {field: "tolerations", obj: spec, fn: buildTolObj},

		// monitor and logs
		"monitoring-interval": {field: "monitor", obj: nil, fn: buildComps},
		"enable-all-logs":     {field: "enable-all-logs", obj: nil, fn: buildComps},

		// backup config
		"backup-enabled":                   {field: "enabled", obj: nil, fn: buildBackup},
		"backup-retention-period":          {field: "retentionPeriod", obj: nil, fn: buildBackup},
		"backup-method":                    {field: "method", obj: nil, fn: buildBackup},
		"backup-cron-expression":           {field: "cronExpression", obj: nil, fn: buildBackup},
		"backup-starting-deadline-minutes": {field: "startingDeadlineMinutes", obj: nil, fn: buildBackup},
		"backup-repo-name":                 {field: "repoName", obj: nil, fn: buildBackup},
		"pitr-enabled":                     {field: "pitrEnabled", obj: nil, fn: buildBackup},
	}

	for _, flag := range flags {
		if f, ok := flagFieldMapping[flag.Name]; ok {
			if err = f.fn(f.obj, flag.Value, f.field); err != nil {
				return err
			}
		}
	}

	if len(affinity) > 0 {
		if err = unstructured.SetNestedField(spec, affinity, "affinity"); err != nil {
			return err
		}
	}

	if o.cluster != nil {
		// if update the backup config, the backup method must have value
		if o.cluster.Spec.Backup != nil {
			defaultBackupMethod, backupMethodMap, err := o.getBackupMethodsFromBackupPolicy()
			if err != nil {
				return err
			}
			if o.cluster.Spec.Backup.Method == "" {
				o.cluster.Spec.Backup.Method = defaultBackupMethod
			}
			if _, ok := backupMethodMap[o.cluster.Spec.Backup.Method]; !ok {
				return fmt.Errorf("backup method %s is not supported, please view the supported backup methods by `kbcli cd describe %s`", o.cluster.Spec.Backup.Method, o.cluster.Spec.ClusterDefRef)
			}
		}

		data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&o.cluster.Spec)
		if err != nil {
			return err
		}

		if err = unstructured.SetNestedField(spec, data["componentSpecs"], "componentSpecs"); err != nil {
			return err
		}

		if err = unstructured.SetNestedField(spec, data["backup"], "backup"); err != nil {
			return err
		}
	}

	obj := unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": spec,
		},
	}
	bytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}
	o.Patch = string(bytes)
	return nil
}

func (o *updateOptions) buildComponents(field string, val string) error {
	if o.cluster == nil {
		c, err := cluster.GetClusterByName(o.dynamic, o.Names[0], o.namespace)
		if err != nil {
			return err
		}
		o.cluster = c
	}

	switch field {
	case "monitor":
		return o.updateMonitor(val)
	case "enable-all-logs":
		return o.updateEnabledLog(val)
	default:
		return nil
	}
}

func (o *updateOptions) buildBackup(field string, val string) error {
	if o.cluster == nil {
		c, err := cluster.GetClusterByName(o.dynamic, o.Names[0], o.namespace)
		if err != nil {
			return err
		}
		o.cluster = c
	}
	if o.cluster.Spec.Backup == nil {
		o.cluster.Spec.Backup = &appsv1alpha1.ClusterBackup{}
	}

	switch field {
	case "enabled":
		return o.updateBackupEnabled(val)
	case "retentionPeriod":
		return o.updateBackupRetentionPeriod(val)
	case "method":
		return o.updateBackupMethod(val)
	case "cronExpression":
		return o.updateBackupCronExpression(val)
	case "startingDeadlineMinutes":
		return o.updateBackupStartingDeadlineMinutes(val)
	case "repoName":
		return o.updateBackupRepoName(val)
	case "pitrEnabled":
		return o.updateBackupPitrEnabled(val)
	default:
		return nil
	}
}

func (o *updateOptions) updateEnabledLog(val string) error {
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}

	// update --enabled-all-logs=false for all components
	if !boolVal {
		for index := range o.cluster.Spec.ComponentSpecs {
			o.cluster.Spec.ComponentSpecs[index].EnabledLogs = nil
		}
		return nil
	}

	// update --enabled-all-logs=true for all components
	cd, err := cluster.GetClusterDefByName(o.dynamic, o.cluster.Spec.ClusterDefRef)
	if err != nil {
		return err
	}
	// set --enabled-all-logs at cluster components
	setEnableAllLogs(o.cluster, cd)
	if err = o.reconfigureLogVariables(o.cluster, cd); err != nil {
		return errors.Wrap(err, "failed to reconfigure log variables of target cluster")
	}
	return nil
}

const logsBlockName = "logsBlock"
const logsTemplateName = "template-logs-block"
const topTPLLogsObject = "component"
const defaultSectionName = "default"

// reconfigureLogVariables reconfigures the log variables of cluster
func (o *updateOptions) reconfigureLogVariables(c *appsv1alpha1.Cluster, cd *appsv1alpha1.ClusterDefinition) error {
	var (
		err        error
		configSpec *appsv1alpha1.ComponentConfigSpec
		logValue   *gotemplate.TplValues
	)

	createReconfigureOps := func(compSpec appsv1alpha1.ClusterComponentSpec, configSpec *appsv1alpha1.ComponentConfigSpec, logValue *gotemplate.TplValues) error {
		var (
			buf             bytes.Buffer
			keyName         string
			configTemplate  *corev1.ConfigMap
			formatter       *appsv1alpha1.FormatterConfig
			logTPL          *template.Template
			logVariables    map[string]string
			unstructuredObj *unstructured.Unstructured
		)

		if configTemplate, formatter, err = findConfigTemplateInfo(o.dynamic, configSpec); err != nil {
			return err
		}
		if keyName, logTPL, err = findLogsBlockTPL(configTemplate.Data); err != nil {
			return err
		}
		if logTPL == nil {
			return nil
		}
		if err = logTPL.Execute(&buf, logValue); err != nil {
			return err
		}
		// TODO: very hack logic for ini config file
		formatter.FormatterOptions = appsv1alpha1.FormatterOptions{IniConfig: &appsv1alpha1.IniConfig{SectionName: defaultSectionName}}
		if logVariables, err = cfgcore.TransformConfigFileToKeyValueMap(keyName, formatter, buf.Bytes()); err != nil {
			return err
		}
		// build OpsRequest and apply this OpsRequest
		opsRequest := buildLogsReconfiguringOps(c.Name, c.Namespace, compSpec.Name, configSpec.Name, keyName, logVariables)
		if unstructuredObj, err = util.ConvertObjToUnstructured(opsRequest); err != nil {
			return err
		}
		return util.CreateResourceIfAbsent(o.dynamic, types.OpsGVR(), c.Namespace, unstructuredObj)
	}

	for _, compSpec := range c.Spec.ComponentSpecs {
		if configSpec, err = findFirstConfigSpec(c.Spec.ComponentSpecs, cd.Spec.ComponentDefs, compSpec.Name); err != nil {
			return err
		}
		if logValue, err = buildLogsTPLValues(&compSpec); err != nil {
			return err
		}
		if err = createReconfigureOps(compSpec, configSpec, logValue); err != nil {
			return err
		}
	}
	return nil
}

func findFirstConfigSpec(
	compSpecs []appsv1alpha1.ClusterComponentSpec,
	cdCompSpecs []appsv1alpha1.ClusterComponentDefinition,
	compName string) (*appsv1alpha1.ComponentConfigSpec, error) {
	configSpecs, err := util.GetConfigTemplateListWithResource(compSpecs, cdCompSpecs, nil, compName, true)
	if err != nil {
		return nil, err
	}
	if len(configSpecs) == 0 {
		return nil, errors.Errorf("no config templates for component %s", compName)
	}
	return &configSpecs[0], nil
}

func findConfigTemplateInfo(dynamic dynamic.Interface, configSpec *appsv1alpha1.ComponentConfigSpec) (*corev1.ConfigMap, *appsv1alpha1.FormatterConfig, error) {
	if configSpec == nil {
		return nil, nil, errors.New("configTemplateSpec is nil")
	}
	configTemplate, err := cluster.GetConfigMapByName(dynamic, configSpec.Namespace, configSpec.TemplateRef)
	if err != nil {
		return nil, nil, err
	}
	configConstraint, err := cluster.GetConfigConstraintByName(dynamic, configSpec.ConfigConstraintRef)
	if err != nil {
		return nil, nil, err
	}
	return configTemplate, configConstraint.Spec.FormatterConfig, nil
}

func newConfigTemplateEngine() *template.Template {
	customizedFuncMap := configuration.BuiltInCustomFunctions(nil, nil, nil)
	engine := gotemplate.NewTplEngine(nil, customizedFuncMap, logsTemplateName, nil, context.TODO())
	return engine.GetTplEngine()
}

func findLogsBlockTPL(confData map[string]string) (string, *template.Template, error) {
	engine := newConfigTemplateEngine()
	for key, value := range confData {
		if !strings.Contains(value, logsBlockName) {
			continue
		}
		tpl, err := engine.Parse(value)
		if err != nil {
			return key, nil, err
		}
		logTPL := tpl.Lookup(logsBlockName)
		// find target logs template
		if logTPL != nil {
			return key, logTPL, nil
		}
		return "", nil, errors.New("no logs config template found")
	}
	return "", nil, nil
}

func buildLogsTPLValues(compSpec *appsv1alpha1.ClusterComponentSpec) (*gotemplate.TplValues, error) {
	compMap := map[string]interface{}{}
	bytesData, err := json.Marshal(compSpec)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytesData, &compMap)
	if err != nil {
		return nil, err
	}
	value := gotemplate.TplValues{
		topTPLLogsObject: compMap,
	}
	return &value, nil
}

func buildLogsReconfiguringOps(clusterName, namespace, compName, configName, keyName string, variables map[string]string) *appsv1alpha1.OpsRequest {
	opsName := fmt.Sprintf("%s-%s", "logs-reconfigure", uuid.NewString())
	opsRequest := util.NewOpsRequestForReconfiguring(opsName, namespace, clusterName)
	parameterPairs := make([]appsv1alpha1.ParameterPair, 0, len(variables))
	for key, value := range variables {
		v := value
		parameterPairs = append(parameterPairs, appsv1alpha1.ParameterPair{
			Key:   key,
			Value: &v,
		})
	}
	var keys []appsv1alpha1.ParameterConfig
	keys = append(keys, appsv1alpha1.ParameterConfig{
		Key:        keyName,
		Parameters: parameterPairs,
	})
	var configurations []appsv1alpha1.ConfigurationItem
	configurations = append(configurations, appsv1alpha1.ConfigurationItem{
		Keys: keys,
		Name: configName,
	})
	reconfigure := opsRequest.Spec.Reconfigure
	reconfigure.ComponentName = compName
	reconfigure.Configurations = append(reconfigure.Configurations, configurations...)
	return opsRequest
}

func (o *updateOptions) updateMonitor(val string) error {
	intVal, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return err
	}

	for i := range o.cluster.Spec.ComponentSpecs {
		o.cluster.Spec.ComponentSpecs[i].Monitor = intVal != 0
	}
	return nil
}

func (o *updateOptions) updateBackupEnabled(val string) error {
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}
	o.cluster.Spec.Backup.Enabled = &boolVal
	return nil
}

func (o *updateOptions) updateBackupRetentionPeriod(val string) error {
	// if val is empty, do nothing
	if len(val) == 0 {
		return nil
	}

	// judge whether val end with the 'd'|'h' character
	lastChar := val[len(val)-1]
	if lastChar != 'd' && lastChar != 'h' {
		return fmt.Errorf("invalid retention period: %s, only support d|h", val)
	}

	o.cluster.Spec.Backup.RetentionPeriod = dpv1alpha1.RetentionPeriod(val)
	return nil
}

func (o *updateOptions) updateBackupMethod(val string) error {
	// TODO(ldm): validate backup method are defined in the backup policy.
	o.cluster.Spec.Backup.Method = val
	return nil
}

func (o *updateOptions) updateBackupCronExpression(val string) error {
	// judge whether val is a valid cron expression
	if _, err := cron.ParseStandard(val); err != nil {
		return fmt.Errorf("invalid cron expression: %s, please see https://en.wikipedia.org/wiki/Cron", val)
	}

	o.cluster.Spec.Backup.CronExpression = val
	return nil
}

func (o *updateOptions) updateBackupStartingDeadlineMinutes(val string) error {
	intVal, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return err
	}
	o.cluster.Spec.Backup.StartingDeadlineMinutes = &intVal
	return nil
}

func (o *updateOptions) updateBackupRepoName(val string) error {
	o.cluster.Spec.Backup.RepoName = val
	return nil
}

func (o *updateOptions) updateBackupPitrEnabled(val string) error {
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}
	o.cluster.Spec.Backup.PITREnabled = &boolVal
	return nil
}

// get backup methods from cluster's backup policy
// if method's snapshotVolumes is true, use the method as the default backup method
func (o *updateOptions) getBackupMethodsFromBackupPolicy() (string, map[string]struct{}, error) {
	if o.cluster == nil {
		return "", nil, fmt.Errorf("cluster is nil")
	}

	var backupPolicy []dpv1alpha1.BackupPolicy
	obj, err := o.dynamic.Resource(types.BackupPolicyGVR()).Namespace(o.namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", constant.AppInstanceLabelKey, o.cluster.Name),
	})
	if err != nil {
		return "", nil, err
	}
	for _, item := range obj.Items {
		var bp dpv1alpha1.BackupPolicy
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &bp); err != nil {
			return "", nil, err
		}
		backupPolicy = append(backupPolicy, bp)
	}

	var defaultBackupMethod string
	var backupMethodsMap = make(map[string]struct{})
	for _, policy := range backupPolicy {
		if policy.Annotations[dptypes.DefaultBackupPolicyAnnotationKey] != annotationTrueValue {
			continue
		}
		if policy.Status.Phase != dpv1alpha1.AvailablePhase {
			continue
		}
		for _, method := range policy.Spec.BackupMethods {
			if boolptr.IsSetToTrue(method.SnapshotVolumes) {
				defaultBackupMethod = method.Name
			}
			backupMethodsMap[method.Name] = struct{}{}
		}
	}
	if defaultBackupMethod == "" {
		return "", nil, fmt.Errorf("failed to find default backup method which snapshotVolumes is true, please check cluster's backup policy")
	}
	return defaultBackupMethod, backupMethodsMap, nil
}
