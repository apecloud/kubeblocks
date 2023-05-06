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

package util

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

func EnableLogToFile(fs *pflag.FlagSet) error {
	logFile, err := getCliLogFile()
	if err != nil {
		return err
	}

	setFlag := func(kv map[string]string) {
		for k, v := range kv {
			_ = fs.Set(k, v)
		}
	}

	if klog.V(1).Enabled() {
		// if log is enabled, write log to standard output and log file
		setFlag(map[string]string{
			"alsologtostderr": "true",
			"logtostderr":     "false",
			"log-file":        logFile,
		})
	} else {
		// if log is not enabled, enable it and write log to file
		setFlag(map[string]string{
			"v":               "1",
			"logtostderr":     "false",
			"alsologtostderr": "false",
			"log-file":        logFile,
		})
	}
	return nil
}

func getCliLogFile() (string, error) {
	homeDir, err := GetCliHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, fmt.Sprintf("%s-%s.log", types.DefaultLogFilePrefix, time.Now().Format("2006-01-02"))), nil
}

// AddKlogFlags adds flags from k8s.io/klog
// marks the flags as hidden to avoid showing them in help
func AddKlogFlags(fs *pflag.FlagSet) {
	local := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(local)
	local.VisitAll(func(f *flag.Flag) {
		f.Name = strings.ReplaceAll(f.Name, "_", "-")
		if fs.Lookup(f.Name) != nil {
			return
		}
		newFlag := pflag.PFlagFromGoFlag(f)
		newFlag.Hidden = true
		fs.AddFlag(newFlag)
	})
}
