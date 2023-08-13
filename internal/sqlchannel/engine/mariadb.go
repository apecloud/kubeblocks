package engine

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"strings"
)

type mariadb struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newMariadb() *mariadb {
	return &mariadb{
		info: EngineInfo{
			Client:      "mariadb",
			PasswordEnv: "$MARIADB_ROOT_PASSWORD",
			UserEnv:     "$MARIADB_ROOT_USER",
			Database:    "mysql",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# mysql client connection example
mysql -h %s -P %s -u %s -p%s
`, info.Host, info.Port, info.User, info.Password)
			},
		},
	}
}

func (m *mariadb) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := m.info.UserEnv
	userPass := m.info.PasswordEnv

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	// avoid using env variables
	// MYSQL_PWD is deprecated as of MySQL 8.0; expect it to be removed in a future version of MySQL.
	// ref to mysql manual for more details.
	// https://dev.mysql.com/doc/refman/8.0/en/environment-variables.html
	mysqlCmd := []string{fmt.Sprintf("%s -u%s -p%s", m.info.Client, userName, userPass)}

	return []string{"sh", "-c", strings.Join(mysqlCmd, " ")}
}

func (m *mariadb) Container() string {
	return m.info.Container
}

func (m *mariadb) ConnectExample(info *ConnectionInfo, client string) string {
	if len(info.Database) == 0 {
		info.Database = m.info.Database
	}
	return buildExample(info, client, m.examples)
}

func (m *mariadb) ExecuteCommand(scripts []string) ([]string, []corev1.EnvVar, error) {
	cmd := []string{}
	cmd = append(cmd, "/bin/sh", "-c", "-ex")
	cmd = append(cmd, fmt.Sprintf("%s -u%s -p%s -e %s", m.info.Client,
		fmt.Sprintf("$%s", envVarMap[user]),
		fmt.Sprintf("$%s", envVarMap[password]),
		strconv.Quote(strings.Join(scripts, " "))))

	envs := []corev1.EnvVar{
		{
			Name:  "MYSQL_HOST",
			Value: fmt.Sprintf("$(%s)", envVarMap[host]),
		},
	}
	return cmd, envs, nil
}
