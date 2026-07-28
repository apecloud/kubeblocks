/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	actionStateDirMode  = 0700
	actionStateFileMode = 0600
)

type actionRecord struct {
	RequestHash            string              `json:"requestHash"`
	Running                bool                `json:"running"`
	Result                 *storedActionResult `json:"result,omitempty"`
	StartedAt              time.Time           `json:"startedAt"`
	CompletedAt            *time.Time          `json:"completedAt,omitempty"`
	ResultPersistenceError error               `json:"-"`
}

type storedActionResult struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Output  []byte `json:"output,omitempty"`
}

func newStoredActionResult(output []byte, err error) *storedActionResult {
	result := &storedActionResult{Output: append([]byte(nil), output...)}
	if err != nil {
		result.Error = proto.Error2Type(err)
		result.Message = err.Error()
	}
	return result
}

func (r *storedActionResult) response() ([]byte, error) {
	if r == nil || r.Error == "" {
		if r == nil {
			return nil, nil
		}
		return append([]byte(nil), r.Output...), nil
	}
	base := proto.Type2Error(r.Error)
	if r.Message == "" {
		return nil, base
	}
	return nil, &storedResponseError{message: r.Message, cause: base}
}

type storedResponseError struct {
	message string
	cause   error
}

func (e *storedResponseError) Error() string {
	return e.message
}

func (e *storedResponseError) Unwrap() error {
	return e.cause
}

type actionRequestIdentity struct {
	Action         string             `json:"action"`
	Parameters     map[string]string  `json:"parameters"`
	Arguments      [][]string         `json:"arguments"`
	TimeoutSeconds int32              `json:"timeoutSeconds"`
	RetryPolicy    *requestRetryState `json:"retryPolicy"`
}

type requestRetryState struct {
	MaxRetries    int   `json:"maxRetries"`
	RetryInterval int64 `json:"retryInterval"`
}

func actionRequestHash(req *proto.ActionRequest, timeout *int32, retryPolicy *proto.RetryPolicy) (string, error) {
	parameters := req.Parameters
	if parameters == nil {
		parameters = map[string]string{}
	}
	arguments := req.Arguments
	if len(arguments) == 0 {
		arguments = [][]string{}
	} else {
		arguments = append([][]string(nil), arguments...)
		for i, argument := range arguments {
			if argument == nil {
				arguments[i] = []string{}
			}
		}
	}
	var retry *requestRetryState
	if retryPolicy != nil && retryPolicy.MaxRetries > 0 {
		retry = &requestRetryState{
			MaxRetries:    retryPolicy.MaxRetries,
			RetryInterval: int64(max(retryPolicy.RetryInterval, 0)),
		}
	}
	identity := actionRequestIdentity{
		Action:         req.Action,
		Parameters:     parameters,
		Arguments:      arguments,
		TimeoutSeconds: effectiveTimeoutSeconds(timeout),
		RetryPolicy:    retry,
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return "", errors.Wrap(err, "marshal Action request identity")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func effectiveTimeoutSeconds(timeout *int32) int32 {
	if timeout == nil || *timeout == 0 {
		return int32(defaultActionCallTimeout / time.Second)
	}
	if *timeout < 0 {
		return -1
	}
	return *timeout
}

type actionStateStore struct {
	dir string
}

func newActionStateStore(dir string) (*actionStateStore, error) {
	store := &actionStateStore{dir: dir}
	if dir == "" {
		return store, nil
	}
	if err := os.MkdirAll(dir, actionStateDirMode); err != nil {
		return nil, errors.Wrap(err, "create Action state directory")
	}
	if err := os.Chmod(dir, actionStateDirMode); err != nil {
		return nil, errors.Wrap(err, "set Action state directory permissions")
	}
	return store, nil
}

func (s *actionStateStore) enabled() bool {
	return s != nil && s.dir != ""
}

func (s *actionStateStore) load(action string) (*actionRecord, error) {
	if !s.enabled() {
		return nil, nil
	}
	data, err := os.ReadFile(s.path(action))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "read state for Action %s", action)
	}
	record := &actionRecord{}
	if err := json.Unmarshal(data, record); err != nil {
		return nil, errors.Wrapf(err, "decode state for Action %s", action)
	}
	if record.RequestHash == "" {
		return nil, fmt.Errorf("state for Action %s has an empty request hash", action)
	}
	if !record.Running && record.Result == nil {
		return nil, fmt.Errorf("terminal state for Action %s has no result", action)
	}
	return record, nil
}

func (s *actionStateStore) save(action string, record *actionRecord) error {
	if !s.enabled() {
		return nil
	}
	data, err := json.Marshal(record)
	if err != nil {
		return errors.Wrapf(err, "encode state for Action %s", action)
	}
	file, err := os.CreateTemp(s.dir, ".action-state-*")
	if err != nil {
		return errors.Wrapf(err, "create temporary state for Action %s", action)
	}
	tmp := file.Name()
	defer os.Remove(tmp)

	if err := file.Chmod(actionStateFileMode); err != nil {
		_ = file.Close()
		return errors.Wrapf(err, "set state permissions for Action %s", action)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return errors.Wrapf(err, "write state for Action %s", action)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return errors.Wrapf(err, "sync state for Action %s", action)
	}
	if err := file.Close(); err != nil {
		return errors.Wrapf(err, "close state for Action %s", action)
	}
	if err := os.Rename(tmp, s.path(action)); err != nil {
		return errors.Wrapf(err, "replace state for Action %s", action)
	}
	return syncDirectory(s.dir)
}

func (s *actionStateStore) path(action string) string {
	sum := sha256.Sum256([]byte(action))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func syncDirectory(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return errors.Wrap(err, "open Action state directory")
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil {
		return errors.Wrap(err, "sync Action state directory")
	}
	return nil
}
