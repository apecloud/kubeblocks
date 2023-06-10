package helm

import (
	"bufio"
	_ "gopkg.in/yaml.v2"
	"strings"
)

var yamlSeparator = []byte("\n---\n")

// MappingResult to store result of diff
type MappingResult struct {
	Name    string
	Kind    string
	Content string
}

// GetChartManifestByDryRun get the manifest of the chart which we want to install or upgrade
func (i *InstallOpts) GetChartManifestByDryRun(cfg *Config) (string, error) {
	actionCfg, err := NewActionConfig(cfg)
	if err != nil {
		return "", err
	}
	release, err := i.tryUpgrade(actionCfg)
	if err != nil {
		return "", err
	}

	return release.Manifest, nil
}

func Parse(manifest string) map[string]*MappingResult {
	scanner := bufio.NewScanner(strings.NewReader("\n" + manifest))
	scanner.Split(scanYamlSpecs)
	// Allow for tokens (specs) up to 10MiB in size
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), 10485760)

	result := make(map[string]*MappingResult)
	return nil
}
