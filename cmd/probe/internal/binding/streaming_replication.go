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

package binding

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"k8s.io/utils/env"
)

const (
	streamingReplicatorPort = 3502
	xbstreamBinary          = "/usr/bin/xbstream"
	xtrabackupBinary        = "/usr/bin/xtrabackup"
)

var (
	logReplicator = logger.NewLogger("streaming.replication")

	dbHost     = "127.0.0.1"
	dbUser     = env.GetString("KB_SERVICE_USER", "root")
	dbPassword = env.GetString("KB_SERVICE_PASSWORD", "")
	dataDir    = env.GetString("KB_SERVICE_DATA_DIR", "/data/mysql/data")
)

func init() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", streamingReplicatorPort))
	if err != nil {
		panic(fmt.Sprintf("streaming replicator listen error: %s", err.Error()))
	}
	logReplicator.Info(fmt.Sprintf("streaming replicator is listening at %s...", listener.Addr().String()))
	go streamingReplicator(listener)
}

func (ops *BaseOperations) StreamingReplicationOps(ctx context.Context, req *bindings.InvokeRequest, rsp *bindings.InvokeResponse) (OpsResult, error) {
	keys := []string{"SRC_POD_IP", "DATA_VOLUME", "DATA_DIR", "LOG_BIN"}
	for _, key := range keys {
		if _, ok := req.Metadata[key]; !ok {
			return nil, fmt.Errorf("%s should be provided", key)
		}
	}
	if err := streamingRestore(req); err != nil {
		return nil, err
	}
	return nil, nil
}

func streamingReplicator(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logReplicator.Info("streaming replicator accept new connection error: %s", err.Error())
			continue
		}
		logReplicator.Info(fmt.Sprintf("accept new connection, remote: %s", conn.RemoteAddr().String()))
		if err := streamingBackup(dbHost, dbUser, dbPassword, dataDir, conn); err != nil {
			logReplicator.Info(fmt.Sprintf("streaming replication error: %s, remote: %s", err.Error(), conn.RemoteAddr().String()))
		}
		logReplicator.Info(fmt.Sprintf("close connection, remote: %s", conn.RemoteAddr().String()))
		conn.Close()
	}
}

func streamingBackup(host, user, password, dataDir string, writer io.Writer) error {
	args := []string{
		"--compress",
		"--backup",
		"--safe-slave-backup",
		"--slave-info",
		"--stream=xbstream",
		fmt.Sprintf("--host=%s", host),
		fmt.Sprintf("--user=%s", user),
		fmt.Sprintf("--password=%s", password),
		fmt.Sprintf("--datadir=%s", dataDir),
	}
	cmd := exec.Command("xtrabackup", args...)
	cmd.Stdout = writer
	logReplicator.Info(fmt.Sprintf("exec backup commond: %s", cmd.String()))
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func streamingRestore(req *bindings.InvokeRequest) error {
	var (
		srcIP      = req.Metadata["SRC_POD_IP"]
		dataDir    = req.Metadata["DATA_DIR"]
		logBinDir  = req.Metadata["LOG_BIN"]
		dataVolume = req.Metadata["DATA_VOLUME"]
		tmpDir     = fmt.Sprintf("%s/tmp", dataVolume)
	)

	if err := setup4Restore(dataDir, tmpDir); err != nil {
		return err
	}

	if err := os.Chdir(tmpDir); err != nil {
		return err
	}
	// it will replicate data to current directory (the tmpDir)
	if err := streamingReplication(srcIP); err != nil {
		return err
	}

	postActions := []func(string, string, string) error{
		xtrabackupDecompress,
		xtrabackupPrepare,
		xtrabackupCleanup,
		xtrabackupMoveBack,
	}
	for _, action := range postActions {
		if err := action(dataDir, logBinDir, tmpDir); err != nil {
			return xtrabackupPostRestore(dataVolume, dataDir, tmpDir, err)
		}
	}
	return xtrabackupPostRestore(dataVolume, dataDir, tmpDir, nil)
}

func setup4Restore(dataDir, tmpDir string) error {
	if err := os.MkdirAll(dataDir, 0777); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(tmpDir, 0666); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func streamingReplication(srcIP string) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", srcIP, streamingReplicatorPort))
	if err != nil {
		return err
	}
	defer conn.Close()

	cmd := exec.Command(xbstreamBinary, "-x")
	cmd.Stdin = conn
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	if err = cmd.Run(); err != nil {
		return nil
	}
	return nil
}

func xtrabackupDecompress(dataDir, logBinDir, tmpDir string) error {
	args := fmt.Sprintf("--decompress  --target-dir=%s", tmpDir)
	return execXtrabackupCommand(args)
}

func xtrabackupPrepare(dataDir, logBinDir, tmpDir string) error {
	args := fmt.Sprintf("--prepare --target-dir=%s", tmpDir)
	return execXtrabackupCommand(args)
}

func xtrabackupMoveBack(dataDir, logBinDir, tmpDir string) error {
	args := fmt.Sprintf(" --move-back --target-dir=%s --datadir=%s --log-bin=%s", tmpDir, dataDir, logBinDir)
	return execXtrabackupCommand(args)
}

func xtrabackupCleanup(dataDir, logBinDir, tmpDir string) error {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".qp") {
			if err = os.Remove(e.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

func xtrabackupPostRestore(dataVolume, dataDir, tmpDir string, err error) error {
	if err1 := os.RemoveAll(tmpDir); err1 != nil {
		return err1
	}
	if err == nil { // restore succeed
		for _, file := range []string{
			fmt.Sprintf("%s/.xtrabackup_restore", dataDir),
			fmt.Sprintf("%s/.xtrabackup_restore_done", dataVolume),
		} {
			_, err1 := os.Create(file)
			if err1 != nil && !os.IsExist(err1) {
				return err1
			}
		}
		if err1 := os.Chmod(dataDir, 0777); err1 != nil {
			return err1
		}
	} else {
		if err1 := os.RemoveAll(dataDir); err1 != nil {
			return err1
		}
	}
	return err
}

func execXtrabackupCommand(args string) error {
	cmd := exec.Command(xtrabackupBinary, strings.Split(args, " ")...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logReplicator.Info("exec xtrabackup cmd error, cmd: %s, output: %s", cmd.String(), out)
		return err
	}
	return nil
}
