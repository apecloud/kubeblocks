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

package bench

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	dbName         string
	host           string
	port           int
	user           string
	password       string
	threads        int
	driver         string
	totalTime      time.Duration
	totalCount     int
	dropData       bool
	ignoreError    bool
	outputInterval time.Duration
	isolationLevel int
	silence        bool
	maxProcs       int

	globalDB  *sql.DB
	globalCtx context.Context
)

// NewBenchCmd creates the bench command
func NewBenchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run a benchmark.",
	}

	cmd.PersistentFlags().IntVar(&maxProcs, "max-procs", 0, "runtime.GOMAXPROCS")
	cmd.PersistentFlags().StringVarP(&dbName, "db", "D", "kb_test", "Database name")
	cmd.PersistentFlags().StringVarP(&host, "host", "H", "127.0.0.1", "Database host")
	cmd.PersistentFlags().StringVarP(&user, "user", "U", "root", "Database user")
	cmd.PersistentFlags().StringVarP(&password, "password", "p", "sakila", "Database password")
	cmd.PersistentFlags().IntVarP(&port, "port", "P", 3306, "Database port")
	cmd.PersistentFlags().IntVarP(&threads, "threads", "T", 1, "Thread concurrency")
	cmd.PersistentFlags().StringVarP(&driver, "driver", "d", mysqlDriver, "Database driver: mysql")
	cmd.PersistentFlags().DurationVar(&totalTime, "time", 1<<63-1, "Total execution time")
	cmd.PersistentFlags().IntVar(&totalCount, "count", 0, "Total execution count, 0 means infinite")
	cmd.PersistentFlags().BoolVar(&dropData, "dropdata", false, "Cleanup data before prepare")
	cmd.PersistentFlags().BoolVar(&ignoreError, "ignore-error", false, "Ignore error when running workload")
	cmd.PersistentFlags().BoolVar(&silence, "silence", false, "Don't print error when running workload")
	cmd.PersistentFlags().DurationVar(&outputInterval, "interval", 5*time.Second, "Output interval time")
	cobra.EnablePrefixMatching = true

	// add subcommands
	cmd.AddCommand(
		NewTpccCmd(),
	)

	var cancel context.CancelFunc
	globalCtx, cancel = context.WithCancel(context.Background())

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	closeDone := make(chan struct{}, 1)
	go func() {
		sig := <-sc
		fmt.Printf("\nGot signal [%v] to exit.\n", sig)
		cancel()

		select {
		case <-sc:
			// send signal again, return directly
			fmt.Printf("\nGot signal [%v] again to exit.\n", sig)
			os.Exit(1)
		case <-time.After(10 * time.Second):
			fmt.Print("\nWait 10s for closed, force exit\n")
			os.Exit(1)
		case <-closeDone:
			return
		}
	}()

	return cmd
}
