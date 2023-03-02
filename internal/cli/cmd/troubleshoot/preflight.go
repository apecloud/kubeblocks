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
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
	"github.com/tj/go-spin"
	"golang.org/x/sync/errgroup"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
)

var (
	preflightExample = templates.Examples(`
		# Run preflight checks against the customized rules of preflight-check.yaml
		kbcli troubleshoot preflight preflight-check.yaml

		# Run preflight checks and display AnalyzeResults with non-interactive mode
		kbcli troubleshoot preflight preflight-check.yaml --interactive=false`)
)

// PreflightOptions declares the arguments accepted by the preflight command
type PreflightOptions struct {
	factory cmdutil.Factory
	genericclioptions.IOStreams
	*preflight.PreflightFlags
	yamlCheckFiles []string
	namespace      string
}

func NewPreflightCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	p := &PreflightOptions{
		factory:        f,
		IOStreams:      streams,
		PreflightFlags: preflight.NewPreflightFlags(),
	}
	cmd := &cobra.Command{
		Use:     "preflight",
		Short:   "Run and retrieve preflight checks for KubeBlocks",
		Example: preflightExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(p.complete(args))
			util.CheckErr(p.validate())
			util.CheckErr(p.run())
		},
	}
	// add flags
	cmd.Flags().BoolVar(p.Interactive, flagInteractive, false, "interactive preflights, default is false")
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

func (p *PreflightOptions) complete(args []string) error {
	p.yamlCheckFiles = args
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		os.Exit(0)
	}()
	return nil
}

func (p *PreflightOptions) validate() error {
	if len(p.yamlCheckFiles) < 1 {
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
	// set progress chain
	progressCh := make(chan interface{})
	defer close(progressCh)
	// make sure we shut down progress collection goroutines if an error occurs
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	progressCollections, ctx := errgroup.WithContext(ctx)
	if *p.Interactive {
		progressCollections.Go(CollectInteractiveProgress(ctx, progressCh))
	} else {
		progressCollections.Go(CollectNonInteractiveProgress(ctx, progressCh))
	}
	// 1. load yaml
	if kbPreflight, kbHostPreflight, preflightName, err = kbpreflight.LoadPreflightSpec(p.yamlCheckFiles); err != nil {
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
		return kbpreflight.ShowStdoutResults(preflightName, analyzeResults, *p.Format)
	}
}

func CollectInteractiveProgress(ctx context.Context, progressCh <-chan interface{}) func() error {
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

func CollectNonInteractiveProgress(ctx context.Context, progressCh <-chan interface{}) func() error {
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
