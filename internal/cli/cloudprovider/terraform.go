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
	"os"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
)

type TFPlugin struct {
	Name     string
	Registry string
	Source   string
	Version  string
}

var (
	TFExecPath  string
	TFBaseDir   string
	TFPluginDir string
	providerCfg string
)

func initTerraform() error {
	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion("1.0.6")),
	}
	execPath, err := installer.Install(context.Background())
	if err != nil {
		return err
	}
	TFExecPath = execPath
	return nil
}

func tfInitAndApply(workingDir string, opts ...tfexec.ApplyOption) error {
	ctx := context.Background()

	tf, err := tfexec.NewTerraform(workingDir, TFExecPath)
	if err != nil {
		return err
	}

	tf.SetStdout(os.Stdout)
	tf.SetStderr(os.Stderr)

	if err = tf.Init(ctx, tfexec.Upgrade(true)); err != nil {
		return err
	}

	if err = tf.Apply(ctx, opts...); err != nil {
		return err
	}
	return nil
}

func tfDestroy(workingDir string) error {
	tf, err := tfexec.NewTerraform(workingDir, TFExecPath)
	if err != nil {
		return err
	}
	return tf.Destroy(context.Background())
}
