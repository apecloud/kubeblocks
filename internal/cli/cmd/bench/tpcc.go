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
	"github.com/pingcap/go-tpc/tpcc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var tpccConfig tpcc.Config

func NewTpccCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tpcc",
		Short: "Run a TPCC benchmark.",
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
		Short: "Prepare data for TPCC.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTpcc("prepare")
		},
	}

	cmd.Flags().BoolVar(&tpccConfig.NoCheck, "no-check", false, "TPCC prepare check, default false")
	cmd.Flags().StringVar(&tpccConfig.OutputType, "output-type", "", "Output file type."+
		" If not set, database generates the data itself. Current only support csv")
	cmd.Flags().StringVar(&tpccConfig.OutputDir, "output-dir", "", "Output directory for generating file if specified")
	cmd.Flags().StringVar(&tpccConfig.SpecifiedTables, "tables", "", "Specified tables for "+
		"generating file, separated by ','. Valid only if output is set. If not set, generate all tables by default")
	cmd.Flags().IntVar(&tpccConfig.PrepareRetryCount, "retry-count", 50, "Retry count when errors occur")
	cmd.Flags().DurationVar(&tpccConfig.PrepareRetryInterval, "retry-interval", 5*time.Second, "The interval for each retry")
	return cmd
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run workload.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTpcc("run")
		},
	}

	cmd.Flags().BoolVar(&tpccConfig.Wait, "wait", false, "including keying & thinking time described on TPC-C Standard Specification")
	cmd.Flags().DurationVar(&tpccConfig.MaxMeasureLatency, "max-measure-latency", measurement.DefaultMaxLatency, "max measure latency in milliseconds")
	cmd.Flags().IntSliceVar(&tpccConfig.Weight, "weight", []int{45, 43, 4, 4, 4}, "Weight for NewOrder, Payment, OrderStatus, Delivery, StockLevel")

	return cmd
}

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup data for TPCC.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeTpcc("cleanup")
		},
	}
	return cmd
}

func executeTpcc(action string) error {
	runtime.GOMAXPROCS(maxProcs)

	var (
		w   workload.Workloader
		err error
	)

	err = openDB()
	defer func() { _ = closeDB() }()
	if err != nil {
		return err
	}

	tpccConfig.DBName = dbName
	tpccConfig.Threads = threads
	tpccConfig.Isolation = isolationLevel
	tpccConfig.Driver = driver

	switch tpccConfig.OutputType {
	case "csv", "CSV":
		if tpccConfig.OutputDir == "" {
			return errors.New("Output Directory cannot be empty when generating files")
		}
		w, err = tpcc.NewCSVWorkloader(globalDB, &tpccConfig)
	default:
		w, err = tpcc.NewWorkloader(globalDB, &tpccConfig)
	}

	if err != nil {
		return fmt.Errorf("failed to init work loader: %v", err)
	}

	timeoutCtx, cancel := context.WithTimeout(globalCtx, totalTime)
	defer cancel()

	executeWorkload(timeoutCtx, w, threads, action)

	fmt.Println("Finished")
	w.OutputStats(true)
	return nil
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
					panic(fmt.Sprintf("a fatal error occurred when preparing data: %v", err))
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
	var err error
	count := totalCount / threads
	ctx = w.InitThread(ctx, index)
	defer w.CleanupThread(ctx, index)

	defer func() {
		if recover() != nil {
			fmt.Fprintln(os.Stdout, "Unexpected error")
		}
	}()

	switch action {
	case "prepare":
		// Do cleanup only if dropData is set and not generate csv data.
		if dropData {
			if err := w.Cleanup(ctx, index); err != nil {
				return err
			}
		}
		err = w.Prepare(ctx, index)
	case "cleanup":
		err = w.Cleanup(ctx, index)
	case "check":
		err = w.Check(ctx, index)
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

	return err
}
