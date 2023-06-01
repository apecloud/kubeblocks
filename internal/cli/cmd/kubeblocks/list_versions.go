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

package kubeblocks

import (
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	defaultLimit = 10
)

var (
	listVersionsExample = templates.Examples(`
	# list KubeBlocks release versions
	kbcli kubeblocks list-versions
	
	# list KubeBlocks versions including development versions, such as alpha, beta and release candidate
	kbcli kubeblocks list-versions --devel`)
)

type listVersionsOption struct {
	genericclioptions.IOStreams
	version string
	devel   bool
	limit   int
}

func newListVersionsCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := listVersionsOption{IOStreams: streams}
	cmd := &cobra.Command{
		Use:     "list-versions",
		Short:   "List KubeBlocks versions.",
		Aliases: []string{"ls-versions"},
		Args:    cobra.NoArgs,
		Example: listVersionsExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.listVersions())
		},
	}

	cmd.Flags().BoolVar(&o.devel, "devel", false, "Use development versions (alpha, beta, and release candidate releases), too. Equivalent to version '>0.0.0-0'.")
	cmd.Flags().IntVar(&o.limit, "limit", defaultLimit, fmt.Sprintf("Maximum rows of versions to return, 0 means no limit (default %d)", defaultLimit))
	return cmd
}

func (o *listVersionsOption) listVersions() error {
	if o.limit < 0 {
		return fmt.Errorf("limit should be greater than or equal to 0")
	}

	// get chart versions
	versions, err := getHelmChartVersions(types.KubeBlocksChartName)
	if err != nil {
		return err
	}

	// sort version descending and select the versions that meet the constraint
	o.setupSearchedVersion()
	sort.Sort(sort.Reverse(semver.Collection(versions)))
	versions, err = o.applyConstraint(versions)
	if err != nil {
		return err
	}

	// print result
	num := 0
	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("VERSION", "RELEASE-NOTES")
	for _, v := range versions {
		tbl.AddRow(v.String(), fmt.Sprintf("https://github.com/apecloud/kubeblocks/releases/tag/v%s", v))
		num += 1
		if num == o.limit {
			break
		}
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
