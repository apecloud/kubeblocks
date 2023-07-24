package organization

import (
	"path/filepath"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	CloudContextDir          = "cloud_context"
	CurrentOrgAndContextFile = "current.json"
	ContextFile              = "context.json"
	TokenFile                = "token.json"
)

func GetCurrentOrgAndContextFilePath() (string, error) {
	cliHomeDir, err := util.GetCliHomeDir()
	if err != nil {
		return "", err
	}
	if err != nil {
		return "", err
	}
	filePath := filepath.Join(cliHomeDir, CloudContextDir, CurrentOrgAndContextFile)
	return filePath, nil
}

func GetContextFilePath() (string, error) {
	cliHomeDir, err := util.GetCliHomeDir()
	if err != nil {
		return "", err
	}
	filePath := filepath.Join(cliHomeDir, CloudContextDir, ContextFile)
	return filePath, nil
}

func GetTokenFilePath() (string, error) {
	cliHomeDir, err := util.GetCliHomeDir()
	if err != nil {
		return "", err
	}
	filePath := filepath.Join(cliHomeDir, CloudContextDir, TokenFile)
	return filePath, nil
}
