package dbaas

import (
	"fmt"
)

func getEnvReplacementMapForConnCrential(clusterName string) map[string]string {
	return map[string]string{
		"$(CONN_CREDENTIAL_SECRET_NAME)": fmt.Sprintf("%s-conn-credential", clusterName),
	}
}

func getEnvReplacementMapForAccount(name, passwd string) map[string]string {
	return map[string]string{
		"$(USERNAME)": name,
		"$(PASSWD)":   passwd,
	}
}
