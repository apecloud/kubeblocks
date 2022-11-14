/*
Copyright ApeCloud Inc.

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

package builder

import (
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type BuildFn func(cmd *Command) *cobra.Command

type CustomCompleteFn func(o interface{}, args []string) error

type CustomFlags func(o interface{}, cmd *cobra.Command)

// Command records the command info
type Command struct {
	Use     string
	Short   string
	Example string
	GVR     schema.GroupVersionResource
	Factory cmdutil.Factory
	// CustomComplete custom complete function for cmd
	CustomComplete CustomCompleteFn
	// CustomFlags custom flags for cmd, return args
	CustomFlags CustomFlags
	genericclioptions.IOStreams
}

// CmdBuilder used to build a cobra Command
type CmdBuilder struct {
	cmd *Command
}

func NewCmdBuilder() *CmdBuilder {
	return &CmdBuilder{&Command{}}
}

func (b *CmdBuilder) Use(use string) *CmdBuilder {
	b.cmd.Use = use
	return b
}

func (b *CmdBuilder) Short(short string) *CmdBuilder {
	b.cmd.Short = short
	return b
}

func (b *CmdBuilder) Example(example string) *CmdBuilder {
	b.cmd.Example = example
	return b
}

func (b *CmdBuilder) GVR(gvr schema.GroupVersionResource) *CmdBuilder {
	b.cmd.GVR = gvr
	return b
}

func (b *CmdBuilder) Factory(f cmdutil.Factory) *CmdBuilder {
	b.cmd.Factory = f
	return b
}

func (b *CmdBuilder) IOStreams(streams genericclioptions.IOStreams) *CmdBuilder {
	b.cmd.IOStreams = streams
	return b
}

func (b *CmdBuilder) CustomComplete(fn CustomCompleteFn) *CmdBuilder {
	b.cmd.CustomComplete = fn
	return b
}

func (b *CmdBuilder) CustomFlags(fn CustomFlags) *CmdBuilder {
	b.cmd.CustomFlags = fn
	return b
}

func (b *CmdBuilder) Build(buildFn BuildFn) *cobra.Command {
	return buildFn(b.cmd)
}
