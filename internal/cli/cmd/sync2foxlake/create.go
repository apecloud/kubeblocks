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

package sync2foxlake

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/cli/cli"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var createSync2FoxLakeExample = templates.Examples(`
	# create a sync2foxlake task to synchronize data from mysql:testdb to foxlake
	kbcli sync2foxlake create mytask --source user:12345@127.0.0.1:3306 --sink foxlake-cluster --database testdb

	# create a sync2foxlake task to synchronize data from wesql:testdb to foxlake
	kbcli sync2foxlake create mytask --source wesql-cluster --sink foxlake-cluster --database testdb

	# create a sync2foxlake task with specific latency, engine, and quota
	kbcli sync2foxlake create mytask 
	--source wesql-cluster
	--sink foxlake-cluster
	--database testdb 
	--lag "10 seconds"
	--engine "columnar@myengine"
	--quota "myquota"

	# create a sync2foxlake task to synchronize specific tables in testdb
	kbcli sync2foxlake create mytask
	--source wesql-cluster
	--sink foxlake-cluster
	--database testdb
	--tables-included '"tb1","tb2","tb3"'
`)

type CreateSync2FoxLakeOptions struct {
	Name                string
	Source              string
	SourceEndpointModel EndpointModel
	Sink                string
	SinkEndpointModel   EndpointModel
	SelectedDatabase    string
	DatabaseType        string
	Lag                 string
	Engine              string
	TabelsIncluded      []string
	TabelsExcluded      []string
	Quota               string
	*Sync2FoxLakeExecOptions
}

func NewSync2FoxLakeCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateSync2FoxLakeOptions{Sync2FoxLakeExecOptions: newSync2FoxLakeExecOptions(f, streams)}
	cmd := &cobra.Command{
		Use:               "create NAME",
		Short:             "Create a foxlake synchronized database.",
		Args:              cli.ExactArgs(1),
		Example:           createSync2FoxLakeExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(args))
			util.CheckErr(o.run())
		},
	}
	cmd.Flags().StringVar(&o.Source, "source", "", "Set the source database information for synchronization. Supports clustername or connection string like '{username}:{password}@{connection_address}:{connection_port}'.")
	cmd.Flags().StringVar(&o.Sink, "sink", "", "Set the source database information for synchronization. Supports clustername or connection string like '{username}:{password}@{connection_address}:{connection_port}'.")
	cmd.Flags().StringVar(&o.SelectedDatabase, "database", "", "Set the database name to be synchronized in the source database.")
	cmd.Flags().StringVar(&o.DatabaseType, "database-type", "mysql", "Set the database type of the source database.")
	cmd.Flags().StringVar(&o.Lag, "lag", "1 seconds", "Set the latency of the synchronized database.")
	cmd.Flags().StringVar(&o.Engine, "engine", "columnar@default", "Set the engine of the synchronized database.")
	cmd.Flags().StringVar(&o.Quota, "quota", "default", "Set the quota of the synchronized database.")
	cmd.Flags().StringSliceVar(&o.TabelsIncluded, "tables-included", []string{}, "Set the tables to be synchronized in the source database.")
	cmd.Flags().StringSliceVar(&o.TabelsExcluded, "tables-excluded", []string{}, "Set the tables not to be synchronized in the source database.")

	util.CheckErr(cmd.MarkFlagRequired("source"))
	util.CheckErr(cmd.MarkFlagRequired("sink"))
	util.CheckErr(cmd.MarkFlagRequired("database"))

	return cmd
}

func (o *CreateSync2FoxLakeOptions) complete(args []string) error {
	var err error
	o.Name = args[0]
	if err = o.Sync2FoxLakeExecOptions.complete(); err != nil {
		return err
	}
	if err = o.completeEndpointModel(&o.SourceEndpointModel, o.Source); err != nil {
		return err
	}
	if err = o.completeEndpointModel(&o.SinkEndpointModel, o.Sink); err != nil {
		return err
	}

	if o.SinkEndpointModel.EndpointType != ClusterNameEndpointType || o.SinkEndpointModel.EndpointCharacterType != "foxlake" {
		return fmt.Errorf("sink must be clustername of a foxlake cluster")
	}

	labels := fmt.Sprintf("%s in (%s)", "app.kubernetes.io/instance", o.Sink) + fmt.Sprintf(",%s in (%s)", "apps.kubeblocks.io/component-name", "foxlake-metadb")
	if err != nil {
		return err
	}
	podList, err := o.Client.CoreV1().Pods(o.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pod found with labels %s", labels)
	}
	o.PodName = podList.Items[0].Name

	return nil
}

func (o *CreateSync2FoxLakeOptions) run() error {
	var err error

	if _, hasExists := o.Cm.Data[o.Name]; hasExists {
		return fmt.Errorf("Sync2foxlake task name %s already exists", o.Name)
	}

	if o.Cm.Data == nil {
		o.Cm.Data = make(map[string]string)
	}
	o.Cm.Data[o.Name] = o.SinkEndpointModel.Endpoint + ":" + o.SelectedDatabase + ":" + o.PodName + ":" + o.SinkEndpointModel.Host + ":" + o.SinkEndpointModel.Port + ":" + o.SinkEndpointModel.UserName + ":" + o.SinkEndpointModel.Password

	buildSQL := func(database string) string {
		createQuota := "Create quota if not exists default cpu='64' memory='256Gi';"
		return createQuota + "Create synchronized database " + database +
			" Engine = '" + o.Engine + "' DATASOURCE_TYPE = '" + o.DatabaseType +
			"' DATASOURCE_ENDPOINT = '" + o.SourceEndpointModel.Host + ":" + o.SourceEndpointModel.Port + "' DATASOURCE_USER = '" + o.SourceEndpointModel.UserName +
			"' DATASOURCE_PASSWORD = '" + o.SourceEndpointModel.Password + "' DATABASE_SELECTED = '" + o.SelectedDatabase +
			"' LAG = '" + o.Lag + "' QUOTA = '" + o.Quota + "';"
	}
	if err = o.Sync2FoxLakeExecOptions.run(o.Name, buildSQL); err != nil {
		return err
	}

	if _, err := o.Client.CoreV1().ConfigMaps(o.Namespace).Update(context.TODO(), o.Cm, metav1.UpdateOptions{}); err != nil {
		return err
	}
	fmt.Printf("Sync2foxlake task %s created\n", o.Name)
	return nil
}

func (o *CreateSync2FoxLakeOptions) completeEndpointModel(e *EndpointModel, endpointName string) error {
	var err error
	if err = e.complete(endpointName); err != nil {
		return err
	}
	if e.EndpointType == AddressEndpointType {
		return nil
	}
	getter := cluster.ObjectsGetter{
		Client:    o.Client,
		Dynamic:   o.Dynamic,
		Name:      e.Endpoint,
		Namespace: o.Namespace,
		GetOptions: cluster.GetOptions{
			WithClusterDef: true,
			WithService:    true,
			WithSecret:     true,
		},
	}
	objs, err := getter.Get()
	if err != nil {
		return err
	}
	if len(objs.Cluster.Spec.ComponentSpecs) == 0 {
		return fmt.Errorf("cluster component not found")
	}

	internalSvcs, _ := cluster.GetComponentServices(objs.Services, &objs.Cluster.Spec.ComponentSpecs[0])
	if len(internalSvcs) == 0 {
		return fmt.Errorf("cluster internal service not found")
	}

	userName, password, err := getUserAndPassword(objs.ClusterDef, objs.Secrets)
	if err != nil {
		return err
	}

	e.EndpointCharacterType = objs.ClusterDef.Spec.ComponentDefs[0].CharacterType
	e.Host = internalSvcs[0].Spec.ClusterIP
	e.Port = strconv.Itoa(int(internalSvcs[0].Spec.Ports[0].Port))
	e.UserName = userName
	e.Password = password

	return nil
}

// get cluster user and password from secrets
func getUserAndPassword(clusterDef *appsv1alpha1.ClusterDefinition, secrets *corev1.SecretList) (string, string, error) {
	var (
		user, password = "", ""
		err            error
	)

	if len(secrets.Items) == 0 {
		return user, password, fmt.Errorf("failed to find the cluster username and password")
	}

	getPasswordKey := func(connectionCredential map[string]string) string {
		for k := range connectionCredential {
			if strings.Contains(k, "password") {
				return k
			}
		}
		return "password"
	}

	getSecretVal := func(secret *corev1.Secret, key string) (string, error) {
		val, ok := secret.Data[key]
		if !ok {
			return "", fmt.Errorf("failed to find the cluster %s", key)
		}
		return string(val), nil
	}

	// now, we only use the first secret
	var secret corev1.Secret
	for i, s := range secrets.Items {
		if strings.Contains(s.Name, "conn-credential") {
			secret = secrets.Items[i]
			break
		}
	}
	user, err = getSecretVal(&secret, "username")
	if err != nil {
		return user, password, err
	}

	passwordKey := getPasswordKey(clusterDef.Spec.ConnectionCredential)
	password, err = getSecretVal(&secret, passwordKey)
	return user, password, err
}
