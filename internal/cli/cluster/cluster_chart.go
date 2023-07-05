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

package cluster

import (
	"compress/gzip"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

const (
	templatesDir = "templates"
	clusterFile  = "cluster.yaml"
)

type SchemaPropName string

// the common schema property name
const (
	VersionSchemaProp SchemaPropName = "version"
)

type ChartInfo struct {
	// Schema is the cluster parent helm chart schema, used to render the command flag
	Schema *spec.Schema

	// SubSchema is the sub chart schema, used to render the command flag
	SubSchema *spec.Schema

	// SubChartName is the name (alias if exists) of the sub chart
	SubChartName string

	// ClusterDef is the cluster definition
	ClusterDef string

	// Chart is the cluster helm chart object
	Chart *chart.Chart
}

type (
	// ClusterType is the type of the cluster
	ClusterType string

	// chartFile is the helm chart file information
	chartFile struct {
		chartFS embed.FS
		name    string
	}
)

var clusterTypeCharts = map[ClusterType]chartFile{}

func registerClusterType(t ClusterType, chartFS embed.FS, name string) {
	if _, ok := clusterTypeCharts[t]; ok {
		panic(fmt.Sprintf("cluster type %s already registered", t))
	}
	clusterTypeCharts[t] = chartFile{chartFS: chartFS, name: name}
}

func BuildChartInfo(t ClusterType) (*ChartInfo, error) {
	var err error

	c := &ChartInfo{}
	// load helm chart from embed tgz file
	if c.Chart, err = loadHelmChart(t); err != nil {
		return nil, err
	}

	if err = c.buildClusterSchema(); err != nil {
		return nil, err
	}

	if err = c.buildClusterDef(); err != nil {
		return nil, err
	}
	return c, nil
}

// GetManifests gets the cluster manifests
func GetManifests(c *chart.Chart, namespace, name string, values map[string]interface{}) (map[string]string, error) {
	// get the helm chart manifest
	actionCfg, err := helm.NewActionConfig(helm.NewFakeConfig(namespace))
	if err != nil {
		return nil, err
	}
	actionCfg.Log = func(format string, v ...interface{}) {
		fmt.Printf(format, v...)
	}

	client := action.NewInstall(actionCfg)
	client.DryRun = true
	client.Replace = true
	client.ClientOnly = true
	client.ReleaseName = name
	client.Namespace = namespace

	rel, err := client.Run(c, values)
	if err != nil {
		return nil, err
	}
	return releaseutil.SplitManifests(rel.Manifest), nil
}

// buildClusterSchema build the schema for the given cluster chart.
func (c *ChartInfo) buildClusterSchema() error {
	var err error
	cht := c.Chart
	buildSchema := func(bs []byte) (*spec.Schema, error) {
		schema := &spec.Schema{}
		if err = json.Unmarshal(bs, schema); err != nil {
			return nil, errors.Wrapf(err, "failed to build schema for engine %s", cht.Name())
		}
		return schema, nil
	}

	// build cluster schema
	if c.Schema, err = buildSchema(cht.Schema); err != nil {
		return err
	}

	if len(cht.Dependencies()) == 0 {
		return nil
	}

	// build extra schema in sub chart, now, we only support one sub chart
	subChart := cht.Dependencies()[0]
	c.SubChartName = subChart.Name()
	if c.SubSchema, err = buildSchema(subChart.Schema); err != nil {
		return err
	}

	// if sub chart has alias, we should use alias instead of chart name
	for _, dep := range cht.Metadata.Dependencies {
		if dep.Name != c.SubChartName {
			continue
		}

		if dep.Alias != "" {
			c.SubChartName = dep.Alias
		}
	}

	return nil
}

func (c *ChartInfo) buildClusterDef() error {
	cht := c.Chart
	clusterFilePath := filepath.Join(templatesDir, clusterFile)
	for _, tpl := range cht.Templates {
		if tpl.Name != clusterFilePath {
			continue
		}

		// get cluster definition from cluster.yaml
		pattern := "  clusterDefinitionRef: "
		str := string(tpl.Data)
		start := strings.Index(str, pattern)
		if start != -1 {
			end := strings.IndexAny(str[start+len(pattern):], " \n")
			if end != -1 {
				c.ClusterDef = strings.TrimSpace(str[start+len(pattern) : start+len(pattern)+end])
				return nil
			}
		}
	}
	return fmt.Errorf("failed to find the cluster definition of %s", cht.Name())
}

// ValidateValues validates the given values against the schema.
func ValidateValues(c *ChartInfo, values map[string]interface{}) error {
	validateFn := func(s *spec.Schema, values map[string]interface{}) error {
		if s == nil {
			return nil
		}
		v := validate.NewSchemaValidator(s, nil, "", strfmt.Default)
		err := v.Validate(values).AsError()
		if err != nil {
			// the default error message is like "cpu in body should be a multiple of 0.5"
			// the "in body" is not necessary, so we remove it
			errMsg := strings.ReplaceAll(err.Error(), " in body", "")
			return errors.New(errMsg)
		}
		return nil
	}

	if err := validateFn(c.Schema, values); err != nil {
		return err
	}
	return validateFn(c.SubSchema, values)
}

func loadHelmChart(t ClusterType) (*chart.Chart, error) {
	cf, ok := clusterTypeCharts[t]
	if !ok {
		return nil, fmt.Errorf("failed to find the helm chart of %s", t)
	}

	file, err := cf.chartFS.Open(fmt.Sprintf("charts/%s", cf.name))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	c, err := loader.LoadArchive(file)
	if err != nil {
		if err == gzip.ErrHeader {
			return nil, fmt.Errorf("file '%s' does not appear to be a valid chart file (details: %s)", err)
		}
	}

	if c == nil {
		return nil, fmt.Errorf("failed to load cluster helm chart %s", t.String())
	}
	return c, err
}

func SupportedTypes() []ClusterType {
	types := maps.Keys(clusterTypeCharts)
	slices.SortFunc(types, func(i, j ClusterType) bool {
		return i < j
	})
	return types
}

func (t ClusterType) String() string {
	return string(t)
}

func sortClusterTypes(types []ClusterType) {
	sort.Slice(types, func(i, j int) bool {
		return types[i] < types[j]
	})
}

func (s SchemaPropName) String() string {
	return string(s)
}
