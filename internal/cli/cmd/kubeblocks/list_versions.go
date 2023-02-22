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

package kubeblocks

import (
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var (
	listVersionsExample = templates.Examples(`
	# list KubeBlocks release version
	kbcli kubeblocks list-versions
	
	# list KubeBlocks versions that including development versions, such as alpha, beta and release candidate
	kbcli kubeblocks list-versions --devel`)
)

type listVersionsOption struct {
	genericclioptions.IOStreams
	HelmCfg   *action.Configuration
	Namespace string
	version   string
	devel     bool
}

func newListVersionsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := listVersionsOption{IOStreams: streams}
	cmd := &cobra.Command{
		Use:     "list-versions",
		Short:   "List all KubeBlocks versions",
		Aliases: []string{"ls-versions"},
		Args:    cobra.NoArgs,
		Example: listVersionsExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.listVersions())
		},
	}

	cmd.Flags().BoolVar(&o.devel, "devel", false, "use development versions (alpha, beta, and release candidate releases), too. Equivalent to version '>0.0.0-0'.")
	return cmd
}

func (o *listVersionsOption) complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error
	if o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	config, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}

	ctx, err := cmd.Flags().GetString("context")
	if err != nil {
		return err
	}

	if o.HelmCfg, err = helm.NewActionConfig(o.Namespace, config, helm.WithContext(ctx)); err != nil {
		return err
	}
	return nil
}

func (o *listVersionsOption) listVersions() error {
	// add repo, if exists, will update it
	if err := helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}

	// get chart versions
	versions, err := helm.GetChartVersions(types.KubeBlocksChartName)
	if err != nil {
		return err
	}

	// sort version descending
	o.setupSearchedVersion()
	sort.Sort(sort.Reverse(semver.Collection(versions)))
	versions, err = o.applyConstraint(versions)
	if err != nil {
		return err
	}

	// print result
	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("VERSION", "RELEASE-NOTE")
	for _, v := range versions {
		tbl.AddRow(v.String(), fmt.Sprintf("https://github.com/apecloud/kubeblocks/releases/tag/v%s", v))
	}
	tbl.Print()
	return nil
}

func (o *listVersionsOption) setupSearchedVersion() {
	if o.devel {
		o.version = ">0.0.0-0"
	} else {
		o.version = ">0.0.0"
	}
}

func (o *listVersionsOption) applyConstraint(versions []*semver.Version) ([]*semver.Version, error) {
	constraint, err := semver.NewConstraint(o.version)
	if err != nil {
		return nil, errors.Wrap(err, "an invalid version/constraint format")
	}

	var res []*semver.Version
	found := map[string]bool{}
	for _, version := range versions {
		if found[version.String()] {
			continue
		}
		if constraint.Check(version) {
			res = append(res, version)
			found[version.String()] = true
		}
	}
	return res, nil
}
