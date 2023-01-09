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

package troubleshoot

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootclientsetscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
	"github.com/tj/go-spin"
	"golang.org/x/sync/errgroup"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/util"
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
)

// preflightOptions declares the arguments accepted by the preflight command
type preflightOptions struct {
	genericclioptions.IOStreams
	factory cmdutil.Factory
	*preflight.PreflightFlags
	yamlCheckFiles []string
	namespace      string
}

func NewPreflightCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	p := &preflightOptions{
		factory:        f,
		IOStreams:      streams,
		PreflightFlags: preflight.NewPreflightFlags(),
	}
	cmd := &cobra.Command{
		Use:     "preflight",
		Short:   "Run and retrieve preflight checks for kubeblocks",
		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(p.complete(args))
			util.CheckErr(p.validate())
			util.CheckErr(p.run())
		},
	}
	cmd.Flags().BoolVar(p.Interactive, flagInteractive, *p.Interactive, "interactive preflights")
	cmd.Flags().StringVar(p.Format, flagFormat, *p.Format, "output format, one of human, json, yaml. only used when interactive is set to false")
	cmd.Flags().StringVar(p.CollectorImage, flagCollectorImage, *p.CollectorImage, "the full name of the collector image to use")
	cmd.Flags().StringVar(p.CollectorPullPolicy, flagCollectorPullPolicy, *p.CollectorPullPolicy, "the pull policy of the collector image")
	cmd.Flags().BoolVar(p.CollectWithoutPermissions, flagCollectWithoutPermissions, *p.CollectWithoutPermissions, "always run preflight checks even if some require permissions that preflight does not have")
	cmd.Flags().StringVar(p.Selector, flagSelector, *p.Selector, "selector (label query) to filter remote collection nodes on.")
	cmd.Flags().StringVar(p.SinceTime, flagSinceTime, *p.SinceTime, "force pod logs collectors to return logs after a specific date (RFC3339)")
	cmd.Flags().StringVar(p.Since, flagSince, *p.Since, "force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.")
	cmd.Flags().StringVarP(p.Output, flagOutput, *p.Output, "", "specify the output file path for the preflight checks")
	cmd.Flags().BoolVar(p.Debug, flagDebug, *p.Debug, "enable debug logging")
	cmd.Flags().StringVarP(&p.namespace, flagNamespace, "n", "", "If present, the namespace scope for this CLI request")
	return cmd
}

func (p *preflightOptions) complete(args []string) error {
	p.yamlCheckFiles = args
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		os.Exit(0)
	}()
	return nil
}

func (p *preflightOptions) validate() error {
	if len(p.yamlCheckFiles) < 1 {
		return fmt.Errorf("must specify at least one checks yaml")
	}
	return nil
}

func (p *preflightOptions) run() error {
	if *p.Interactive {
		fmt.Print(cursor.Hide())
		defer fmt.Print(cursor.Show())
	}
	var (
		preflightSpec     *troubleshootv1beta2.Preflight
		hostPreflightSpec *troubleshootv1beta2.HostPreflight
		collectResults    []preflight.CollectResult
		analyzeResults    []*analyzer.AnalyzeResult
		preflightName     string
		err               error
	)
	// register the scheme of troubleshoot API and decode function
	if err = troubleshootclientsetscheme.AddToScheme(scheme.Scheme); err != nil {
		return err
	}
	// load preflight content
	if preflightSpec, hostPreflightSpec, preflightName, err = p.loadPreflightSpec(); err != nil {
		return err
	}
	// set progress chain
	progressCh := make(chan interface{})
	defer close(progressCh)
	ctx, stopProgressCollection := context.WithCancel(context.Background())
	// make sure we shut down progress collection goroutines if an error occurs
	defer stopProgressCollection()
	progressCollection, ctx := errgroup.WithContext(ctx)
	if *p.Interactive {
		progressCollection.Go(collectInteractiveProgress(ctx, progressCh))
	} else {
		progressCollection.Go(collectNonInteractiveProgress(ctx, progressCh))
	}
	// collect data
	if collectResults, err = p.collectData(preflightSpec, hostPreflightSpec, progressCh); err != nil {
		return err
	}
	// analyze data
	for _, res := range collectResults {
		analyzeResults = append(analyzeResults, res.Analyze()...)
	}
	// wait for collection end
	stopProgressCollection()
	_ = progressCollection.Wait()
	// display analyzeResults
	if *p.Interactive {
		return showInteractiveResults(preflightName, analyzeResults, *p.Output)
	} else {
		return showStdoutResults(preflightName, analyzeResults, *p.Format)
	}
}

// loadPreflightSpec loads content of preflightSpec and hostPreflightSpec against yamlFiles from args
func (p *preflightOptions) loadPreflightSpec() (*troubleshootv1beta2.Preflight, *troubleshootv1beta2.HostPreflight, string, error) {
	var (
		preflightSpec     *troubleshootv1beta2.Preflight
		hostPreflightSpec *troubleshootv1beta2.HostPreflight
		preflightContent  []byte
		preflightName     string
		err               error
	)
	for _, fileName := range p.yamlCheckFiles {
		// support to load yaml from stdin, local file and URI
		if preflightContent, err = cluster.MultipleSourceComponents(fileName, os.Stdin); err != nil {
			return preflightSpec, hostPreflightSpec, preflightName, err
		}
		obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(preflightContent, nil, nil)
		if err != nil {
			return preflightSpec, hostPreflightSpec, preflightName, errors.Wrapf(err, "failed to parse %s", fileName)
		}
		if spec, ok := obj.(*troubleshootv1beta2.Preflight); ok {
			preflightSpec = ConcatPreflightSpec(preflightSpec, spec)
			preflightName = preflightSpec.Name
		} else if spec, ok := obj.(*troubleshootv1beta2.HostPreflight); ok {
			hostPreflightSpec = ConcatHostPreflightSpec(hostPreflightSpec, spec)
			preflightName = preflightSpec.Name
		}
	}
	return preflightSpec, hostPreflightSpec, preflightName, nil
}

func (p *preflightOptions) collectData(preflightSpec *troubleshootv1beta2.Preflight, hostPreflightSpec *troubleshootv1beta2.HostPreflight, progressCh chan interface{}) ([]preflight.CollectResult, error) {
	var collectResults []preflight.CollectResult
	if preflightSpec != nil {
		res, err := collectDataInCluster(preflightSpec, progressCh, *p)
		if err != nil {
			return collectResults, errors.Wrap(err, "failed to collect data in cluster")
		}
		collectResults = append(collectResults, *res)
	}
	if hostPreflightSpec != nil {
		if len(hostPreflightSpec.Spec.Collectors) > 0 {
			res, err := collectHostData(hostPreflightSpec, progressCh)
			if err != nil {
				return collectResults, errors.Wrap(err, "failed to collect data from host")
			}
			collectResults = append(collectResults, *res)
		}
		if len(hostPreflightSpec.Spec.RemoteCollectors) > 0 {
			res, err := collectRemoteData(hostPreflightSpec, progressCh, *p)
			if err != nil {
				return collectResults, errors.Wrap(err, "failed to collect data remotely")
			}
			collectResults = append(collectResults, *res)
		}
	}
	return collectResults, nil
}

func collectInteractiveProgress(ctx context.Context, progressCh <-chan interface{}) func() error {
	return func() error {
		spinner := spin.New()
		lastMsg := ""
		errorTxt := color.New(color.FgHiRed)
		infoTxt := color.New(color.FgCyan)

		for {
			select {
			case msg := <-progressCh:
				switch msg := msg.(type) {
				case error:
					errorTxt.Printf("%s\r * %v\n", cursor.ClearEntireLine(), msg)
				case string:
					if lastMsg == msg {
						break
					}
					lastMsg = msg
					infoTxt.Printf("%s\r * %s\n", cursor.ClearEntireLine(), msg)
				}
			case <-time.After(time.Millisecond * 100):
				fmt.Printf("\r  %s %s ", color.CyanString("Running Preflight Checks"), spinner.Next())
			case <-ctx.Done():
				fmt.Printf("\r%s\r", cursor.ClearEntireLine())
				return nil
			}
		}
	}
}

func collectNonInteractiveProgress(ctx context.Context, progressCh <-chan interface{}) func() error {
	return func() error {
		for {
			select {
			case msg := <-progressCh:
				switch msg := msg.(type) {
				case error:
					fmt.Fprintf(os.Stderr, "error - %v\n", msg)
				case string:
					fmt.Fprintf(os.Stderr, "%s\n", msg)
				case preflight.CollectProgress:
					fmt.Fprintf(os.Stderr, "%s\n", msg.String())
				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}
