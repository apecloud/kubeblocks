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
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var clusterCreateExample = templates.Examples(`
	# Create a cluster with cluster definition apecloud-mysql and cluster version ac-mysql-8.0.30
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --cluster-version ac-mysql-8.0.30

	# --cluster-definition is required, if --cluster-version is not specified, will use the most recently created version
	kbcli cluster create mycluster --cluster-definition apecloud-mysql

	# Create a cluster and set termination policy DoNotTerminate that will prevent the cluster from being deleted
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy DoNotTerminate

	# In scenarios where you want to delete resources such as statements, deployments, services, pdb, but keep PVCs
	# when deleting the cluster, use termination policy Halt
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy Halt

	# In scenarios where you want to delete resource such as statements, deployments, services, pdb, and including
	# PVCs when deleting the cluster, use termination policy Delete
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy Delete

	# In scenarios where you want to delete all resources including all snapshots and snapshot data when deleting
	# the cluster, use termination policy WipeOut
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy WipeOut

	# Create a cluster and set cpu to 1 core, memory to 1Gi, storage size to 20Gi and replicas to 3
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --set cpu=1,memory=1Gi,storage=20Gi,replicas=3

	# Create a cluster and set the class to general-1c1g
	# run "kbcli class list --cluster-definition=cluster-definition-name" to get the class list
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --set class=general-1c1g

	# Create a cluster with replicationSet workloadType and set switchPolicy to Noop
	kbcli cluster create mycluster --cluster-definition postgresql --set switchPolicy=Noop

	# Create a cluster and use a URL to set cluster resource
	kbcli cluster create mycluster --cluster-definition apecloud-mysql \
		--set-file https://kubeblocks.io/yamls/apecloud-mysql.yaml

	# Create a cluster and load cluster resource set from stdin
	cat << EOF | kbcli cluster create mycluster --cluster-definition apecloud-mysql --set-file -
	- name: my-test ...

	# Create a cluster forced to scatter by node
	kbcli cluster create --cluster-definition apecloud-mysql --topology-keys kubernetes.io/hostname \
		--pod-anti-affinity Required

	# Create a cluster in specific labels nodes
	kbcli cluster create --cluster-definition apecloud-mysql \
		--node-labels '"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'

	# Create a Cluster with two tolerations 
	kbcli cluster create --cluster-definition apecloud-mysql --tolerations \ '"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'

    # Create a cluster, with each pod runs on their own dedicated node
    kbcli cluster create --cluster-definition apecloud-mysql --tenancy=DedicatedNode
`)

const (
	CueTemplateName = "cluster_template.cue"
	monitorKey      = "monitor"
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

type envSet struct {
	name       string
	defaultVal string
}

var setKeyEnvMap = map[setKey]envSet{
	keyCPU:      {"CLUSTER_DEFAULT_CPU", "1000m"},
	keyMemory:   {"CLUSTER_DEFAULT_MEMORY", "1Gi"},
	keyStorage:  {"CLUSTER_DEFAULT_STORAGE_SIZE", "20Gi"},
	keyReplicas: {"CLUSTER_DEFAULT_REPLICAS", "1"},
}

// UpdatableFlags is the flags that cat be updated by update command
type UpdatableFlags struct {
	// Options for cluster termination policy
	TerminationPolicy string `json:"terminationPolicy"`

	// Add-on switches for cluster observability
	Monitor       bool `json:"monitor"`
	EnableAllLogs bool `json:"enableAllLogs"`

	// Configuration and options for cluster affinity and tolerations
	PodAntiAffinity string `json:"podAntiAffinity"`
	// TopologyKeys if TopologyKeys is nil, add omitempty json tag, because CueLang can not covert null to list.
	TopologyKeys   []string          `json:"topologyKeys,omitempty"`
	NodeLabels     map[string]string `json:"nodeLabels,omitempty"`
	Tenancy        string            `json:"tenancy"`
	TolerationsRaw []string          `json:"-"`
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

	// backup name to restore in creation
	Backup string `json:"backup,omitempty"`
	UpdatableFlags
	create.BaseOptions
}

func setMonitor(monitor bool, components []map[string]interface{}) {
	if len(components) == 0 {
		return
	}
	for _, component := range components {
		component[monitorKey] = monitor
	}
}

func getRestoreFromBackupAnnotation(backup *dataprotectionv1alpha1.Backup, compSpecsCount int, firstCompName string) (string, error) {
	componentName := backup.Labels[constant.KBAppComponentLabelKey]
	if len(componentName) == 0 {
		if compSpecsCount != 1 {
			return "", fmt.Errorf("unable to obtain the name of the component to be recovered, please ensure that Backup.status.componentName exists")
		}
		componentName = firstCompName
	}
	restoreFromBackupAnnotation := fmt.Sprintf(`{"%s":"%s"}`, componentName, backup.Name)
	return restoreFromBackupAnnotation, nil
}

func setBackup(o *CreateOptions, components []map[string]interface{}) error {
	backupName := o.Backup
	if len(backupName) == 0 || len(components) == 0 {
		return nil
	}
	backup := &dataprotectionv1alpha1.Backup{}
	if err := cluster.GetK8SClientObject(o.Dynamic, backup, types.BackupGVR(), o.Namespace, backupName); err != nil {
		return err
	}
	if backup.Status.Phase != dataprotectionv1alpha1.BackupCompleted {
		return fmt.Errorf(`backup "%s" is not completed`, backup.Name)
	}
	restoreAnnotation, err := getRestoreFromBackupAnnotation(backup, len(components), components[0]["name"].(string))
	if err != nil {
		return err
	}
	if o.Annotations == nil {
		o.Annotations = map[string]string{}
	}
	o.Annotations[constant.RestoreFromBackUpAnnotationKey] = restoreAnnotation
	return nil
}

func (o *CreateOptions) Validate() error {
	if o.ClusterDefRef == "" {
		return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one, run \"kbcli clusterdefinition list\" to show all cluster definition")
	}

	if o.TerminationPolicy == "" {
		return fmt.Errorf("a valid termination policy is needed, use --termination-policy to specify one of: DoNotTerminate, Halt, Delete, WipeOut")
	}

	if o.ClusterVersionRef == "" {
		version, err := cluster.GetLatestVersion(o.Dynamic, o.ClusterDefRef)
		if err != nil {
			return err
		}
		o.ClusterVersionRef = version
		fmt.Fprintf(o.Out, "Info: --cluster-version is not specified, ClusterVersion %s is applied by default\n", o.ClusterVersionRef)
	}

	if len(o.Values) > 0 && len(o.SetFile) > 0 {
		return fmt.Errorf("does not support --set and --set-file being specified at the same time")
	}

	// if name is not specified, generate a random cluster name
	if o.Name == "" {
		name, err := generateClusterName(o.Dynamic, o.Namespace)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("failed to generate a random cluster name")
		}
		o.Name = name
	}
	if len(o.Name) > 16 {
		return fmt.Errorf("cluster name should be less than 16 characters")
	}

	return nil
}

func (o *CreateOptions) Complete() error {
	if err := o.Validate(); err != nil {
		return err
	}

	components, err := o.buildComponents()
	if err != nil {
		return err
	}

	setMonitor(o.Monitor, components)
	if err := setBackup(o, components); err != nil {
		return err
	}
	o.ComponentSpecs = components

	// TolerationsRaw looks like `["key=engineType,value=mongo,operator=Equal,effect=NoSchedule"]` after parsing by cmd
	tolerations := buildTolerations(o.TolerationsRaw)
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

// buildComponents build components from file or set values
func (o *CreateOptions) buildComponents() ([]map[string]interface{}, error) {
	var (
		err       error
		cd        *appsv1alpha1.ClusterDefinition
		compSpecs []*appsv1alpha1.ClusterComponentSpec
	)

	compClasses, err := class.ListClassesByClusterDefinition(o.Dynamic, o.ClusterDefRef)
	if err != nil {
		return nil, err
	}

	cd, err = cluster.GetClusterDefByName(o.Dynamic, o.ClusterDefRef)
	if err != nil {
		return nil, err
	}

	// build components from file
	if len(o.SetFile) > 0 {
		var (
			compByte []byte
			comps    []map[string]interface{}
		)
		if compByte, err = MultipleSourceComponents(o.SetFile, o.IOStreams.In); err != nil {
			return nil, err
		}
		if compByte, err = yaml.YAMLToJSON(compByte); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(compByte, &comps); err != nil {
			return nil, err
		}
		for _, comp := range comps {
			var compSpec appsv1alpha1.ClusterComponentSpec
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(comp, &compSpec); err != nil {
				return nil, err
			}
			compSpecs = append(compSpecs, &compSpec)
		}
	} else {
		// build components from set values or environment variables
		compSets, err := buildCompSetsMap(o.Values, cd)
		if err != nil {
			return nil, err
		}

		compSpecs, err = buildClusterComp(cd, compSets, compClasses)
		if err != nil {
			return nil, err
		}
	}

	var comps []map[string]interface{}
	for _, compSpec := range compSpecs {
		// validate component classes
		if _, err = class.ValidateComponentClass(compSpec, compClasses); err != nil {
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
	saNamePrefix          = "kb-sa-"
	roleNamePrefix        = "kb-role-"
	roleBindingNamePrefix = "kb-rolebinding-"
)

// buildDependenciesFn create dependencies function for components, e.g. postgresql depends on
// a service account, a role and a rolebinding
func (o *CreateOptions) buildDependenciesFn(cd *appsv1alpha1.ClusterDefinition,
	compSpec *appsv1alpha1.ClusterComponentSpec) error {

	// set component service account name
	compSpec.ServiceAccountName = saNamePrefix + o.Name
	return nil
}

func (o *CreateOptions) CreateDependencies(dryRun []string) error {
	var (
		saName          = saNamePrefix + o.Name
		roleName        = roleNamePrefix + o.Name
		roleBindingName = roleBindingNamePrefix + o.Name
	)

	klog.V(1).Infof("create dependencies for cluster %s", o.Name)
	// create service account
	labels := buildResourceLabels(o.Name)
	applyOptions := metav1.ApplyOptions{FieldManager: "kbcli", DryRun: dryRun}
	sa := corev1ac.ServiceAccount(saName, o.Namespace).WithLabels(labels)

	klog.V(1).Infof("create service account %s", saName)
	if _, err := o.Client.CoreV1().ServiceAccounts(o.Namespace).Apply(context.TODO(), sa, applyOptions); err != nil {
		return err
	}

	// create role
	klog.V(1).Infof("create role %s", roleName)
	role := rbacv1ac.Role(roleName, o.Namespace).WithRules([]*rbacv1ac.PolicyRuleApplyConfiguration{
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create"},
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

	if _, err := o.Client.RbacV1().Roles(o.Namespace).Apply(context.TODO(), role, applyOptions); err != nil {
		return err
	}

	// create role binding
	rbacAPIGroup := "rbac.authorization.k8s.io"
	rbacKind := "Role"
	saKind := "ServiceAccount"
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
			Kind:     &rbacKind,
			Name:     &roleName,
		})
	klog.V(1).Infof("create role binding %s", roleBindingName)
	_, err := o.Client.RbacV1().RoleBindings(o.Namespace).Apply(context.TODO(), roleBinding, applyOptions)
	return err
}

// MultipleSourceComponents get component data from multiple source, such as stdin, URI and local file
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

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:                "create [NAME]",
		Short:              "Create a cluster.",
		Example:            clusterCreateExample,
		CueTemplateName:    CueTemplateName,
		ResourceName:       types.ResourceClusters,
		BaseOptionsObj:     &o.BaseOptions,
		Options:            o,
		Factory:            f,
		Complete:           o.Complete,
		PreCreate:          o.PreCreate,
		CleanUpFn:          o.CleanUp,
		CreateDependencies: o.CreateDependencies,
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "Specify cluster definition, run \"kbcli cd list\" to show all available cluster definitions")
			cmd.Flags().StringVar(&o.ClusterVersionRef, "cluster-version", "", "Specify cluster version, run \"kbcli cv list\" to show all available cluster versions, use the latest version if not specified")
			cmd.Flags().StringVarP(&o.SetFile, "set-file", "f", "", "Use yaml file, URL, or stdin to set the cluster resource")
			cmd.Flags().StringArrayVar(&o.Values, "set", []string{}, "Set the cluster resource including cpu, memory, replicas and storage, or you can just specify the class, each set corresponds to a component.(e.g. --set cpu=1,memory=1Gi,replicas=3,storage=20Gi or --set class=general-1c1g)")
			cmd.Flags().StringVar(&o.Backup, "backup", "", "Set a source backup to restore data")

			// add updatable flags
			o.UpdatableFlags.addFlags(cmd)

			// set required flag
			util.CheckErr(cmd.MarkFlagRequired("cluster-definition"))

			// register flag completion func
			registerFlagCompletionFunc(cmd, f)
		},
	}

	return create.BuildCommand(inputs)
}

func registerFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster-definition",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterDefGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster-version",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterVersionGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
}

// PreCreate before commit yaml to k8s, make changes on Unstructured yaml
func (o *CreateOptions) PreCreate(obj *unstructured.Unstructured) error {
	if !o.EnableAllLogs {
		// EnableAllLogs is false, nothing will change
		return nil
	}
	c := &appsv1alpha1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, c); err != nil {
		return err
	}
	// get cluster definition from k8s
	cd, err := cluster.GetClusterDefByName(o.Dynamic, c.Spec.ClusterDefRef)
	if err != nil {
		return err
	}
	setEnableAllLogs(c, cd)
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
		return false, fmt.Errorf("find no cluster componnet")
	}
	compSpec := o.ComponentSpecs[0]
	for i, def := range cd.Spec.ComponentDefs {
		compDefRef := compSpec["componentDefRef"]
		if compDefRef != nil && def.Name == compDefRef.(string) {
			compDef = &cd.Spec.ComponentDefs[i]
		}
	}

	if compDef == nil {
		return false, fmt.Errorf("failed to find component definition for componnet %v", compSpec["Name"])
	}

	// for postgresql, we need to create a service account, a role and a rolebinding
	if compDef.CharacterType != "postgresql" {
		return false, nil
	}
	return true, nil
}

// setEnableAllLog set enable all logs, and ignore enabledLogs of component level.
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

func buildClusterComp(cd *appsv1alpha1.ClusterDefinition, setsMap map[string]map[setKey]string,
	componentClasses map[string]map[string]*appsv1alpha1.ComponentClassInstance) ([]*appsv1alpha1.ClusterComponentSpec, error) {
	// get value from set values and environment variables, the second return value is
	// true if the value is from environment variables
	getVal := func(c *appsv1alpha1.ClusterComponentDefinition, key setKey, sets map[setKey]string) string {
		// get value from set values
		if sets != nil {
			if v := sets[key]; len(v) > 0 {
				return v
			}
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
		env := setKeyEnvMap[key]
		val := viper.GetString(env.name)
		if len(val) == 0 {
			val = env.defaultVal
		}
		return val
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
		sets := map[setKey]string{}
		if setsMap != nil {
			sets = setsMap[c.Name]
		}

		// get replicas
		setReplicas, err := strconv.Atoi(getVal(&c, keyReplicas, sets))
		if err != nil {
			return nil, fmt.Errorf("repicas is illegal " + err.Error())
		}
		replicas := int32(setReplicas)

		compObj := &appsv1alpha1.ClusterComponentSpec{
			Name:            c.Name,
			ComponentDefRef: c.Name,
			Replicas:        replicas,
		}

		// class has higher priority than other resource related parameters
		resourceList := make(corev1.ResourceList)
		if _, ok := componentClasses[c.Name]; ok {
			if className := getVal(&c, keyClass, sets); className != "" {
				compObj.ClassDefRef = &appsv1alpha1.ClassDefRef{Class: className}
			} else {
				if cpu, ok := sets[keyCPU]; ok {
					resourceList[corev1.ResourceCPU] = resource.MustParse(cpu)
				}
				if mem, ok := sets[keyMemory]; ok {
					resourceList[corev1.ResourceMemory] = resource.MustParse(mem)
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
			compObj.VolumeClaimTemplates[i].Spec.StorageClassName = &storageClass
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
	keys := []string{string(keyCPU), string(keyType), string(keyStorage), string(keyMemory), string(keyReplicas), string(keyClass), string(keyStorageClass), string(keySwitchPolicy)}
	parseKey := func(key string) setKey {
		for _, k := range keys {
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
				return nil, fmt.Errorf("unknown set key \"%s\", should be one of [%s]", kv[0], strings.Join(keys, ","))
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

func buildTolerations(raw []string) []interface{} {
	tolerations := make([]interface{}, 0)
	for _, tolerationRaw := range raw {
		toleration := map[string]interface{}{}
		for _, entries := range strings.Split(tolerationRaw, ",") {
			parts := strings.SplitN(entries, "=", 2)
			toleration[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
		tolerations = append(tolerations, toleration)
	}
	return tolerations
}

// generateClusterName generate a random cluster name that does not exist
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
	return "", nil
}

func (f *UpdatableFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.PodAntiAffinity, "pod-anti-affinity", "Preferred", "Pod anti-affinity type, one of: (Preferred, Required)")
	cmd.Flags().BoolVar(&f.Monitor, "monitor", true, "Set monitor enabled and inject metrics exporter")
	cmd.Flags().BoolVar(&f.EnableAllLogs, "enable-all-logs", false, "Enable advanced application all log extraction, and true will ignore enabledLogs of component level, default is false")
	cmd.Flags().StringVar(&f.TerminationPolicy, "termination-policy", "Delete", "Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut)")
	cmd.Flags().StringArrayVar(&f.TopologyKeys, "topology-keys", nil, "Topology keys for affinity")
	cmd.Flags().StringToStringVar(&f.NodeLabels, "node-labels", nil, "Node label selector")
	cmd.Flags().StringSliceVar(&f.TolerationsRaw, "tolerations", nil, `Tolerations for cluster, such as '"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"'`)
	cmd.Flags().StringVar(&f.Tenancy, "tenancy", "SharedNode", "Tenancy options, one of: (SharedNode, DedicatedNode)")

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

// validateStorageClass check whether the StorageClasses we need are exist in K8S or
// the default StorageClasses are exist
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

// getStorageClasses return all StorageClasses in K8S and return true if the cluster have a default StorageClasses
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
	return allStorageClasses, existedDefault, nil
}

func buildResourceLabels(clusterName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:  clusterName,
		constant.AppManagedByLabelKey: "kbcli",
	}
}
