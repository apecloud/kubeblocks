/*
Copyright 2022 The KubeBlocks Authors

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

package dbaas

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

const (
	// configurations to connect to Mysql, either a data source name represent by URL.
	connectionURLKey = "url"

	// other general settings for DB connections.
	maxIdleConnsKey    = "maxIdleConns"
	maxOpenConnsKey    = "maxOpenConns"
	connMaxLifetimeKey = "connMaxLifetime"
	connMaxIdleTimeKey = "connMaxIdleTime"
)

// Mysql represents MySQL output bindings.
type Mysql struct {
	db *sql.DB
}

// Init initializes the MySQL binding.
func (m *Mysql) Init(metadata map[string]string) error {
	p := metadata
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return fmt.Errorf("missing MySql connection string")
	}

	db, err := initDB(url)
	if err != nil {
		return err
	}

	err = propertyToInt(p, maxIdleConnsKey, db.SetMaxIdleConns)
	if err != nil {
		return err
	}

	err = propertyToInt(p, maxOpenConnsKey, db.SetMaxOpenConns)
	if err != nil {
		return err
	}

	err = propertyToDuration(p, connMaxIdleTimeKey, db.SetConnMaxIdleTime)
	if err != nil {
		return err
	}

	err = propertyToDuration(p, connMaxLifetimeKey, db.SetConnMaxLifetime)
	if err != nil {
		return err
	}

	err = db.Ping()
	if err != nil {
		return errors.Wrap(err, "unable to ping the DB")
	}

	m.db = db

	return nil
}

// Close will close the DB.
func (m *Mysql) Close() error {
	if m.db != nil {
		return m.db.Close()
	}

	return nil
}

func (m *Mysql) query(ctx context.Context, sql string) ([]interface{}, error) {
	rows, err := m.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", sql)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	result, err := m.jsonify(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshalling query result for %s", sql)
	}

	return result, nil
}

func propertyToInt(props map[string]string, key string, setter func(int)) error {
	if v, ok := props[key]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			setter(i)
		} else {
			return errors.Wrapf(err, "error converitng %s:%s to int", key, v)
		}
	}

	return nil
}

func propertyToDuration(props map[string]string, key string, setter func(time.Duration)) error {
	if v, ok := props[key]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			setter(d)
		} else {
			return errors.Wrapf(err, "error converitng %s:%s to time duration", key, v)
		}
	}

	return nil
}

func initDB(url string) (*sql.DB, error) {
	if _, err := mysql.ParseDSN(url); err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", connectionURLKey)
	}

	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, errors.Wrap(err, "error opening DB connection")
	}

	return db, nil
}

func (m *Mysql) jsonify(rows *sql.Rows) ([]interface{}, error) {
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	var ret []interface{}
	for rows.Next() {
		values := prepareValues(columnTypes)
		err := rows.Scan(values...)
		if err != nil {
			return nil, err
		}

		r := m.convert(columnTypes, values)
		ret = append(ret, r)
	}

	return ret, nil
}

func prepareValues(columnTypes []*sql.ColumnType) []interface{} {
	types := make([]reflect.Type, len(columnTypes))
	for i, tp := range columnTypes {
		types[i] = tp.ScanType()
	}

	values := make([]interface{}, len(columnTypes))
	for i := range values {
		values[i] = reflect.New(types[i]).Interface()
	}

	return values
}

func (m *Mysql) convert(columnTypes []*sql.ColumnType, values []interface{}) map[string]interface{} {
	r := map[string]interface{}{}

	for i, ct := range columnTypes {
		value := values[i]

		switch v := values[i].(type) {
		case driver.Valuer:
			if vv, err := v.Value(); err == nil {
				value = interface{}(vv)
			}
		case *sql.RawBytes:
			// special case for sql.RawBytes, see https://github.com/go-sql-driver/mysql/blob/master/fields.go#L178
			switch ct.DatabaseTypeName() {
			case "VARCHAR", "CHAR", "TEXT", "LONGTEXT":
				value = string(*v)
			}
		}

		if value != nil {
			r[ct.Name()] = value
		}
	}

	return r
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func observeRole(ip string, port int32) (string, error) {
	url := "root@tcp(" + ip + ":" + strconv.Itoa(int(port)) + ")/information_schema?allowNativePasswords=true"
	sql := "select role from information_schema.wesql_cluster_local"
	mysql := &Mysql{}
	params := map[string]string{connectionURLKey: url}
	err := mysql.Init(params)
	if err != nil {
		return "", err
	}

	result, err := mysql.query(context.Background(), sql)
	if err != nil {
		return "", err
	}
	if len(result) != 1 {
		return "", errors.New("only one role should be observed")
	}
	row, ok := result[0].(map[string]interface{})
	if !ok {
		return "", errors.New("query result wrong type")
	}
	role, ok := row["role"].(string)
	if !ok {
		return "", errors.New("role parsing error")
	}
	if len(role) == 0 {
		return "", errors.New("got empty role")
	}

	err = mysql.Close()
	role = strings.ToLower(role)
	if err != nil {
		return role, err
	}

	return role, nil
}

func startPortForward(kind, name string, port int32) error {
	portStr := strconv.Itoa(int(port))
	cmd := exec.Command("bash", "-c", "kubectl port-forward "+kind+"/"+name+" --address 0.0.0.0 "+portStr+":"+portStr+" &")
	return cmd.Start()
}

func stopPortForward(name string) error {
	cmd := exec.Command("bash", "-c", "ps aux | grep port-forward | grep -v grep | grep "+name+" | awk '{print $2}' | xargs kill -9")
	return cmd.Run()
}

func observeRoleOfPod(pod *corev1.Pod) (string, error) {
	kind := "pod"
	name := pod.Name
	ip := getLocalIP()
	port := pod.Spec.Containers[0].Ports[0].ContainerPort
	_ = startPortForward(kind, name, port)
	time.Sleep(time.Second)
	role, err := observeRole(ip, port)
	_ = stopPortForward(name)

	return role, err
}

func setupConsensusRoleObserveFallbackMethod(cli client.Client) {
	fallbackRoleObserve := func(ctx context.Context) {
		namespace := defaultNamespace
		// list all clusters
		clusterList := &dbaasv1alpha1.ClusterList{}
		err := cli.List(ctx, clusterList, &client.ListOptions{Namespace: namespace})
		if err != nil {
			return
		}

		// filter pods running wesql
		pods := make([]corev1.Pod, 0)
		for _, cluster := range clusterList.Items {
			for _, component := range cluster.Spec.Components {
				componentDef, err := getComponent(ctx, cli, &cluster, component.Type)
				if err != nil {
					continue
				}
				if componentDef.ComponentType != dbaasv1alpha1.Consensus {
					continue
				}

				podList := &corev1.PodList{}
				stsName := cluster.Name + "-" + component.Name
				selector, err := labels.Parse(appInstanceLabelKey + "=" + stsName)
				if err != nil {
					continue
				}
				err = cli.List(ctx, podList,
					&client.ListOptions{Namespace: cluster.Namespace},
					client.MatchingLabelsSelector{Selector: selector})
				if err != nil {
					continue
				}
				pods = append(pods, podList.Items...)
			}
		}

		// do fallback role observing: query role by mysql client driver
		for _, pod := range pods {
			role, err := observeRoleOfPod(&pod)
			if err != nil {
				continue
			}

			patch := client.MergeFrom(pod.DeepCopy())
			pod.Labels[consensusSetRoleLabelKey] = role
			err = cli.Patch(ctx, &pod, patch)
			if err != nil {
				continue
			}
		}
	}
	go wait.UntilWithContext(context.TODO(), fallbackRoleObserve, time.Second*5)
}
