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
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
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
	Source               string        `json:"source"`
	SourceEndpointModel  EndpointModel `json:"sourceEndpointModel"`
	Sink                 string        `json:"sink"`
	SinkEndpointModel    EndpointModel `json:"sinkEndpointModel"`
	SelectedDatabase     string        `json:"selectedDatabase"`
	DatabaseType         string        `json:"databaseType"`
	Lag                  string        `json:"lag"`
	Engine               string        `json:"engine"`
	TabelsIncluded       []string      `json:"tablesIncluded"`
	Quota                string        `json:"quota"`
	create.CreateOptions `json:"-"`
}

func NewSync2FoxLakeCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateSync2FoxLakeOptions{CreateOptions: create.CreateOptions{
		Factory:         f,
		IOStreams:       streams,
		CueTemplateName: "sync2foxlake_template.cue",
		GVR:             types.Sync2FoxLakeTaskGVR(),
	}}
	o.CreateOptions.Options = o

	cmd := &cobra.Command{
		Use:               "create NAME",
		Short:             "Create a sync2foxlake task.",
		Example:           createSync2FoxLakeExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.Sync2FoxLakeTaskGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
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

	util.CheckErr(cmd.MarkFlagRequired("source"))
	util.CheckErr(cmd.MarkFlagRequired("sink"))
	util.CheckErr(cmd.MarkFlagRequired("database"))

	return cmd
}

func (o *CreateSync2FoxLakeOptions) Validate() error {
	var err error

	err = o.completeEndpointModel(&o.SourceEndpointModel, o.Source)
	if err != nil {
		return err
	}

	err = o.completeEndpointModel(&o.SinkEndpointModel, o.Sink)
	if err != nil {
		return err
	}

	if o.SinkEndpointModel.EndpointType != ClusterNameEndpointType || o.SinkEndpointModel.EndpointCharacterType != "foxlake" {
		return fmt.Errorf("sink must be clustername of a foxlake cluster")
	}

	return nil
}
func (o *CreateSync2FoxLakeOptions) completeEndpointModel(e *EndpointModel, endpointName string) error {
	var err error
	if err = e.buildFromStr(endpointName); err != nil {
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
