/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cloudprovider

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	tfStateFileName = "terraform.tfstate"
)

type TFPlugin struct {
	Name     string
	Registry string
	Source   string
	Version  string
}

var (
	TFExecPath string
)

func initTerraform() error {
	cliHomeDir, err := util.GetCliHomeDir()
	if err != nil {
		return err
	}

	// check if terraform exists
	TFExecPath = filepath.Join(cliHomeDir, product.Terraform.BinaryName())
	v, err := product.Terraform.GetVersion(context.Background(), TFExecPath)
	if err == nil && v != nil {
		return nil
	}

	// does not exist, install it to cli home dir
	installer := &releases.ExactVersion{
		Product:                  product.Terraform,
		Version:                  version.Must(version.NewVersion("1.3.9")),
		Timeout:                  180 * time.Second,
		SkipChecksumVerification: true,
		InstallDir:               cliHomeDir,
	}
	execPath, err := installer.Install(context.Background())
	if err != nil {
		return err
	}
	TFExecPath = execPath
	return nil
}

func tfInitAndApply(workingDir string, stdout, stderr io.Writer, opts ...tfexec.ApplyOption) error {
	ctx := context.Background()
	tf, err := newTerraform(workingDir, stdout, stderr)
	if err != nil {
		return err
	}

	if err = tf.Init(ctx, tfexec.Upgrade(false)); err != nil {
		return err
	}

	if err = tf.Apply(ctx, opts...); err != nil {
		return err
	}
	return nil
}

func tfInitAndDestroy(workingDir string, stdout, stderr io.Writer, opts ...tfexec.DestroyOption) error {
	ctx := context.Background()
	tf, err := newTerraform(workingDir, stdout, stderr)
	if err != nil {
		return err
	}

	if err = tf.Init(ctx, tfexec.Upgrade(false)); err != nil {
		return err
	}

	return tf.Destroy(ctx, opts...)
}

func newTerraform(workingDir string, stdout, stderr io.Writer) (*tfexec.Terraform, error) {
	tf, err := tfexec.NewTerraform(workingDir, TFExecPath)
	if err != nil {
		return nil, err
	}

	tf.SetStdout(stdout)
	tf.SetStderr(stderr)
	return tf, nil
}

func getOutputValues(tfPath string, keys ...outputKey) ([]string, error) {
	stateFile := filepath.Join(tfPath, tfStateFileName)
	content, err := os.ReadFile(stateFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	var state map[string]interface{}
	if err = json.Unmarshal(content, &state); err != nil {
		return nil, err
	}
	outputs, ok := state["outputs"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	vals := make([]string, len(keys))
	for i, k := range keys {
		v, ok := outputs[string(k)].(map[string]interface{})
		if !ok {
			continue
		}
		vals[i] = v["value"].(string)
	}
	return vals, nil
}
