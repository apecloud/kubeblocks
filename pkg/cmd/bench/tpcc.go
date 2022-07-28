/*
Copyright © 2022 The OpenCli Authors

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

package bench

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/pingcap/go-tpc/pkg/measurement"
	"github.com/pingcap/go-tpc/pkg/workload"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"jihulab.com/infracreate/dbaas-system/opencli/pkg/cmd/bench/tpcc"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/utils"
)

var tpccConfig tpcc.Config

func NewTpccCmd(f cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tpcc",
		Short: "Run a TPCC benchmark",
		Run: func(cmd *cobra.Command, args []string) {
			runSimpleBench(f, args)
		},
	}

	cmd.PersistentFlags().IntVar(&tpccConfig.Parts, "parts", 1, "Number to partition warehouses")
	cmd.PersistentFlags().IntVar(&tpccConfig.PartitionType, "partition-type", 1, "Partition type (1 - HASH, 2 - RANGE, 3 - LIST (like HASH), 4 - LIST (like RANGE)")
	cmd.PersistentFlags().IntVar(&tpccConfig.Warehouses, "warehouses", 4, "Number of warehouses")
	cmd.PersistentFlags().BoolVar(&tpccConfig.CheckAll, "check-all", false, "Run all consistency checks")

	// add subcommands
	cmd.AddCommand(newPrepareCmd(), newRunCmd(), newCleanCmd())
	return cmd
}

func newPrepareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare data for TPCC",
		Run: func(cmd *cobra.Command, args []string) {
			executeTpcc("prepare")
		},
	}

	cmd.Flags().BoolVar(&tpccConfig.NoCheck, "no-check", false, "TPCC prepare check, default false")
	cmd.Flags().StringVar(&tpccConfig.OutputType, "output-type", "", "Output file type."+
		" If empty, then load data to db. Current only support csv")
	cmd.Flags().StringVar(&tpccConfig.OutputDir, "output-dir", "", "Output directory for generating file if specified")
	cmd.Flags().StringVar(&tpccConfig.SpecifiedTables, "tables", "", "Specified tables for "+
		"generating file, separated by ','. Valid only if output is set. If this flag is not set, generate all tables by default")
	cmd.Flags().IntVar(&tpccConfig.PrepareRetryCount, "retry-count", 50, "Retry count when errors occur")
	cmd.Flags().DurationVar(&tpccConfig.PrepareRetryInterval, "retry-interval", 5*time.Second, "The interval for each retry")
	return cmd
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run workload",
		Run: func(cmd *cobra.Command, args []string) {
			executeTpcc("run")
		},
	}

	cmd.Flags().BoolVar(&tpccConfig.Wait, "wait", false, "including keying & thinking time described on TPC-C Standard Specification")
	cmd.Flags().DurationVar(&tpccConfig.MaxMeasureLatency, "max-measure-latency", measurement.DefaultMaxLatency, "max measure latency in millisecond")
	cmd.Flags().IntSliceVar(&tpccConfig.Weight, "weight", []int{45, 43, 4, 4, 4}, "Weight for NewOrder, Payment, OrderStatus, Delivery, StockLevel")

	return cmd
}

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup data for TPCC",
		Run: func(cmd *cobra.Command, args []string) {
			executeTpcc("cleanup")
		},
	}
	return cmd
}

func executeTpcc(action string) {
	runtime.GOMAXPROCS(maxProcs)

	openDB()
	defer closeDB()

	tpccConfig.DBName = dbName
	tpccConfig.Threads = threads
	tpccConfig.Isolation = isolationLevel
	var (
		w   workload.Workloader
		err error
	)
	switch tpccConfig.OutputType {
	case "csv", "CSV":
		if tpccConfig.OutputDir == "" {
			fmt.Printf("Output Directory cannot be empty when generating files")
			os.Exit(1)
		}
		w, err = tpcc.NewCSVWorkloader(globalDB, &tpccConfig)
	default:
		w, err = tpcc.NewWorkloader(globalDB, &tpccConfig)
	}

	if err != nil {
		fmt.Printf("Failed to init work loader: %v\n", err)
		os.Exit(1)
	}

	timeoutCtx, cancel := context.WithTimeout(globalCtx, totalTime)
	defer cancel()

	executeWorkload(timeoutCtx, w, threads, action)

	fmt.Println("Finished")
	w.OutputStats(true)
}

func executeWorkload(ctx context.Context, w workload.Workloader, threads int, action string) {
	var wg sync.WaitGroup
	wg.Add(threads)

	outputCtx, outputCancel := context.WithCancel(ctx)
	ch := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(outputInterval)
		defer ticker.Stop()

		for {
			select {
			case <-outputCtx.Done():
				ch <- struct{}{}
				return
			case <-ticker.C:
				w.OutputStats(false)
			}
		}
	}()

	for i := 0; i < threads; i++ {
		go func(index int) {
			defer wg.Done()
			if err := execute(ctx, w, action, threads, index); err != nil {
				if action == "prepare" {
					panic(fmt.Sprintf("a fatal occurred when preparing data: %v", err))
				}
				fmt.Printf("execute %s failed, err %v\n", action, err)
				return
			}
		}(i)
	}

	wg.Wait()
	outputCancel()

	<-ch
}

func execute(ctx context.Context, w workload.Workloader, action string, threads, index int) error {
	count := totalCount / threads

	ctx = w.InitThread(ctx, index)
	defer w.CleanupThread(ctx, index)

	switch action {
	case "prepare":
		// Do cleanup only if dropData is set and not generate csv data.
		if dropData {
			if err := w.Cleanup(ctx, index); err != nil {
				return err
			}
		}
		return w.Prepare(ctx, index)
	case "cleanup":
		return w.Cleanup(ctx, index)
	case "check":
		return w.Check(ctx, index)
	}

	for i := 0; i < count || count <= 0; i++ {
		err := w.Run(ctx, index)

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err != nil {
			if !silence {
				fmt.Printf("[%s] execute %s failed, err %v\n", time.Now().Format("2006-01-02 15:04:05"), action, err)
			}
			if !ignoreError {
				return err
			}
		}
	}

	return nil
}

// runSimpleBench for a bench that specified database cluster name, we always bench this
// cluster and ignore other connection info
func runSimpleBench(f cmdutil.Factory, args []string) {
	if len(args) == 0 {
		utils.Info("You must specify a cluster name")
		return
	}

	clusterName := args[0]
	if clusterName != "mycluster" {
		utils.Infof("Do not find \"%s\" cluster", clusterName)
		return
	}

	utils.Info("Preparing data...")
	executeTpcc("prepare")

	utils.Info("Run tpcc 60s ...")
	totalTime = 60 * time.Second
	executeTpcc("run")

	utils.Info("Clean up data...")
	executeTpcc("cleanup")
}
