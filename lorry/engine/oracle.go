package engine

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

var _ ClusterCommands = &oracle{}

type oracle struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

// sqlplus sys/$ORACLE_PWD@//localhost:1521/$ORACLE_SID as sysdba
func newOracle() *oracle {
	return &oracle{
		info: EngineInfo{
			Client:      "sqlplus",
			PasswordEnv: "$ORACLE_PWD",
			Database:    "$ORACLE_SID",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# sqlplus client connection example
sqlplus sys/%s@//localhost:1521/%s as sysdba`, info.Password, info.Database)
			},
		},
	}
}

func (c *oracle) ConnectCommand(info *AuthInfo) []string {
	userPass := c.info.PasswordEnv
	serviceSID := c.info.Database
	dsn := fmt.Sprintf("sqlplus sys/%s@//localhost:1521/%s as sysdba", userPass, serviceSID)
	return []string{"sh", "-c", dsn}
}

func (c *oracle) Container() string {
	return c.info.Container
}

func (c *oracle) ConnectExample(info *ConnectionInfo, client string) string {
	return buildExample(info, client, c.examples)
}

func (c *oracle) ExecuteCommand(strings []string) ([]string, []corev1.EnvVar, error) {
	return nil, nil, fmt.Errorf("opengauss execute cammand interface do not implement")
}
