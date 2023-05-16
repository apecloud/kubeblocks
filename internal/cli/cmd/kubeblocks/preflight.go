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
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"helm.sh/helm/v3/pkg/cli/values"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	kbpreflight "github.com/apecloud/kubeblocks/internal/preflight"
)

const (
	flagCollectorImage            = "collector-image"
	flagCollectorPullPolicy       = "collector-pullpolicy"
	flagCollectWithoutPermissions = "collect-without-permissions"
	flagSelector                  = "selector"
	flagSinceTime                 = "since-time"
	flagSince                     = "since"
	flagOutput                    = "output"
	flagDebug                     = "debug"
	flagNamespace                 = "namespace"
	flagVerbose                   = "verbose"
	flagForce                     = "force"
	flagFormat                    = "format"

	PreflightPattern     = "data/%s_preflight.yaml"
	HostPreflightPattern = "data/%s_hostpreflight.yaml"
	PreflightMessage     = "Run a preflight to check that the environment meets the requirement for KubeBlocks. It takes 10~20 seconds."
)

var (
	//go:embed data/*
	defaultVendorYamlData embed.FS
	preflightExample      = templates.Examples(`
		# Run preflight provider checks against the default rules automatically
		kbcli kubeblocks preflight

		# Run preflight provider checks and output more verbose info
		kbcli kubeblocks preflight --verbose

		# Run preflight checks against the customized rules of preflight-check.yaml
		kbcli kubeblocks preflight preflight-check.yaml

		# Run preflight checks and display AnalyzeResults with interactive mode
		kbcli kubeblocks preflight preflight-check.yaml --interactive=true`)
)

// PreflightOptions declares the arguments accepted by the preflight command
type PreflightOptions struct {
	factory cmdutil.Factory
	genericclioptions.IOStreams
	*preflight.PreflightFlags
	checkFileList []string
	checkYamlData [][]byte
	namespace     string
	verbose       bool
	force         bool
	ValueOpts     values.Options
}

func NewPreflightCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	p := &PreflightOptions{
		factory:        f,
		IOStreams:      streams,
		PreflightFlags: preflight.NewPreflightFlags(),
	}
	cmd := &cobra.Command{
		Use:     "preflight",
		Short:   "Run and retrieve preflight checks for KubeBlocks.",
		Example: preflightExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(p.complete(f, args))
			util.CheckErr(p.run())
		},
	}
	// add flags
	cmd.Flags().StringVar(p.Format, flagFormat, "yaml", "output format, one of json, yaml. only used when interactive is set to false, default format is yaml")
	cmd.Flags().StringVar(p.CollectorImage, flagCollectorImage, *p.CollectorImage, "the full name of the collector image to use")
	cmd.Flags().StringVar(p.CollectorPullPolicy, flagCollectorPullPolicy, *p.CollectorPullPolicy, "the pull policy of the collector image")
	cmd.Flags().BoolVar(p.CollectWithoutPermissions, flagCollectWithoutPermissions, *p.CollectWithoutPermissions, "always run preflight checks even if some require permissions that preflight does not have")
	cmd.Flags().StringVar(p.Selector, flagSelector, *p.Selector, "selector (label query) to filter remote collection nodes on.")
	cmd.Flags().StringVar(p.SinceTime, flagSinceTime, *p.SinceTime, "force pod logs collectors to return logs after a specific date (RFC3339)")
	cmd.Flags().StringVar(p.Since, flagSince, *p.Since, "force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.")
	cmd.Flags().StringVarP(p.Output, flagOutput, *p.Output, "", "specify the output file path for the preflight checks")
	cmd.Flags().BoolVar(p.Debug, flagDebug, *p.Debug, "enable debug logging")
	cmd.Flags().StringVarP(&p.namespace, flagNamespace, "n", "", "If present, the namespace scope for this CLI request")
	cmd.Flags().BoolVar(&p.verbose, flagVerbose, p.verbose, "print more verbose logs, default value is false")
	helm.AddValueOptionsFlags(cmd.Flags(), &p.ValueOpts)
	return cmd
}

func LoadVendorCheckYaml(vendorName util.K8sProvider) ([][]byte, error) {
	var yamlDataList [][]byte
	if data, err := defaultVendorYamlData.ReadFile(newPreflightPath(vendorName)); err == nil {
		yamlDataList = append(yamlDataList, data)
	}
	if data, err := defaultVendorYamlData.ReadFile(newHostPreflightPath(vendorName)); err == nil {
		yamlDataList = append(yamlDataList, data)
	}
	if len(yamlDataList) == 0 {
		return yamlDataList, errors.New("unsupported k8s provider, and the validation of provider will coming soon")
	}
	return yamlDataList, nil
}

func (p *PreflightOptions) Preflight(f cmdutil.Factory, args []string, opts values.Options) error {
	// if force flag set, skip preflight
	if p.force {
		return nil
	}
	p.ValueOpts = opts
	*p.Format = "yaml"

	var err error
	if err = p.complete(f, args); err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeSkipPreflight) {
			return nil
		}
		return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, err.Error())
	}
	if err = p.run(); err != nil {
		return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, err.Error())
	}
	return nil
}

func (p *PreflightOptions) complete(f cmdutil.Factory, args []string) error {
	// default no args, and run default validating vendor
	if len(args) == 0 {
		clientSet, err := f.KubernetesClientSet()
		if err != nil {
			return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, "init k8s client failed, and please check kubeconfig")
		}
		versionInfo, err := util.GetVersionInfo(clientSet)
		if err != nil {
			return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, "get k8s version of server failed, and please check your k8s accessibility")
		}
		vendorName, err := util.GetK8sProvider(versionInfo.Kubernetes, clientSet)
		if err != nil {
			return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, "get k8s cloud provider failed, and please check your k8s accessibility")
		}
		p.checkYamlData, err = LoadVendorCheckYaml(vendorName)
		if err != nil {
			return intctrlutil.NewError(intctrlutil.ErrorTypeSkipPreflight, err.Error())
		}
		color.New(color.FgCyan).Println(PreflightMessage)
	} else {
		p.checkFileList = args
		color.New(color.FgCyan).Println(PreflightMessage)
	}
	if len(p.checkFileList) < 1 && len(p.checkYamlData) < 1 {
		return intctrlutil.NewError(intctrlutil.ErrorTypeSkipPreflight, "must specify at least one checks yaml")
	}

	p.factory = f
	// conceal warning logs
	rest.SetDefaultWarningHandler(rest.NoWarnings{})
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		os.Exit(0)
	}()
	return nil
}

func (p *PreflightOptions) run() error {
	var (
		kbPreflight     *preflightv1beta2.Preflight
		kbHostPreflight *preflightv1beta2.HostPreflight
		collectResults  []preflight.CollectResult
		analyzeResults  []*analyze.AnalyzeResult
		preflightName   string
		err             error
	)
	// set progress chan
	progressCh := make(chan interface{})
	defer close(progressCh)
	// make sure we shut down progress collection goroutines if an error occurs
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	progressCollections, ctx := errgroup.WithContext(ctx)
	progressCollections.Go(CollectProgress(ctx, progressCh, p.verbose))
	// 1. load yaml
	if kbPreflight, kbHostPreflight, preflightName, err = kbpreflight.LoadPreflightSpec(p.checkFileList, p.checkYamlData); err != nil {
		return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, err.Error())
	}
	// 2. collect data
	collectResults, err = kbpreflight.CollectPreflight(p.factory, &p.ValueOpts, ctx, kbPreflight, kbHostPreflight, progressCh)
	if err != nil {
		return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, err.Error())
	}
	// 3. analyze data
	for _, res := range collectResults {
		analyzeResults = append(analyzeResults, res.Analyze()...)
	}
	cancelFunc()
	if err := progressCollections.Wait(); err != nil {
		return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, err.Error())
	}
	// 4. display analyzed data
	if len(analyzeResults) == 0 {
		fmt.Fprintln(p.Out, "no data has been collected")
		return nil
	}
	if err = kbpreflight.ShowTextResults(preflightName, analyzeResults, *p.Format, p.verbose); err != nil {
		return intctrlutil.NewError(intctrlutil.ErrorTypePreflightCommon, err.Error())
	}
	return nil
}

func CollectProgress(ctx context.Context, progressCh <-chan interface{}, verbose bool) func() error {
	return func() error {
		for {
			select {
			case msg := <-progressCh:
				if verbose {
					switch msg := msg.(type) {
					case error:
						fmt.Fprintf(os.Stderr, "error - %v\n", msg)
					case string:
						fmt.Fprintf(os.Stderr, "%s\n", msg)
					case preflight.CollectProgress:
						fmt.Fprintf(os.Stderr, "%s\n", msg.String())
					}
				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func newPreflightPath(vendorName util.K8sProvider) string {
	return fmt.Sprintf(PreflightPattern, strings.ToLower(string(vendorName)))
}

func newHostPreflightPath(vendorName util.K8sProvider) string {
	return fmt.Sprintf(HostPreflightPattern, strings.ToLower(string(vendorName)))
}
