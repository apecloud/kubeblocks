package engine

import corev1 "k8s.io/api/core/v1"

type mariadb struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func (m *mariadb) ConnectCommand(connectInfo *AuthInfo) []string {
	return nil
}

func (m *mariadb) Container() string {
	return ""
}

func (m *mariadb) ConnectExample(info *ConnectionInfo, client string) string {
	return ""
}

func (m *mariadb) ExecuteCommand([]string) ([]string, []corev1.EnvVar, error) {
	return nil, nil, nil
}
