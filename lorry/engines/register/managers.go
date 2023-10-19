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

package register

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/lorry/engines"
	"github.com/apecloud/kubeblocks/lorry/engines/mongodb"
	"github.com/apecloud/kubeblocks/lorry/engines/mysql"
	"github.com/apecloud/kubeblocks/lorry/engines/polardbx"
	"github.com/apecloud/kubeblocks/lorry/engines/postgres/apecloudpostgres"
	"github.com/apecloud/kubeblocks/lorry/engines/postgres/officalpostgres"
	"github.com/apecloud/kubeblocks/lorry/engines/wesql"
)

type managerNewFunc func(engines.Properties) (engines.DBManager, error)

var managerNewFuncs = make(map[string]managerNewFunc)

// DBPilot runs with a single database engine instance at a time,
// so only one dbmanager is initialized and cached here during execution.
var dbManager engines.DBManager

var configDir string

func init() {
	pflag.StringVar(&configDir, "config-path", "/config/lorry/components/", "Lorry default config directory for builtin type")

	RegisterManagerNewFunc("mysql", "consensus", wesql.NewManager)
	RegisterManagerNewFunc("mysql", "replication", mysql.NewManager)
	RegisterManagerNewFunc("mongodb", "consensus", mongodb.NewManager)
	RegisterManagerNewFunc("polardbx", "consensus", polardbx.NewManager)
	RegisterManagerNewFunc("postgresql", "replication", officalpostgres.NewManager)
	RegisterManagerNewFunc("postgresql", "consensus", apecloudpostgres.NewManager)
}

func RegisterManagerNewFunc(characterType, workloadType string, newFunc managerNewFunc) {
	key := strings.ToLower(characterType + "_" + workloadType)
	managerNewFuncs[key] = newFunc
}

func GetManagerNewFunc(characterType, workloadType string) managerNewFunc {
	key := strings.ToLower(characterType + "_" + workloadType)
	return managerNewFuncs[key]
}

func SetDBManager(manager engines.DBManager) {
	dbManager = manager
}

func GetDBManager() (engines.DBManager, error) {
	if dbManager != nil {
		return dbManager, nil
	}

	return nil, errors.Errorf("no db manager")
}

func InitDBManager() error {
	if dbManager != nil {
		return nil
	}

	ctrl.Log.Info("Initialize DB manager")
	characterType := viper.GetString(constant.KBEnvCharacterType)
	if characterType == "" {
		return fmt.Errorf("%s not set", constant.KBEnvCharacterType)
	}

	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	if workloadType == "" {
		return fmt.Errorf("%s not set", constant.KBEnvWorkloadType)
	}

	err := GetAllComponent(configDir) // find all builtin config file and read
	if err != nil {                   // Handle errors reading the config file
		return errors.Wrap(err, "fatal error config file")
	}

	properties := GetProperties(characterType)
	newFunc := GetManagerNewFunc(characterType, workloadType)
	if newFunc == nil {
		return errors.Errorf("no db manager for characterType %s and workloadType %s", characterType, workloadType)
	}
	mgr, err := newFunc(properties)
	if err != nil {
		return err
	}

	dbManager = mgr
	return nil
}

type Component struct {
	Name string
	Spec ComponentSpec
}

type ComponentSpec struct {
	Version  string
	Metadata []kv
}

type kv struct {
	Name  string
	Value string
}

var Name2Property = map[string]engines.Properties{}

func readConfig(filename string) (string, engines.Properties, error) {
	viper.SetConfigType("yaml")
	viper.SetConfigFile(filename)
	if err := viper.ReadInConfig(); err != nil {
		return "", nil, err
	}
	component := &Component{}
	if err := viper.Unmarshal(component); err != nil {
		return "", nil, err
	}
	properties := make(engines.Properties)
	properties["version"] = component.Spec.Version
	for _, pair := range component.Spec.Metadata {
		properties[pair.Name] = pair.Value
	}
	return component.Name, properties, nil
}

func GetAllComponent(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		name, properties, err := readConfig(dir + "/" + file.Name())
		if err != nil {
			return err
		}
		Name2Property[name] = properties
	}
	return nil
}

func GetProperties(name string) engines.Properties {
	return Name2Property[name]
}
