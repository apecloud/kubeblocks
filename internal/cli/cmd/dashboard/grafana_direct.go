package dashboard

import (
	"fmt"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/spf13/cobra"
	"strings"
)

var (
	characterType  string
	availableTypes = [...]string{
		"apecloud-mysql",
		"cadvisor",
		"jmx",
		"kafka",
		"mongodb",
		"node",
		"postgresql",
		"redis",
		"weaviate",
	}
)

func addCharacterFlag(cmd *cobra.Command, characterType *string) {
	cmd.Flags().StringVar(characterType, "character-type", "", "the cluster character type opened directly. eg 'apecloud-mysql'")
	util.CheckErr(cmd.RegisterFlagCompletionFunc("character-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var name []string
		for i := range availableTypes {
			if strings.HasPrefix(availableTypes[i], toComplete) {
				name = append(name, availableTypes[i])
			}
		}
		return name, cobra.ShellCompDirectiveNoFileComp
	}))
}

func buildGrafanaDirectUrl(url *string, targetType string) error {
	if targetType == "" {
		return nil
	}
	for i := range availableTypes {
		if targetType == availableTypes[i] {
			*url += "/d/" + availableTypes[i]
			return nil
		}
	}
	return fmt.Errorf("input an invalid character type, please check your input")
}
