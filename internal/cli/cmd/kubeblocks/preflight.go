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
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"

	"github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	kbpreflight "github.com/apecloud/kubeblocks/internal/preflight"
	kbinteractive "github.com/apecloud/kubeblocks/internal/preflight/interactive"
)

const (
	flagInteractive               = "interactive"
	flagFormat                    = "format"
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

const (
	EKSHostPreflight = "data/eks_hostpreflight.yaml"
	EKSPreflight     = "data/eks_preflight.yaml"
	GKEHostPreflight = "data/gke_hostpreflight.yaml"
	GKEPreflight     = "data/gke_preflight.yaml"
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
			util.CheckErr(p.validate())
			util.CheckErr(p.run())
		},
	}
	// add flags
	cmd.Flags().BoolVar(p.Interactive, flagInteractive, false, "interactive preflights, default value is false")
	cmd.Flags().StringVar(p.Format, flagFormat, "yaml", "output format, one of human, json, yaml. only used when interactive is set to false, default format is yaml")
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
	return cmd
}

func LoadVendorCheckYaml(vendorName util.K8sProvider) ([][]byte, error) {
	var yamlDataList [][]byte
	switch vendorName {
	case util.EKSProvider:
		if data, err := defaultVendorYamlData.ReadFile(EKSHostPreflight); err == nil {
			yamlDataList = append(yamlDataList, data)
		}
		if data, err := defaultVendorYamlData.ReadFile(EKSPreflight); err == nil {
			yamlDataList = append(yamlDataList, data)
		}
	case util.GKEProvider:
		if data, err := defaultVendorYamlData.ReadFile(GKEHostPreflight); err == nil {
			yamlDataList = append(yamlDataList, data)
		}
		if data, err := defaultVendorYamlData.ReadFile(GKEPreflight); err == nil {
			yamlDataList = append(yamlDataList, data)
		}
	case util.UnknownProvider:
		fallthrough
	default:
		fmt.Println("unsupported k8s provider, and the validation of provider will coming soon")
		return yamlDataList, errors.New("no supported provider")
	}
	return yamlDataList, nil
}

func (p *PreflightOptions) complete(factory cmdutil.Factory, args []string) error {
	// default no args, and run default validating vendor
	if len(args) == 0 {
		clientSet, err := factory.KubernetesClientSet()
		if err != nil {
			return errors.New("init k8s client failed, and please check kubeconfig")
		}
		versionInfo, err := util.GetVersionInfo(clientSet)
		if err != nil {
			return errors.New("get k8s version of server failed, and please check your k8s accessibility")
		}
		vendorName, err := util.GetK8sProvider(versionInfo[util.KubernetesApp], clientSet)
		if err != nil {
			return errors.New("get k8s cloud provider failed, and please check your k8s accessibility")
		}
		p.checkYamlData, err = LoadVendorCheckYaml(vendorName)
		if err != nil {
			return err
		}
		color.New(color.FgCyan).Printf("current provider %s. collecting and analyzing data will take 10-20 seconds...  \n", vendorName)
	} else {
		p.checkFileList = args
		color.New(color.FgCyan).Println("collecting and analyzing data will take 10-20 seconds...")
	}
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

func (p *PreflightOptions) validate() error {
	if len(p.checkFileList) < 1 && len(p.checkYamlData) < 1 {
		return fmt.Errorf("must specify at least one checks yaml")
	}
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
	if *p.Interactive {
		fmt.Print(cursor.Hide())
		defer fmt.Print(cursor.Show())
	}
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
		return err
	}
	// 2. collect data
	collectResults, err = kbpreflight.CollectPreflight(ctx, kbPreflight, kbHostPreflight, progressCh)
	if err != nil {
		return err
	}
	// 3. analyze data
	for _, res := range collectResults {
		analyzeResults = append(analyzeResults, res.Analyze()...)
	}
	cancelFunc()
	if err := progressCollections.Wait(); err != nil {
		return err
	}
	// 4. display analyzed data
	if len(analyzeResults) == 0 {
		return errors.New("no data has been collected")
	}
	if *p.Interactive {
		return kbinteractive.ShowInteractiveResults(preflightName, analyzeResults, *p.Output)
	} else {
		return kbpreflight.ShowTextResults(preflightName, analyzeResults, *p.Format, p.verbose)
	}
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
