/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudprovider

import (
	"context"
	"io"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"

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

func tfInitAndApply(workingDir string, init bool, stdout, stderr io.Writer, opts ...tfexec.ApplyOption) error {
	ctx := context.Background()
	tf, err := newTerraform(workingDir, stdout, stderr)
	if err != nil {
		return err
	}

	if init {
		if err = tf.Init(ctx, tfexec.Upgrade(true)); err != nil {
			return err
		}
	}

	if err = tf.Apply(ctx, opts...); err != nil {
		return err
	}
	return nil
}

func tfDestroy(workingDir string, stdout, stderr io.Writer, opts ...tfexec.DestroyOption) error {
	ctx := context.Background()
	tf, err := newTerraform(workingDir, stdout, stderr)
	if err != nil {
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
