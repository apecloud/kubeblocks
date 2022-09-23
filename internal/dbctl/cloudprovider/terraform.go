package cloudprovider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
	terraform "github.com/hashicorp/terraform/libterraform"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
)

type TFPlugin struct {
	Name     string
	Registry string
	Source   string
	Version  string
}

var (
	CLIBaseDir  string
	TFBaseDir   string
	TFPluginDir string
	providerCfg string
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(errors.Wrap(err, "Failed to get current user home dir"))
	}
	CLIBaseDir = path.Join(homeDir, ".opendbaas")
	TFBaseDir = path.Join(CLIBaseDir, "terraform")
	TFPluginDir = path.Join(TFBaseDir, "providers")

	providerCfg = path.Join(CLIBaseDir, "cloud_provider.json")
}

func NewTFPlugin(name, registry, source, version string) *TFPlugin {
	return &TFPlugin{
		Name:     name,
		Registry: registry,
		Source:   source,
		Version:  version,
	}
}

func (p *TFPlugin) Install() error {
	pluginPath := path.Join(
		TFPluginDir,
		p.Registry,
		p.Source,
		p.Version,
		fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH),
		p.Name,
	)
	if err := os.MkdirAll(path.Dir(pluginPath), os.FileMode(0700)); err != nil {
		return errors.Wrap(err, "Failed to create plugin dir")
	}

	if stat, err := os.Stat(pluginPath); err == nil {
		if stat.Size() > 0 {
			util.Infof("Plugin %s has already exists, skip downloading", p.Source)
			return nil
		} else if err := os.RemoveAll(pluginPath); err != nil {
			return errors.Wrap(err, "Failed to remove corrupted plugin")
		}
	} else {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, fmt.Sprintf("Failed to check if plugin %s exists", p.Source))
		}
	}

	util.Infof("Downloading plugin %s", p.Source)
	// Create the file
	out, err := os.Create(pluginPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// TODO optimize, move to another place
	// Get the data
	resp, err := http.Get(fmt.Sprintf("http://54.223.93.54:8000/infracreate/v0.2.0/%s", p.Name))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	if err := os.Chmod(pluginPath, os.FileMode(0700)); err != nil {
		return err
	}
	return nil
}

func tfApply(template string, tfDir string, destroy bool) error {
	if err := os.MkdirAll(tfDir, 0700); err != nil {
		return errors.Wrap(err, "Failed to create terraform working directory")
	}

	wd, _ := os.Getwd()
	// nolint
	defer os.Chdir(wd)

	if err := os.Chdir(tfDir); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to change working directory to %s", tfDir))
	}

	tfCfg := path.Join(tfDir, "demo.tf")
	var args []string
	if err := ioutils.AtomicWriteFile(tfCfg, []byte(template), 0700); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to create %s", tfCfg))
	}

	if _, err := os.Stat(tfCfg); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Terraform config %s not exists", tfCfg))
	}

	cmd := fmt.Sprintf("terraform -chdir=%s", tfDir)

	// terraform init
	args = []string{cmd, "init", fmt.Sprintf("-plugin-dir=%s", TFPluginDir)}
	util.Infof("Execute terraform init: %s", strings.Join(args, " "))
	if err := terraform.RunCli(args); err != nil {
		return errors.Wrap(err, "Failed to init terraform project")
	}

	// terraform apply
	args = []string{cmd, "apply", "-auto-approve"}
	if destroy {
		args = append(args, "-destroy")
	}
	util.Infof("Execute terraform apply: %s", strings.Join(args, " "))
	if err := terraform.RunCli(args); err != nil {
		return errors.Wrap(err, "Failed to apply resources")
	}
	return nil
}

func parseInstancePublicIP(stateFile string) (string, error) {
	content, err := os.ReadFile(stateFile)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read terraform state")
	}
	var state map[string]interface{}
	if err := json.Unmarshal(content, &state); err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal terraform state")
	}
	resources := state["resources"].([]interface{})
	var result string
	for _, item := range resources {
		resource := item.(map[string]interface{})
		if resource["type"] != "aws_instance" {
			continue
		}
		instances, ok := resource["instances"].([]interface{})
		if !ok {
			return "", errors.Wrap(nil, "Failed to find instances")
		}
		instance := instances[0].(map[string]interface{})
		attributes := instance["attributes"].(map[string]interface{})
		result = attributes["public_ip"].(string)
		break
	}
	if result == "" {
		return "", errors.New("Failed to find instance public IP")
	}
	return result, nil
}
