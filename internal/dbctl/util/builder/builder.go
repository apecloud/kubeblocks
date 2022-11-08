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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// Command used to build a command
type Command struct {
	Use       string
	Short     string
	Example   string
	GroupKind schema.GroupKind
	Factory   cmdutil.Factory
	genericclioptions.IOStreams
}

// CmdBuilder build a Command
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

func (b *CmdBuilder) GroupKind(gk schema.GroupKind) *CmdBuilder {
	b.cmd.GroupKind = gk
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

func (b *CmdBuilder) Cmd() *Command {
	return b.cmd
}
