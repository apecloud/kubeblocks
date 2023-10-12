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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	rbacv1ac "k8s.io/client-go/applyconfigurations/rbac/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/storage"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

var clusterCreateExample = templates.Examples(`
	# Create a cluster with cluster definition apecloud-mysql and cluster version ac-mysql-8.0.30
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --cluster-version ac-mysql-8.0.30

	# --cluster-definition is required, if --cluster-version is not specified, pick the most recently created version
	kbcli cluster create mycluster --cluster-definition apecloud-mysql

	# Output resource information in YAML format, without creation of resources.
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --dry-run -o yaml

	# Output resource information in YAML format, the information will be sent to the server
	# but the resources will not be actually created.
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --dry-run=server -o yaml
	
	# Create a cluster and set termination policy DoNotTerminate that prevents the cluster from being deleted
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy DoNotTerminate

	# Delete resources such as statefulsets, deployments, services, pdb, but keep PVCs
	# when deleting the cluster, use termination policy Halt
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy Halt

	# Delete resource such as statefulsets, deployments, services, pdb, and including
	# PVCs when deleting the cluster, use termination policy Delete
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy Delete

	# Delete all resources including all snapshots and snapshot data when deleting
	# the cluster, use termination policy WipeOut
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy WipeOut

	# Create a cluster and set cpu to 1 core, memory to 1Gi, storage size to 20Gi and replicas to 3
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --set cpu=1,memory=1Gi,storage=20Gi,replicas=3

	# Create a cluster and set storageClass to csi-hostpath-sc, if storageClass is not specified,
	# the default storage class will be used
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --set storageClass=csi-hostpath-sc

	# Create a cluster with replicationSet workloadType and set switchPolicy to Noop
	kbcli cluster create mycluster --cluster-definition postgresql --set switchPolicy=Noop

	# Create a cluster with more than one component, use "--set type=component-name" to specify the component,
	# if not specified, the main component will be used, run "kbcli cd list-components CLUSTER-DEFINITION-NAME"
	# to show the components in the cluster definition
	kbcli cluster create mycluster --cluster-definition redis --set type=redis,cpu=1 --set type=redis-sentinel,cpu=200m

	# Create a cluster and use a URL to set cluster resource
	kbcli cluster create mycluster --cluster-definition apecloud-mysql \
		--set-file https://kubeblocks.io/yamls/apecloud-mysql.yaml

	# Create a cluster and load cluster resource set from stdin
	cat << EOF | kbcli cluster create mycluster --cluster-definition apecloud-mysql --set-file -
	- name: my-test ...

	# Create a cluster scattered by nodes
	kbcli cluster create --cluster-definition apecloud-mysql --topology-keys kubernetes.io/hostname \
		--pod-anti-affinity Required

	# Create a cluster in specific labels nodes
	kbcli cluster create --cluster-definition apecloud-mysql \
		--node-labels '"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'

	# Create a Cluster with two tolerations 
	kbcli cluster create --cluster-definition apecloud-mysql --tolerations \ '"engineType=mongo:NoSchedule","diskType=ssd:NoSchedule"'

    # Create a cluster, with each pod runs on their own dedicated node
    kbcli cluster create --cluster-definition apecloud-mysql --tenancy=DedicatedNode

    # Create a cluster with backup to restore data
    kbcli cluster create --backup backup-default-mycluster-20230616190023

    # Create a cluster with time to restore from point in time
    kbcli cluster create --restore-to-time "Jun 16,2023 18:58:53 UTC+0800" --source-cluster mycluster

	# Create a cluster with auto backup
	kbcli cluster create --cluster-definition apecloud-mysql --backup-enabled
`)

const (
	CueTemplateName = "cluster_template.cue"
	monitorKey      = "monitor"
	apeCloudMysql   = "apecloud-mysql"
)

type setKey string

const (
	keyType         setKey = "type"
	keyCPU          setKey = "cpu"
	keyClass        setKey = "class"
	keyMemory       setKey = "memory"
	keyReplicas     setKey = "replicas"
	keyStorage      setKey = "storage"
	keyStorageClass setKey = "storageClass"
	keySwitchPolicy setKey = "switchPolicy"
	keyUnknown      setKey = "unknown"
)

var setKeyCfg = map[setKey]string{
	keyCPU:      types.CfgKeyClusterDefaultCPU,
	keyMemory:   types.CfgKeyClusterDefaultMemory,
	keyStorage:  types.CfgKeyClusterDefaultStorageSize,
	keyReplicas: types.CfgKeyClusterDefaultReplicas,
}

// With the access of various databases, the simple way of specifying the capacity of storage by --set
// no longer meets the current demand, because many clusters' components are set up with multiple pvc, so we split the way of setting storage from `--set`.
type storageKey string

// map[string]map[storageKey]string `json:"-"`
const (
	// storageKeyType is the key of CreateOptions.Storages, reference to a cluster component name
	storageKeyType storageKey = "type"
	// storageKeyName is the name of a pvc in volumeClaimTemplates, like "data" or "log"
	storageKeyName storageKey = "name"
	// storageKeyStorageClass is the storageClass of a pvc
	storageKeyStorageClass storageKey = "storageClass"
	// storageAccessMode is the storageAccessMode of a pvc, could be ReadWriteOnce,ReadOnlyMany,ReadWriteMany.
	// more information in https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes
	storageAccessMode storageKey = "mode"
	// storageKeySize is the size of a pvc
	storageKeySize storageKey = "size"

	storageKeyUnknown storageKey = "unknown"
)

// UpdatableFlags is the flags that cat be updated by update command
type UpdatableFlags struct {
	// Options for cluster termination policy
	TerminationPolicy string `json:"terminationPolicy"`

	// Add-on switches for cluster observability
	MonitoringInterval uint8 `json:"monitor"`
	EnableAllLogs      bool  `json:"enableAllLogs"`

	// Configuration and options for cluster affinity and tolerations
	PodAntiAffinity string `json:"podAntiAffinity"`
	// TopologyKeys if TopologyKeys is nil, add omitempty json tag, because CueLang can not covert null to list.
	TopologyKeys   []string          `json:"topologyKeys,omitempty"`
	NodeLabels     map[string]string `json:"nodeLabels,omitempty"`
	Tenancy        string            `json:"tenancy"`
	TolerationsRaw []string          `json:"-"`

	// backup config
	BackupEnabled                 bool   `json:"-"`
	BackupRetentionPeriod         string `json:"-"`
	BackupMethod                  string `json:"-"`
	BackupCronExpression          string `json:"-"`
	BackupStartingDeadlineMinutes int64  `json:"-"`
	BackupRepoName                string `json:"-"`
	BackupPITREnabled             bool   `json:"-"`
}

type CreateOptions struct {
	// ClusterDefRef reference clusterDefinition
	ClusterDefRef     string                   `json:"clusterDefRef"`
	ClusterVersionRef string                   `json:"clusterVersionRef"`
	Tolerations       []interface{}            `json:"tolerations,omitempty"`
	ComponentSpecs    []map[string]interface{} `json:"componentSpecs"`
	Annotations       map[string]string        `json:"annotations,omitempty"`
	SetFile           string                   `json:"-"`
	Values            []string                 `json:"-"`
	RBACEnabled       bool                     `json:"-"`
	Storages          []string                 `json:"-"`
	// backup name to restore in creation
	Backup                  string `json:"backup,omitempty"`
	RestoreTime             string `json:"restoreTime,omitempty"`
	RestoreManagementPolicy string `json:"-"`

	// backup config
	BackupConfig *appsv1alpha1.ClusterBackup `json:"backupConfig,omitempty"`

	Cmd *cobra.Command `json:"-"`

	UpdatableFlags
	create.CreateOptions `json:"-"`
}

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateOptions(f, streams)
	cmd := &cobra.Command{
		Use:     "create [NAME]",
		Short:   "Create a cluster.",
		Example: clusterCreateExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "Specify cluster definition, run \"kbcli cd list\" to show all available cluster definitions")
	cmd.Flags().StringVar(&o.ClusterVersionRef, "cluster-version", "", "Specify cluster version, run \"kbcli cv list\" to show all available cluster versions, use the latest version if not specified")
	cmd.Flags().StringVarP(&o.SetFile, "set-file", "f", "", "Use yaml file, URL, or stdin to set the cluster resource")
	cmd.Flags().StringArrayVar(&o.Values, "set", []string{}, "Set the cluster resource including cpu, memory, replicas and storage, each set corresponds to a component.(e.g. --set cpu=1,memory=1Gi,replicas=3,storage=20Gi or --set class=general-1c1g)")
	cmd.Flags().StringArrayVar(&o.Storages, "pvc", []string{}, "Set the cluster detail persistent volume claim, each '--pvc' corresponds to a component, and will override the simple configurations about storage by --set (e.g. --pvc type=mysql,name=data,mode=ReadWriteOnce,size=20Gi --pvc type=mysql,name=log,mode=ReadWriteOnce,size=1Gi)")
	cmd.Flags().StringVar(&o.Backup, "backup", "", "Set a source backup to restore data")
	cmd.Flags().StringVar(&o.RestoreTime, "restore-to-time", "", "Set a time for point in time recovery")
	cmd.Flags().StringVar(&o.RestoreManagementPolicy, "volume-restore-policy", "Parallel", "the volume claim restore policy, supported values: [Serial, Parallel]")
	cmd.Flags().BoolVar(&o.RBACEnabled, "rbac-enabled", false, "Specify whether rbac resources will be created by kbcli, otherwise KubeBlocks server will try to create rbac resources")
	cmd.PersistentFlags().BoolVar(&o.EditBeforeCreate, "edit", o.EditBeforeCreate, "Edit the API resource before creating")
	cmd.PersistentFlags().StringVar(&o.DryRun, "dry-run", "none", `Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent.`)
	cmd.PersistentFlags().Lookup("dry-run").NoOptDefVal = "unchanged"

	// add updatable flags
	o.UpdatableFlags.addFlags(cmd)

	// add print flags
	printer.AddOutputFlagForCreate(cmd, &o.Format, true)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	// add all subcommands for supported cluster type
	cmd.AddCommand(buildCreateSubCmds(&o.CreateOptions)...)

	o.Cmd = cmd

	return cmd
}

func NewCreateOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *CreateOptions {
	o := &CreateOptions{CreateOptions: create.CreateOptions{
		Factory:         f,
		IOStreams:       streams,
		CueTemplateName: CueTemplateName,
		GVR:             types.ClusterGVR(),
	}}
	o.CreateOptions.Options = o
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.CreateDependencies = o.CreateDependencies
	o.CreateOptions.CleanUpFn = o.CleanUp
	return o
}

func setMonitor(monitoringInterval uint8, components []map[string]interface{}) {
	if len(components) == 0 {
		return
	}
	for _, component := range components {
		component[monitorKey] = monitoringInterval != 0
	}
}

func getRestoreFromBackupAnnotation(backup *dpv1alpha1.Backup, managementPolicy string, compSpecsCount int, firstCompName string, restoreTime string) (string, error) {
	componentName := backup.Labels[constant.KBAppComponentLabelKey]
	if len(componentName) == 0 {
		if compSpecsCount != 1 {
			return "", fmt.Errorf("unable to obtain the name of the component to be recovered, please ensure that Backup.status.componentName exists")
		}
		componentName = firstCompName
	}
	backupNameString := fmt.Sprintf(`"%s":"%s"`, constant.BackupNameKeyForRestore, backup.Name)
	backupNamespaceString := fmt.Sprintf(`"%s":"%s"`, constant.BackupNamespaceKeyForRestore, backup.Namespace)
	managementPolicyString := fmt.Sprintf(`"%s":"%s"`, constant.VolumeManagementPolicyKeyForRestore, managementPolicy)
	var restoreTimeString string
	if restoreTime != "" {
		restoreTimeString = fmt.Sprintf(`",%s":"%s"`, constant.RestoreTimeKeyForRestore, restoreTime)
	}

	restoreFromBackupAnnotation := fmt.Sprintf(`{"%s":{%s,%s,%s%s}}`, componentName, backupNameString, backupNamespaceString, managementPolicyString, restoreTimeString)
	return restoreFromBackupAnnotation, nil
}

func getSourceClusterFromBackup(backup *dpv1alpha1.Backup) (*appsv1alpha1.Cluster, error) {
	sourceCluster := &appsv1alpha1.Cluster{}
	sourceClusterJSON := backup.Annotations[constant.ClusterSnapshotAnnotationKey]
	if err := json.Unmarshal([]byte(sourceClusterJSON), sourceCluster); err != nil {
		return nil, err
	}

	return sourceCluster, nil
}

func getBackupObjectFromRestoreArgs(o *CreateOptions, backup *dpv1alpha1.Backup) error {
	if o.Backup == "" {
		return nil
	}
	if err := cluster.GetK8SClientObject(o.Dynamic, backup, types.BackupGVR(), o.Namespace, o.Backup); err != nil {
		return err
	}
	return nil
}

func fillClusterInfoFromBackup(o *CreateOptions, cls **appsv1alpha1.Cluster) error {
	if o.Backup == "" {
		return nil
	}
	backup := &dpv1alpha1.Backup{}
	if err := getBackupObjectFromRestoreArgs(o, backup); err != nil {
		return err
	}
	backupCluster, err := getSourceClusterFromBackup(backup)
	if err != nil {
		return err
	}
	curCluster := *cls
	if curCluster == nil {
		curCluster = backupCluster
	}

	// validate cluster spec
	if o.ClusterDefRef != "" && o.ClusterDefRef != backupCluster.Spec.ClusterDefRef {
		return fmt.Errorf("specified cluster definition does not match from backup(expect: %s, actual: %s),"+
			" please check", backupCluster.Spec.ClusterDefRef, o.ClusterDefRef)
	}
	if o.ClusterVersionRef != "" && o.ClusterVersionRef != backupCluster.Spec.ClusterVersionRef {
		return fmt.Errorf("specified cluster version does not match from backup(expect: %s, actual: %s),"+
			" please check", backupCluster.Spec.ClusterVersionRef, o.ClusterVersionRef)
	}

	o.ClusterDefRef = curCluster.Spec.ClusterDefRef
	o.ClusterVersionRef = curCluster.Spec.ClusterVersionRef

	*cls = curCluster
	return nil
}

func formatRestoreTimeAndValidate(restoreTimeStr string, continuousBackup *dpv1alpha1.Backup) (string, error) {
	if restoreTimeStr == "" {
		return restoreTimeStr, nil
	}
	restoreTime, err := util.TimeParse(restoreTimeStr, time.Second)
	if err != nil {
		// retry to parse time with RFC3339 format.
		var errRFC error
		restoreTime, errRFC = time.Parse(time.RFC3339, restoreTimeStr)
		if errRFC != nil {
			// if retry failure, report the error
			return restoreTimeStr, err
		}
	}
	restoreTimeStr = restoreTime.Format(time.RFC3339)
	// TODO: check with Recoverable time
	if !isTimeInRange(restoreTime, continuousBackup.Status.TimeRange.Start.Time, continuousBackup.Status.TimeRange.End.Time) {
		return restoreTimeStr, fmt.Errorf("restore-to-time is out of time range, you can view the recoverable time: \n"+
			"\tkbcli cluster describe %s -n %s", continuousBackup.Labels[constant.AppInstanceLabelKey], continuousBackup.Namespace)
	}
	return restoreTimeStr, nil
}

func setBackup(o *CreateOptions, components []map[string]interface{}) error {
	backupName := o.Backup
	if len(backupName) == 0 || len(components) == 0 {
		return nil
	}
	backup := &dpv1alpha1.Backup{}
	if err := cluster.GetK8SClientObject(o.Dynamic, backup, types.BackupGVR(), o.Namespace, backupName); err != nil {
		return err
	}
	if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		return fmt.Errorf(`backup "%s" is not completed`, backup.Name)
	}
	restoreTimeStr, err := formatRestoreTimeAndValidate(o.RestoreTime, backup)
	if err != nil {
		return err
	}
	restoreAnnotation, err := getRestoreFromBackupAnnotation(backup, o.RestoreManagementPolicy, len(components), components[0]["name"].(string), restoreTimeStr)
	if err != nil {
		return err
	}
	if o.Annotations == nil {
		o.Annotations = map[string]string{}
	}
	o.Annotations[constant.RestoreFromBackupAnnotationKey] = restoreAnnotation
	return nil
}

func (o *CreateOptions) Validate() error {
	if o.ClusterDefRef == "" {
		return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one, run \"kbcli clusterdefinition list\" to show all cluster definitions")
	}

	if o.TerminationPolicy == "" {
		return fmt.Errorf("a valid termination policy is needed, use --termination-policy to specify one of: DoNotTerminate, Halt, Delete, WipeOut")
	}

	if err := o.validateClusterVersion(); err != nil {
		return err
	}

	if len(o.Values) > 0 && len(o.SetFile) > 0 {
		return fmt.Errorf("does not support --set and --set-file being specified at the same time")
	}

	matched, _ := regexp.MatchString(`^[a-z]([-a-z0-9]*[a-z0-9])?$`, o.Name)
	if !matched {
		return fmt.Errorf("cluster name must begin with a letter and can only contain lowercase letters, numbers, and '-'")
	}

	if len(o.Name) > 16 {
		return fmt.Errorf("cluster name should be less than 16 characters")
	}

	return nil
}

func (o *CreateOptions) Complete() error {
	var (
		compByte         []byte
		cls              *appsv1alpha1.Cluster
		clusterCompSpecs []appsv1alpha1.ClusterComponentSpec
		err              error
	)

	if len(o.SetFile) > 0 {
		if compByte, err = MultipleSourceComponents(o.SetFile, o.IOStreams.In); err != nil {
			return err
		}
		if compByte, err = yaml.YAMLToJSON(compByte); err != nil {
			return err
		}

		// compatible with old file format that only specifies the components
		if err = json.Unmarshal(compByte, &cls); err != nil {
			if clusterCompSpecs, err = parseClusterComponentSpec(compByte); err != nil {
				return err
			}
		} else {
			clusterCompSpecs = cls.Spec.ComponentSpecs
		}
	}
	if err = fillClusterInfoFromBackup(o, &cls); err != nil {
		return err
	}
	if nil != cls && cls.Spec.ComponentSpecs != nil {
		clusterCompSpecs = cls.Spec.ComponentSpecs
	}

	// if name is not specified, generate a random cluster name
	if o.Name == "" {
		o.Name, err = generateClusterName(o.Dynamic, o.Namespace)
		if err != nil {
			return err
		}
	}

	// build annotation
	o.buildAnnotation(cls)

	// build cluster definition
	if err := o.buildClusterDef(cls); err != nil {
		return err
	}

	// build cluster version
	o.buildClusterVersion(cls)

	// build backup config
	if err := o.buildBackupConfig(cls); err != nil {
		return err
	}

	// build components
	components, err := o.buildComponents(clusterCompSpecs)
	if err != nil {
		return err
	}

	setMonitor(o.MonitoringInterval, components)
	if err = setBackup(o, components); err != nil {
		return err
	}
	o.ComponentSpecs = components

	// TolerationsRaw looks like `["key=engineType,value=mongo,operator=Equal,effect=NoSchedule"]` after parsing by cmd
	tolerations, err := util.BuildTolerations(o.TolerationsRaw)
	if err != nil {
		return err
	}
	if len(tolerations) > 0 {
		o.Tolerations = tolerations
	}

	// validate default storageClassName
	return validateStorageClass(o.Dynamic, o.ComponentSpecs)
}

func (o *CreateOptions) CleanUp() error {
	if o.Client == nil {
		return nil
	}

	return deleteDependencies(o.Client, o.Namespace, o.Name)
}

// buildComponents builds components from file or set values
func (o *CreateOptions) buildComponents(clusterCompSpecs []appsv1alpha1.ClusterComponentSpec) ([]map[string]interface{}, error) {
	var (
		err       error
		cd        *appsv1alpha1.ClusterDefinition
		compSpecs []*appsv1alpha1.ClusterComponentSpec
		storages  map[string][]map[storageKey]string
	)

	cd, err = cluster.GetClusterDefByName(o.Dynamic, o.ClusterDefRef)
	if err != nil {
		return nil, err
	}
	clsMgr, err := class.GetManager(o.Dynamic, o.ClusterDefRef)
	if err != nil {
		return nil, err
	}

	compSets, err := buildCompSetsMap(o.Values, cd)
	if err != nil {
		return nil, err
	}
	if len(o.Storages) != 0 {
		storages, err = buildCompStorages(o.Storages, cd)
		if err != nil {
			return nil, err
		}
	}

	overrideComponentBySets := func(comp, setComp *appsv1alpha1.ClusterComponentSpec, setValues map[setKey]string) {
		for k := range setValues {
			switch k {
			case keyReplicas:
				comp.Replicas = setComp.Replicas
			case keyCPU:
				comp.Resources.Requests[corev1.ResourceCPU] = setComp.Resources.Requests[corev1.ResourceCPU]
				comp.Resources.Limits[corev1.ResourceCPU] = setComp.Resources.Limits[corev1.ResourceCPU]
			case keyClass:
				comp.ClassDefRef = setComp.ClassDefRef
			case keyMemory:
				comp.Resources.Requests[corev1.ResourceMemory] = setComp.Resources.Requests[corev1.ResourceMemory]
				comp.Resources.Limits[corev1.ResourceMemory] = setComp.Resources.Limits[corev1.ResourceMemory]
			case keyStorage:
				if len(comp.VolumeClaimTemplates) > 0 && len(setComp.VolumeClaimTemplates) > 0 {
					comp.VolumeClaimTemplates[0].Spec.Resources.Requests = setComp.VolumeClaimTemplates[0].Spec.Resources.Requests
				}
			case keyStorageClass:
				if len(comp.VolumeClaimTemplates) > 0 && len(setComp.VolumeClaimTemplates) > 0 {
					comp.VolumeClaimTemplates[0].Spec.StorageClassName = setComp.VolumeClaimTemplates[0].Spec.StorageClassName
				}
			case keySwitchPolicy:
				comp.SwitchPolicy = setComp.SwitchPolicy
			}
		}
	}

	if clusterCompSpecs != nil {
		setsCompSpecs, err := buildClusterComp(cd, compSets, clsMgr)
		if err != nil {
			return nil, err
		}
		setsCompSpecsMap := map[string]*appsv1alpha1.ClusterComponentSpec{}
		for _, setComp := range setsCompSpecs {
			setsCompSpecsMap[setComp.Name] = setComp
		}
		for index := range clusterCompSpecs {
			comp := clusterCompSpecs[index]
			overrideComponentBySets(&comp, setsCompSpecsMap[comp.Name], compSets[comp.Name])
			compSpecs = append(compSpecs, &comp)
		}
	} else {
		compSpecs, err = buildClusterComp(cd, compSets, clsMgr)
		if err != nil {
			return nil, err
		}
	}

	if len(storages) != 0 {
		compSpecs = rebuildCompStorage(storages, compSpecs)
	}

	var comps []map[string]interface{}
	for _, compSpec := range compSpecs {
		// validate component classes
		if err = clsMgr.ValidateResources(o.ClusterDefRef, compSpec); err != nil {
			return nil, err
		}

		// create component dependencies
		if err = o.buildDependenciesFn(cd, compSpec); err != nil {
			return nil, err
		}

		comp, err := runtime.DefaultUnstructuredConverter.ToUnstructured(compSpec)
		if err != nil {
			return nil, err
		}
		comps = append(comps, comp)
	}
	return comps, nil
}

const (
	saNamePrefix             = "kb-"
	roleNamePrefix           = "kb-"
	roleBindingNamePrefix    = "kb-"
	clusterRolePrefix        = "kb-"
	clusterRoleBindingPrefix = "kb-"
)

var (
	rbacAPIGroup    = "rbac.authorization.k8s.io"
	saKind          = "ServiceAccount"
	roleKind        = "Role"
	clusterRoleKind = "ClusterRole"
)

// buildDependenciesFn creates dependencies function for components, e.g. postgresql depends on
// a service account, a role and a rolebinding
func (o *CreateOptions) buildDependenciesFn(cd *appsv1alpha1.ClusterDefinition,
	compSpec *appsv1alpha1.ClusterComponentSpec) error {
	// set component service account name
	compSpec.ServiceAccountName = saNamePrefix + o.Name
	return nil
}

func (o *CreateOptions) CreateDependencies(dryRun []string) error {
	if !o.RBACEnabled {
		return nil
	}

	var (
		ctx          = context.TODO()
		labels       = buildResourceLabels(o.Name)
		applyOptions = metav1.ApplyOptions{FieldManager: "kbcli", DryRun: dryRun}
	)

	klog.V(1).Infof("create dependencies for cluster %s", o.Name)

	if err := o.createServiceAccount(ctx, labels, applyOptions); err != nil {
		return err
	}
	if err := o.createRoleAndBinding(ctx, labels, applyOptions); err != nil {
		return err
	}
	if err := o.createClusterRoleAndBinding(ctx, labels, applyOptions); err != nil {
		return err
	}
	return nil
}

func (o *CreateOptions) createServiceAccount(ctx context.Context, labels map[string]string, opts metav1.ApplyOptions) error {
	saName := saNamePrefix + o.Name
	klog.V(1).Infof("create service account %s", saName)
	sa := corev1ac.ServiceAccount(saName, o.Namespace).WithLabels(labels)
	_, err := o.Client.CoreV1().ServiceAccounts(o.Namespace).Apply(ctx, sa, opts)
	return err
}

func (o *CreateOptions) createRoleAndBinding(ctx context.Context, labels map[string]string, opts metav1.ApplyOptions) error {
	var (
		saName          = saNamePrefix + o.Name
		roleName        = roleNamePrefix + o.Name
		roleBindingName = roleBindingNamePrefix + o.Name
	)

	klog.V(1).Infof("create role %s", roleName)
	role := rbacv1ac.Role(roleName, o.Namespace).WithRules([]*rbacv1ac.PolicyRuleApplyConfiguration{
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"dataprotection.kubeblocks.io"},
			Resources: []string{"backups/status"},
			Verbs:     []string{"get", "update", "patch"},
		},
		{
			APIGroups: []string{"dataprotection.kubeblocks.io"},
			Resources: []string{"backups"},
			Verbs:     []string{"create", "get", "list", "update", "patch"},
		},
	}...).WithLabels(labels)

	// postgresql need more rules for patroni
	if ok, err := o.isPostgresqlCluster(); err != nil {
		return err
	} else if ok {
		rules := []rbacv1ac.PolicyRuleApplyConfiguration{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "patch", "update", "watch"},
			},
		}
		role.Rules = append(role.Rules, rules...)
	}
	if _, err := o.Client.RbacV1().Roles(o.Namespace).Apply(ctx, role, opts); err != nil {
		return err
	}

	klog.V(1).Infof("create role binding %s", roleBindingName)
	roleBinding := rbacv1ac.RoleBinding(roleBindingName, o.Namespace).WithLabels(labels).
		WithSubjects([]*rbacv1ac.SubjectApplyConfiguration{
			{
				Kind:      &saKind,
				Name:      &saName,
				Namespace: &o.Namespace,
			},
		}...).
		WithRoleRef(&rbacv1ac.RoleRefApplyConfiguration{
			APIGroup: &rbacAPIGroup,
			Kind:     &roleKind,
			Name:     &roleName,
		})
	_, err := o.Client.RbacV1().RoleBindings(o.Namespace).Apply(ctx, roleBinding, opts)
	return err
}

func (o *CreateOptions) createClusterRoleAndBinding(ctx context.Context, labels map[string]string, opts metav1.ApplyOptions) error {
	var (
		saName                 = saNamePrefix + o.Name
		clusterRoleName        = clusterRolePrefix + o.Name
		clusterRoleBindingName = clusterRoleBindingPrefix + o.Name
	)

	klog.V(1).Infof("create cluster role %s", clusterRoleName)
	clusterRole := rbacv1ac.ClusterRole(clusterRoleName).WithRules([]*rbacv1ac.PolicyRuleApplyConfiguration{
		{
			APIGroups: []string{""},
			Resources: []string{"nodes", "nodes/stats"},
			Verbs:     []string{"get", "list"},
		},
	}...).WithLabels(labels)
	if _, err := o.Client.RbacV1().ClusterRoles().Apply(ctx, clusterRole, opts); err != nil {
		return err
	}

	klog.V(1).Infof("create cluster role binding %s", clusterRoleBindingName)
	clusterRoleBinding := rbacv1ac.ClusterRoleBinding(clusterRoleBindingName).WithLabels(labels).
		WithSubjects([]*rbacv1ac.SubjectApplyConfiguration{
			{
				Kind:      &saKind,
				Name:      &saName,
				Namespace: &o.Namespace,
			},
		}...).
		WithRoleRef(&rbacv1ac.RoleRefApplyConfiguration{
			APIGroup: &rbacAPIGroup,
			Kind:     &clusterRoleKind,
			Name:     &clusterRoleName,
		})
	_, err := o.Client.RbacV1().ClusterRoleBindings().Apply(ctx, clusterRoleBinding, opts)
	return err
}

// MultipleSourceComponents gets component data from multiple source, such as stdin, URI and local file
func MultipleSourceComponents(fileName string, in io.Reader) ([]byte, error) {
	var data io.Reader
	switch {
	case fileName == "-":
		data = in
	case strings.Index(fileName, "http://") == 0 || strings.Index(fileName, "https://") == 0:
		resp, err := http.Get(fileName)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		data = resp.Body
	default:
		f, err := os.Open(fileName)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data = f
	}
	return io.ReadAll(data)
}

func registerFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster-definition",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, util.GVRToString(types.ClusterDefGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster-version",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var clusterVersion []string
			clusterDefinition, err := cmd.Flags().GetString("cluster-definition")
			if clusterDefinition == "" || err != nil {
				clusterVersion = utilcomp.CompGetResource(f, util.GVRToString(types.ClusterVersionGVR()), toComplete)
			} else {
				label := fmt.Sprintf("%s=%s", constant.ClusterDefLabelKey, clusterDefinition)
				clusterVersion = util.CompGetResourceWithLabels(f, cmd, util.GVRToString(types.ClusterVersionGVR()), []string{label}, toComplete)
			}
			return clusterVersion, cobra.ShellCompDirectiveNoFileComp
		}))

	var formatsWithDesc = map[string]string{
		"JSON": "Output result in JSON format",
		"YAML": "Output result in YAML format",
	}
	util.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for format, desc := range formatsWithDesc {
				if strings.HasPrefix(format, toComplete) {
					names = append(names, fmt.Sprintf("%s\t%s", format, desc))
				}
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		}))
}

// PreCreate before saving yaml to k8s, makes changes on Unstructured yaml
func (o *CreateOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &appsv1alpha1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, c); err != nil {
		return err
	}
	// get cluster definition from k8s
	cd, err := cluster.GetClusterDefByName(o.Dynamic, c.Spec.ClusterDefRef)
	if err != nil {
		return err
	}

	if !o.EnableAllLogs {
		setEnableAllLogs(c, cd)
	}
	if o.BackupConfig == nil {
		// if backup config is not specified, set cluster's backup to nil
		c.Spec.Backup = nil
	}
	data, e := runtime.DefaultUnstructuredConverter.ToUnstructured(c)
	if e != nil {
		return e
	}
	obj.SetUnstructuredContent(data)
	return nil
}

func (o *CreateOptions) isPostgresqlCluster() (bool, error) {
	cd, err := cluster.GetClusterDefByName(o.Dynamic, o.ClusterDefRef)
	if err != nil {
		return false, err
	}

	var compDef *appsv1alpha1.ClusterComponentDefinition
	if cd.Spec.Type != "postgresql" {
		return false, nil
	}

	// get cluster component definition
	if len(o.ComponentSpecs) == 0 {
		return false, fmt.Errorf("find no cluster component")
	}
	compSpec := o.ComponentSpecs[0]
	for i, def := range cd.Spec.ComponentDefs {
		compDefRef := compSpec["componentDefRef"]
		if compDefRef != nil && def.Name == compDefRef.(string) {
			compDef = &cd.Spec.ComponentDefs[i]
		}
	}

	if compDef == nil {
		return false, fmt.Errorf("failed to find component definition for component %v", compSpec["Name"])
	}

	// for postgresql, we need to create a service account, a role and a rolebinding
	if compDef.CharacterType != "postgresql" {
		return false, nil
	}
	return true, nil
}

// setEnableAllLog sets enable all logs, and ignore enabledLogs of component level.
func setEnableAllLogs(c *appsv1alpha1.Cluster, cd *appsv1alpha1.ClusterDefinition) {
	for idx, comCluster := range c.Spec.ComponentSpecs {
		for _, com := range cd.Spec.ComponentDefs {
			if !strings.EqualFold(comCluster.ComponentDefRef, com.Name) {
				continue
			}
			typeList := make([]string, 0, len(com.LogConfigs))
			for _, logConf := range com.LogConfigs {
				typeList = append(typeList, logConf.Name)
			}
			c.Spec.ComponentSpecs[idx].EnabledLogs = typeList
		}
	}
}

func buildClusterComp(cd *appsv1alpha1.ClusterDefinition, setsMap map[string]map[setKey]string, clsMgr *class.Manager) ([]*appsv1alpha1.ClusterComponentSpec, error) {
	// get value from set values and environment variables, the second return value is
	// true if the value is from environment variables
	getVal := func(c *appsv1alpha1.ClusterComponentDefinition, key setKey, sets map[setKey]string) string {
		// get value from set values
		if v := sets[key]; len(v) > 0 {
			return v
		}

		// HACK: if user does not set by command flag, for replicationSet workload,
		// set replicas to 2, for redis sentinel, set replicas to 3, cpu and memory
		// to 200M and 200Mi
		// TODO: use more graceful way to set default value
		if c.WorkloadType == appsv1alpha1.Replication {
			if key == keyReplicas {
				return "2"
			}
		}

		// the default replicas is 3 if not set by command flag, for Consensus workload
		if c.WorkloadType == appsv1alpha1.Consensus {
			if key == keyReplicas {
				return "3"
			}
		}

		if c.CharacterType == "redis" && c.Name == "redis-sentinel" {
			switch key {
			case keyReplicas:
				return "3"
			case keyCPU:
				return "200m"
			case keyMemory:
				return "200Mi"
			}
		}

		// get value from environment variables
		cfg := setKeyCfg[key]
		return viper.GetString(cfg)
	}

	buildSwitchPolicy := func(c *appsv1alpha1.ClusterComponentDefinition, compObj *appsv1alpha1.ClusterComponentSpec, sets map[setKey]string) error {
		if c.WorkloadType != appsv1alpha1.Replication {
			return nil
		}
		var switchPolicyType appsv1alpha1.SwitchPolicyType
		switch getVal(c, keySwitchPolicy, sets) {
		case "Noop", "":
			switchPolicyType = appsv1alpha1.Noop
		case "MaximumAvailability":
			switchPolicyType = appsv1alpha1.MaximumAvailability
		case "MaximumPerformance":
			switchPolicyType = appsv1alpha1.MaximumDataProtection
		default:
			return fmt.Errorf("switchPolicy is illegal, only support Noop, MaximumAvailability, MaximumPerformance")
		}
		compObj.SwitchPolicy = &appsv1alpha1.ClusterSwitchPolicy{
			Type: switchPolicyType,
		}
		return nil
	}

	var comps []*appsv1alpha1.ClusterComponentSpec
	for i, c := range cd.Spec.ComponentDefs {
		sets := setsMap[c.Name]

		// HACK: for apecloud-mysql cluster definition, if setsMap is empty, user
		// does not specify any set, so we only build the first component.
		// TODO(ldm): remove this hack and use helm chart to render the cluster.
		if i > 0 && len(sets) == 0 && cd.Name == apeCloudMysql {
			continue
		}

		// get replicas
		setReplicas, err := strconv.Atoi(getVal(&c, keyReplicas, sets))
		if err != nil {
			return nil, fmt.Errorf("repicas is illegal " + err.Error())
		}
		if setReplicas < 0 {
			return nil, fmt.Errorf("repicas is illegal, required value >=0")
		}
		if setReplicas > math.MaxInt32 {
			return nil, fmt.Errorf("repicas is illegal, exceed max. value (%d) ", math.MaxInt32)
		}
		replicas := int32(setReplicas)

		compObj := &appsv1alpha1.ClusterComponentSpec{
			Name:            c.Name,
			ComponentDefRef: c.Name,
			Replicas:        replicas,
		}

		// class has higher priority than other resource related parameters
		resourceList := make(corev1.ResourceList)
		if clsMgr.HasClass(compObj.ComponentDefRef, class.Any) {
			if className := getVal(&c, keyClass, sets); className != "" {
				clsDefRef := appsv1alpha1.ClassDefRef{}
				parts := strings.SplitN(className, ":", 2)
				if len(parts) == 1 {
					clsDefRef.Class = parts[0]
				} else {
					clsDefRef.Name = parts[0]
					clsDefRef.Class = parts[1]
				}
				compObj.ClassDefRef = &clsDefRef
			} else {
				resourceList = corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(getVal(&c, keyCPU, sets)),
					corev1.ResourceMemory: resource.MustParse(getVal(&c, keyMemory, sets)),
				}
			}
		} else {
			if className := getVal(&c, keyClass, sets); className != "" {
				return nil, fmt.Errorf("can not find class %s for component type %s", className, c.Name)
			}
			resourceList = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(getVal(&c, keyCPU, sets)),
				corev1.ResourceMemory: resource.MustParse(getVal(&c, keyMemory, sets)),
			}
		}
		compObj.Resources = corev1.ResourceRequirements{
			Requests: resourceList,
			Limits:   resourceList,
		}
		compObj.VolumeClaimTemplates = []appsv1alpha1.ClusterComponentVolumeClaimTemplate{{
			Name: "data",
			Spec: appsv1alpha1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(getVal(&c, keyStorage, sets)),
					},
				},
			},
		}}
		storageClass := getVal(&c, keyStorageClass, sets)
		if len(storageClass) != 0 {
			// now the clusterdefinition components mostly have only one VolumeClaimTemplates in default
			compObj.VolumeClaimTemplates[0].Spec.StorageClassName = &storageClass
		}
		if err = buildSwitchPolicy(&c, compObj, sets); err != nil {
			return nil, err
		}
		comps = append(comps, compObj)
	}
	return comps, nil
}

// buildCompSetsMap builds the map between component definition name and its set values, if the name is not
// specified in the set, use the cluster definition default component name.
func buildCompSetsMap(values []string, cd *appsv1alpha1.ClusterDefinition) (map[string]map[setKey]string, error) {
	allSets := map[string]map[setKey]string{}
	parseKey := func(key string) setKey {
		for _, k := range setKeys() {
			if strings.EqualFold(k, key) {
				return setKey(k)
			}
		}
		return keyUnknown
	}
	buildSetMap := func(sets []string) (map[setKey]string, error) {
		res := map[setKey]string{}
		for _, set := range sets {
			kv := strings.Split(set, "=")
			if len(kv) != 2 {
				return nil, fmt.Errorf("unknown set format \"%s\", should be like key1=value1", set)
			}

			// only record the supported key
			k := parseKey(kv[0])
			if k == keyUnknown {
				return nil, fmt.Errorf("unknown set key \"%s\", should be one of [%s]", kv[0], strings.Join(setKeys(), ","))
			}
			res[k] = kv[1]
		}
		return res, nil
	}

	// each value corresponds to a component
	for _, value := range values {
		sets, err := buildSetMap(strings.Split(value, ","))
		if err != nil {
			return nil, err
		}
		if len(sets) == 0 {
			continue
		}

		// get the component definition name
		compDefName := sets[keyType]

		// type is not specified by user, use the default component definition name, now only
		// support cluster definition with one component
		if len(compDefName) == 0 {
			name, err := cluster.GetDefaultCompName(cd)
			if err != nil {
				return nil, err
			}

			// if the number of component definitions is more than one, default use the first one and output a log
			if len(cd.Spec.ComponentDefs) > 1 {
				klog.V(1).Infof("the component is not specified, use the default component \"%s\" in cluster definition \"%s\"", name, cd.Name)
			}
			compDefName = name
		} else {
			// check the type is a valid component definition name
			valid := false
			for _, c := range cd.Spec.ComponentDefs {
				if c.Name == compDefName {
					valid = true
					break
				}
			}
			if !valid {
				return nil, fmt.Errorf("the type \"%s\" is not a valid component definition name", compDefName)
			}
		}

		// if already set by other value, later values override earlier values
		if old, ok := allSets[compDefName]; ok {
			for k, v := range sets {
				old[k] = v
			}
			sets = old
		}
		allSets[compDefName] = sets
	}
	return allSets, nil
}

// generateClusterName generates a random cluster name that does not exist
func generateClusterName(dynamic dynamic.Interface, namespace string) (string, error) {
	var name string
	// retry 10 times
	for i := 0; i < 10; i++ {
		name = cluster.GenerateName()
		// check whether the cluster exists, if not found, return it
		_, err := dynamic.Resource(types.ClusterGVR()).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return name, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("failed to generate cluster name")
}

func (f *UpdatableFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.PodAntiAffinity, "pod-anti-affinity", "Preferred", "Pod anti-affinity type, one of: (Preferred, Required)")
	cmd.Flags().Uint8Var(&f.MonitoringInterval, "monitoring-interval", 0, "The monitoring interval of cluster, 0 is disabled, the unit is second, any non-zero value means enabling monitoring.")
	cmd.Flags().BoolVar(&f.EnableAllLogs, "enable-all-logs", false, "Enable advanced application all log extraction, set to true will ignore enabledLogs of component level, default is false")
	cmd.Flags().StringVar(&f.TerminationPolicy, "termination-policy", "Delete", "Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut)")
	cmd.Flags().StringArrayVar(&f.TopologyKeys, "topology-keys", nil, "Topology keys for affinity")
	cmd.Flags().StringToStringVar(&f.NodeLabels, "node-labels", nil, "Node label selector")
	cmd.Flags().StringSliceVar(&f.TolerationsRaw, "tolerations", nil, `Tolerations for cluster, such as "key=value:effect, key:effect", for example '"engineType=mongo:NoSchedule", "diskType:NoSchedule"'`)
	cmd.Flags().StringVar(&f.Tenancy, "tenancy", "SharedNode", "Tenancy options, one of: (SharedNode, DedicatedNode)")
	cmd.Flags().BoolVar(&f.BackupEnabled, "backup-enabled", false, "Specify whether enabled automated backup")
	cmd.Flags().StringVar(&f.BackupRetentionPeriod, "backup-retention-period", "1d", "a time string ending with the 'd'|'D'|'h'|'H' character to describe how long the Backup should be retained")
	cmd.Flags().StringVar(&f.BackupMethod, "backup-method", "", "the backup method, view it by \"kbcli cd describe <cluster-definition>\", if not specified, the default backup method will be to take snapshots of the volume")
	cmd.Flags().StringVar(&f.BackupCronExpression, "backup-cron-expression", "", "the cron expression for schedule, the timezone is in UTC. see https://en.wikipedia.org/wiki/Cron.")
	cmd.Flags().Int64Var(&f.BackupStartingDeadlineMinutes, "backup-starting-deadline-minutes", 0, "the deadline in minutes for starting the backup job if it misses its scheduled time for any reason")
	cmd.Flags().StringVar(&f.BackupRepoName, "backup-repo-name", "", "the backup repository name")
	cmd.Flags().BoolVar(&f.BackupPITREnabled, "pitr-enabled", false, "Specify whether enabled point in time recovery")

	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"termination-policy",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"DoNotTerminate\tblock delete operation",
				"Halt\tdelete workload resources such as statefulset, deployment workloads but keep PVCs",
				"Delete\tbased on Halt and deletes PVCs",
				"WipeOut\tbased on Delete and wipe out all volume snapshots and snapshot data from backup storage location",
			}, cobra.ShellCompDirectiveNoFileComp
		}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"pod-anti-affinity",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"Preferred\ttry to spread pods of the cluster by the specified topology-keys",
				"Required\tmust spread pods of the cluster by the specified topology-keys",
			}, cobra.ShellCompDirectiveNoFileComp
		}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"tenancy",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"SharedNode\tpods of the cluster may share the same node",
				"DedicatedNode\teach pod of the cluster will run on their own dedicated node",
			}, cobra.ShellCompDirectiveNoFileComp
		}))
}

// validateStorageClass checks the existence of declared StorageClasses in volume claim templates,
// if not set, check the existence of the default StorageClasses
func validateStorageClass(dynamic dynamic.Interface, components []map[string]interface{}) error {
	existedStorageClasses, existedDefault, err := getStorageClasses(dynamic)
	if err != nil {
		return err
	}
	for _, comp := range components {
		compObj := appsv1alpha1.ClusterComponentSpec{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(comp, &compObj)
		if err != nil {
			return err
		}
		for _, vct := range compObj.VolumeClaimTemplates {
			name := vct.Spec.StorageClassName
			if name != nil {
				// validate the specified StorageClass whether exist
				if _, ok := existedStorageClasses[*name]; !ok {
					return fmt.Errorf("failed to find the specified storageClass \"%s\"", *name)
				}
			} else if !existedDefault {
				// validate the default StorageClass
				return fmt.Errorf("failed to find the default storageClass, use '--set storageClass=NAME' to set it")
			}
		}
	}
	return nil
}

// getStorageClasses returns all StorageClasses in K8S and return true if the cluster have a default StorageClasses
func getStorageClasses(dynamic dynamic.Interface) (map[string]struct{}, bool, error) {
	gvr := types.StorageClassGVR()
	allStorageClasses := make(map[string]struct{})
	existedDefault := false
	list, err := dynamic.Resource(gvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, false, err
	}
	for _, item := range list.Items {
		allStorageClasses[item.GetName()] = struct{}{}
		annotations := item.GetAnnotations()
		if !existedDefault && annotations != nil && (annotations[storage.IsDefaultStorageClassAnnotation] == annotationTrueValue || annotations[storage.BetaIsDefaultStorageClassAnnotation] == annotationTrueValue) {
			existedDefault = true
		}
	}
	// for cloud k8s we will check the kubeblocks-manager-config
	if existedDefault {
		return allStorageClasses, existedDefault, nil
	}
	existedDefault, err = validateDefaultSCInConfig(dynamic)
	return allStorageClasses, existedDefault, err
}

// validateClusterVersion checks the existence of declared cluster version,
// if not set, check the existence of default cluster version
func (o *CreateOptions) validateClusterVersion() error {
	var err error

	// cluster version is specified, validate if exists
	if o.ClusterVersionRef != "" {
		if err = cluster.ValidateClusterVersion(o.Dynamic, o.ClusterDefRef, o.ClusterVersionRef); err != nil {
			return fmt.Errorf("cluster version \"%s\" does not exist, run following command to get the available cluster versions\n\tkbcli cv list --cluster-definition=%s",
				o.ClusterVersionRef, o.ClusterDefRef)
		}
		return nil
	}

	// cluster version is not specified, get the default cluster version
	if o.ClusterVersionRef, err = cluster.GetDefaultVersion(o.Dynamic, o.ClusterDefRef); err != nil {
		return err
	}

	dryRun, err := o.GetDryRunStrategy()
	if err != nil {
		return err
	}
	// if dryRun is set, run in quiet mode, avoid to output yaml file with the info
	if dryRun != create.DryRunNone {
		return nil
	}

	fmt.Fprintf(o.Out, "Info: --cluster-version is not specified, ClusterVersion %s is applied by default\n", o.ClusterVersionRef)
	return nil
}

func buildResourceLabels(clusterName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:  clusterName,
		constant.AppManagedByLabelKey: "kbcli",
	}
}

// build the cluster definition
// if the cluster definition is not specified, pick the cluster definition in the cluster component
// if neither of them is specified, return an error
func (o *CreateOptions) buildClusterDef(cls *appsv1alpha1.Cluster) error {
	if o.ClusterDefRef != "" {
		return nil
	}

	if cls != nil && cls.Spec.ClusterDefRef != "" {
		o.ClusterDefRef = cls.Spec.ClusterDefRef
		return nil
	}

	return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one, run \"kbcli clusterdefinition list\" to show all cluster definitions")
}

// build the cluster version
// if the cluster version is not specified, pick the cluster version in the cluster component
// if neither of them is specified, pick default cluster version
func (o *CreateOptions) buildClusterVersion(cls *appsv1alpha1.Cluster) {
	if o.ClusterVersionRef != "" {
		return
	}

	if cls != nil && cls.Spec.ClusterVersionRef != "" {
		o.ClusterVersionRef = cls.Spec.ClusterVersionRef
	}
}

func (o *CreateOptions) buildAnnotation(cls *appsv1alpha1.Cluster) {
	if cls == nil {
		return
	}

	if o.Annotations == nil {
		o.Annotations = cls.Annotations
	}
}

func (o *CreateOptions) buildBackupConfig(cls *appsv1alpha1.Cluster) error {
	// if the cls.Backup isn't nil, use the backup config in cluster
	if cls != nil && cls.Spec.Backup != nil {
		o.BackupConfig = cls.Spec.Backup
	}

	// check the flag is ser by user or not
	var flags []*pflag.Flag
	if o.Cmd != nil {
		o.Cmd.Flags().Visit(func(flag *pflag.Flag) {
			// only check the backup flags
			if flag.Name == "backup-enabled" || flag.Name == "backup-retention-period" ||
				flag.Name == "backup-method" || flag.Name == "backup-cron-expression" ||
				flag.Name == "backup-starting-deadline-minutes" || flag.Name == "backup-repo-name" ||
				flag.Name == "pitr-enabled" {
				flags = append(flags, flag)
			}
		})
	}

	// must set backup method when set backup config in cli
	if len(flags) > 0 {
		if o.BackupConfig == nil {
			o.BackupConfig = &appsv1alpha1.ClusterBackup{}
		}

		// get default backup method and all backup methods
		defaultBackupMethod, backupMethodsMap, err := getBackupMethodsFromBackupPolicyTemplates(o.Dynamic, o.ClusterDefRef)
		if err != nil {
			return err
		}

		// if backup method is empty in backup config, use the default backup method
		if o.BackupConfig.Method == "" {
			o.BackupConfig.Method = defaultBackupMethod
		}

		// if the flag is set by user, use the flag value
		for _, flag := range flags {
			switch flag.Name {
			case "backup-enabled":
				o.BackupConfig.Enabled = &o.BackupEnabled
			case "backup-retention-period":
				o.BackupConfig.RetentionPeriod = dpv1alpha1.RetentionPeriod(o.BackupRetentionPeriod)
			case "backup-method":
				if _, ok := backupMethodsMap[o.BackupMethod]; !ok {
					return fmt.Errorf("backup method %s is not supported, please view supported backup methods by \"kbcli cd describe %s\"", o.BackupMethod, o.ClusterDefRef)
				}
				o.BackupConfig.Method = o.BackupMethod
			case "backup-cron-expression":
				if _, err := cron.ParseStandard(o.BackupCronExpression); err != nil {
					return fmt.Errorf("invalid cron expression: %s, please see https://en.wikipedia.org/wiki/Cron", o.BackupCronExpression)
				}
				o.BackupConfig.CronExpression = o.BackupCronExpression
			case "backup-starting-deadline-minutes":
				o.BackupConfig.StartingDeadlineMinutes = &o.BackupStartingDeadlineMinutes
			case "backup-repo-name":
				o.BackupConfig.RepoName = o.BackupRepoName
			case "pitr-enabled":
				o.BackupConfig.PITREnabled = &o.BackupPITREnabled
			}
		}
	}

	return nil
}

// get backup methods from backup policy template
// if method's snapshotVolumes is true, use the method as default method
func getBackupMethodsFromBackupPolicyTemplates(dynamic dynamic.Interface, clusterDefRef string) (string, map[string]struct{}, error) {
	var backupPolicyTemplates []appsv1alpha1.BackupPolicyTemplate
	var defaultBackupPolicyTemplate appsv1alpha1.BackupPolicyTemplate

	obj, err := dynamic.Resource(types.BackupPolicyTemplateGVR()).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", constant.ClusterDefLabelKey, clusterDefRef),
	})
	if err != nil {
		return "", nil, err
	}
	for _, item := range obj.Items {
		var backupPolicyTemplate appsv1alpha1.BackupPolicyTemplate
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &backupPolicyTemplate)
		if err != nil {
			return "", nil, err
		}
		backupPolicyTemplates = append(backupPolicyTemplates, backupPolicyTemplate)
	}

	if len(backupPolicyTemplates) == 0 {
		return "", nil, fmt.Errorf("failed to find backup policy template for cluster definition %s", clusterDefRef)
	}
	// if there is only one backup policy template, use it as default backup policy template
	if len(backupPolicyTemplates) == 1 {
		defaultBackupPolicyTemplate = backupPolicyTemplates[0]
	}
	for _, backupPolicyTemplate := range backupPolicyTemplates {
		if backupPolicyTemplate.Annotations[dptypes.DefaultBackupPolicyTemplateAnnotationKey] == annotationTrueValue {
			defaultBackupPolicyTemplate = backupPolicyTemplate
			break
		}
	}

	var defaultBackupMethod string
	var backupMethodsMap = make(map[string]struct{})
	for _, policy := range defaultBackupPolicyTemplate.Spec.BackupPolicies {
		for _, method := range policy.BackupMethods {
			if boolptr.IsSetToTrue(method.SnapshotVolumes) {
				defaultBackupMethod = method.Name
			}
			backupMethodsMap[method.Name] = struct{}{}
		}
	}
	if defaultBackupMethod == "" {
		return "", nil, fmt.Errorf("failed to find default backup method which snapshotVolumes is true, please check backup policy template for cluster definition %s", clusterDefRef)
	}
	return defaultBackupMethod, backupMethodsMap, nil
}

// parse the cluster component spec
// compatible with old file format that only specifies the components
func parseClusterComponentSpec(compByte []byte) ([]appsv1alpha1.ClusterComponentSpec, error) {
	var compSpecs []appsv1alpha1.ClusterComponentSpec
	var comps []map[string]interface{}
	if err := json.Unmarshal(compByte, &comps); err != nil {
		return nil, err
	}
	for _, comp := range comps {
		var compSpec appsv1alpha1.ClusterComponentSpec
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(comp, &compSpec); err != nil {
			return nil, err
		}
		compSpecs = append(compSpecs, compSpec)
	}

	return compSpecs, nil
}

func setKeys() []string {
	return []string{
		string(keyCPU),
		string(keyType),
		string(keyStorage),
		string(keyMemory),
		string(keyReplicas),
		string(keyClass),
		string(keyStorageClass),
		string(keySwitchPolicy),
	}
}

func storageSetKey() []string {
	return []string{
		string(storageKeyType),
		string(storageKeyName),
		string(storageKeyStorageClass),
		string(storageAccessMode),
		string(storageKeySize),
	}
}

// validateDefaultSCInConfig will verify if the ConfigMap of Kubeblocks is configured with the DEFAULT_STORAGE_CLASS.
// When we install Kubeblocks, certain configurations will be rendered in a ConfigMap named kubeblocks-manager-config.
// You can find the details in deploy/helm/template/configmap.yaml.
func validateDefaultSCInConfig(dynamic dynamic.Interface) (bool, error) {
	// todo:  types.KubeBlocksManagerConfigMapName almost is hard code, add a unique label for kubeblocks-manager-config
	namespace, err := util.GetKubeBlocksNamespaceByDynamic(dynamic)
	if err != nil {
		return false, err
	}
	cfg, err := dynamic.Resource(types.ConfigmapGVR()).Namespace(namespace).Get(context.Background(), types.KubeBlocksManagerConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	var config map[string]interface{}
	if cfg.Object["data"] == nil {
		return false, nil
	}
	data := cfg.Object["data"].(map[string]interface{})
	if data["config.yaml"] == nil {
		return false, nil
	}
	err = yaml.Unmarshal([]byte(data["config.yaml"].(string)), &config)
	if err != nil {
		return false, err
	}
	if config["DEFAULT_STORAGE_CLASS"] == nil {
		return false, nil
	}
	return len(config["DEFAULT_STORAGE_CLASS"].(string)) != 0, nil
}

// buildCompStorages will override the storage configurations by --set, and it fixes out the case where there are multiple pvc's in a component
func buildCompStorages(pvcs []string, cd *appsv1alpha1.ClusterDefinition) (map[string][]map[storageKey]string, error) {
	pvcSets := map[string][]map[storageKey]string{}
	parseKey := func(key string) storageKey {
		for _, k := range storageSetKey() {
			if strings.EqualFold(k, key) {
				return storageKey(k)
			}
		}
		return storageKeyUnknown
	}

	buildPVCMap := func(sets []string) (map[storageKey]string, error) {
		res := map[storageKey]string{}
		for _, set := range sets {
			kv := strings.Split(set, "=")
			if len(kv) != 2 {
				return nil, fmt.Errorf("unknown set format \"%s\", should be like key1=value1", set)
			}

			// only record the supported key
			k := parseKey(kv[0])
			if k == storageKeyUnknown {
				return nil, fmt.Errorf("unknown set key \"%s\", should be one of [%s]", kv[0], strings.Join(storageSetKey(), ","))
			}
			res[k] = kv[1]
		}
		return res, nil
	}

	for _, pvc := range pvcs {
		pvcMap, err := buildPVCMap(strings.Split(pvc, ","))
		if err != nil {
			return nil, err
		}
		if len(pvcMap) == 0 {
			continue
		}
		compDefName := pvcMap[storageKeyType]

		// type is not specified by user, use the default component definition name, now only
		// support cluster definition with one component
		if len(compDefName) == 0 {
			name, err := cluster.GetDefaultCompName(cd)
			if err != nil {
				return nil, err
			}

			// if the number of component definitions is more than one, default use the first one and output a log
			if len(cd.Spec.ComponentDefs) > 1 {
				klog.V(1).Infof("the component is not specified, use the default component \"%s\" in cluster definition \"%s\"", name, cd.Name)
			}
			compDefName = name
		} else {
			// check the type is a valid component definition name
			valid := false
			for _, c := range cd.Spec.ComponentDefs {
				if c.Name == compDefName {
					valid = true
					break
				}
			}
			if !valid {
				return nil, fmt.Errorf("the type \"%s\" is not a valid component definition name", compDefName)
			}
		}

		pvcSets[compDefName] = append(pvcSets[compDefName], pvcMap)
	}
	return pvcSets, nil
}

// rebuildCompStorage will rewrite the cluster component specs with the values in pvcMaps
func rebuildCompStorage(pvcMaps map[string][]map[storageKey]string, specs []*appsv1alpha1.ClusterComponentSpec) []*appsv1alpha1.ClusterComponentSpec {
	validateAccessMode := func(mode string) bool {
		return mode == string(corev1.ReadWriteOnce) || mode == string(corev1.ReadOnlyMany) || mode == string(corev1.ReadWriteMany) || mode == string(corev1.ReadWriteOncePod)
	}

	// todo: now each ClusterComponentVolumeClaimTemplate can only set one AccessModes
	buildClusterComponentVolumeClaimTemplate := func(storageSet map[storageKey]string) appsv1alpha1.ClusterComponentVolumeClaimTemplate {
		// set the default value
		res := appsv1alpha1.ClusterComponentVolumeClaimTemplate{
			Name: cluster.GenerateName(),
			Spec: appsv1alpha1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(viper.GetString(types.CfgKeyClusterDefaultStorageSize)),
					},
				},
			},
		}
		if name, ok := storageSet[storageKeyName]; ok {
			res.Name = name
		}
		if accessMode, ok := storageSet[storageAccessMode]; ok {
			if validateAccessMode(accessMode) {
				res.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(accessMode)}
			} else {
				fmt.Printf("Warning: PV access dode %s is invalid, use `ReadWriteOnce` by default", accessMode)
			}
		}
		if storageClass, ok := storageSet[storageKeyStorageClass]; ok {
			res.Spec.StorageClassName = &storageClass
		}
		if storageSize, ok := storageSet[storageKeySize]; ok {
			res.Spec.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			}
		}
		return res
	}

	for componentNames, pvcs := range pvcMaps {
		var compPvcs []appsv1alpha1.ClusterComponentVolumeClaimTemplate
		for i := range pvcs {
			compPvcs = append(compPvcs, buildClusterComponentVolumeClaimTemplate(pvcs[i]))
		}
		for i := range specs {
			if specs[i].Name == componentNames {
				specs[i].VolumeClaimTemplates = compPvcs
			}
		}
	}
	return specs
}
