package engine

import (
	"fmt"
	"strings"
)

type wesql struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func (w *wesql) ConnectCommand(connectInfo *AuthInfo) []string {

	// avoid using env variables
	// MYSQL_PWD is deprecated as of MySQL 8.0; expect it to be removed in a future version of MySQL.
	// ref to mysql manual for more details.
	// https://dev.mysql.com/doc/refman/8.0/en/environment-variables.html
	wesqlCmd := []string{fmt.Sprintf("%s -S /tmp/mysql.sock ", w.info.Client)}

	return []string{"sh", "-c", strings.Join(wesqlCmd, " ")}
}

func (w *wesql) Container() string {
	return w.info.Container
}

func (w *wesql) ConnectExample(info *ConnectionInfo, client string) string {
	if len(info.Database) == 0 {
		info.Database = w.info.Database
	}
	return buildExample(info, client, w.examples)
}

var _ Interface = &wesql{}

func newWesql() *wesql {
	return &wesql{
		info: EngineInfo{
			Client:      "mysql",
			PasswordEnv: "$MYSQL_ROOT_PASSWORD",
			UserEnv:     "$MYSQL_ROOT_USER",
			Database:    "mysql",
		},
	}
}
