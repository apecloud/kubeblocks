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

package zookeeper

import (
	"fmt"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/assert"

	"github.com/dapr/components-contrib/state"
	"github.com/dapr/kit/ptr"
)

//go:generate mockgen -package zookeeper -source zk.go -destination zk_mock.go

// newConfig.
func TestNewConfig(t *testing.T) {
	t.Run("With all required fields", func(t *testing.T) {
		properties := map[string]string{
			"servers":        "127.0.0.1:3000,127.0.0.1:3001,127.0.0.1:3002",
			"sessionTimeout": "5s",
		}
		cp, err := newConfig(properties)
		assert.Equal(t, err, nil, fmt.Sprintf("Unexpected error: %v", err))
		assert.NotNil(t, cp, "failed to respond to missing data field")
		assert.Equal(t, []string{
			"127.0.0.1:3000", "127.0.0.1:3001", "127.0.0.1:3002",
		}, cp.servers, "failed to get servers")
		assert.Equal(t, 5*time.Second, cp.sessionTimeout, "failed to get DialTimeout")
	})

	t.Run("With all required fields", func(t *testing.T) {
		props := &properties{
			Servers:        "localhost:3000",
			SessionTimeout: "5s",
		}
		_, err := props.parse()
		assert.Equal(t, nil, err, "failed to read all fields")
	})
	t.Run("With missing servers", func(t *testing.T) {
		props := &properties{
			SessionTimeout: "5s",
		}
		_, err := props.parse()
		assert.NotNil(t, err, "failed to get missing endpoints error")
	})
	t.Run("With missing sessionTimeout", func(t *testing.T) {
		props := &properties{
			Servers: "localhost:3000",
		}
		_, err := props.parse()
		assert.NotNil(t, err, "failed to get invalid sessionTimeout error")
	})
}

// Get.
func TestGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockConn(ctrl)
	s := StateStore{conn: conn}

	t.Run("With key exists", func(t *testing.T) {
		conn.EXPECT().Get("foo").Return([]byte("bar"), &zk.Stat{Version: 123}, nil).Times(1)

		res, err := s.Get(&state.GetRequest{Key: "foo"})
		assert.NotNil(t, res, "Key must be exists")
		assert.Equal(t, "bar", string(res.Data), "Value must be equals")
		assert.Equal(t, ptr.Of("123"), res.ETag, "ETag must be equals")
		assert.NoError(t, err, "Key must be exists")
	})

	t.Run("With key non-exists", func(t *testing.T) {
		conn.EXPECT().Get("foo").Return(nil, nil, zk.ErrNoNode).Times(1)

		res, err := s.Get(&state.GetRequest{Key: "foo"})
		assert.Equal(t, &state.GetResponse{}, res, "Response must be empty")
		assert.NoError(t, err, "Non-existent key must not be treated as error")
	})
}

// Delete.
func TestDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockConn(ctrl)
	s := StateStore{conn: conn}

	etag := "123"
	t.Run("With key", func(t *testing.T) {
		conn.EXPECT().Delete("foo", int32(anyVersion)).Return(nil).Times(1)

		err := s.Delete(&state.DeleteRequest{Key: "foo"})
		assert.NoError(t, err, "Key must be exists")
	})

	t.Run("With key and version", func(t *testing.T) {
		conn.EXPECT().Delete("foo", int32(123)).Return(nil).Times(1)

		err := s.Delete(&state.DeleteRequest{Key: "foo", ETag: &etag})
		assert.NoError(t, err, "Key must be exists")
	})

	t.Run("With key and concurrency", func(t *testing.T) {
		conn.EXPECT().Delete("foo", int32(anyVersion)).Return(nil).Times(1)

		err := s.Delete(&state.DeleteRequest{
			Key:     "foo",
			ETag:    &etag,
			Options: state.DeleteStateOption{Concurrency: state.LastWrite},
		})
		assert.NoError(t, err, "Key must be exists")
	})

	t.Run("With delete error", func(t *testing.T) {
		conn.EXPECT().Delete("foo", int32(anyVersion)).Return(zk.ErrUnknown).Times(1)

		err := s.Delete(&state.DeleteRequest{Key: "foo"})
		assert.EqualError(t, err, "zk: unknown error")
	})

	t.Run("With delete and ignore NoNode error", func(t *testing.T) {
		conn.EXPECT().Delete("foo", int32(anyVersion)).Return(zk.ErrNoNode).Times(1)

		err := s.Delete(&state.DeleteRequest{Key: "foo"})
		assert.NoError(t, err, "Delete must be successful")
	})
}

// BulkDelete.
func TestBulkDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockConn(ctrl)
	s := StateStore{conn: conn}

	t.Run("With keys", func(t *testing.T) {
		conn.EXPECT().Multi([]interface{}{
			&zk.DeleteRequest{Path: "foo", Version: int32(anyVersion)},
			&zk.DeleteRequest{Path: "bar", Version: int32(anyVersion)},
		}).Return([]zk.MultiResponse{{}, {}}, nil).Times(1)

		err := s.BulkDelete([]state.DeleteRequest{{Key: "foo"}, {Key: "bar"}})
		assert.NoError(t, err, "Key must be exists")
	})

	t.Run("With keys and error", func(t *testing.T) {
		conn.EXPECT().Multi([]interface{}{
			&zk.DeleteRequest{Path: "foo", Version: int32(anyVersion)},
			&zk.DeleteRequest{Path: "bar", Version: int32(anyVersion)},
		}).Return([]zk.MultiResponse{
			{Error: zk.ErrUnknown}, {Error: zk.ErrNoAuth},
		}, nil).Times(1)

		err := s.BulkDelete([]state.DeleteRequest{{Key: "foo"}, {Key: "bar"}})
		assert.Equal(t, err.(*multierror.Error).Errors, []error{zk.ErrUnknown, zk.ErrNoAuth})
	})
	t.Run("With keys and ignore NoNode error", func(t *testing.T) {
		conn.EXPECT().Multi([]interface{}{
			&zk.DeleteRequest{Path: "foo", Version: int32(anyVersion)},
			&zk.DeleteRequest{Path: "bar", Version: int32(anyVersion)},
		}).Return([]zk.MultiResponse{
			{Error: zk.ErrNoNode}, {},
		}, nil).Times(1)

		err := s.BulkDelete([]state.DeleteRequest{{Key: "foo"}, {Key: "bar"}})
		assert.NoError(t, err, "Key must be exists")
	})
}

// Set.
func TestSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockConn(ctrl)
	s := StateStore{conn: conn}

	stat := &zk.Stat{}

	etag := "123"
	t.Run("With key", func(t *testing.T) {
		conn.EXPECT().Set("foo", []byte("\"bar\""), int32(anyVersion)).Return(stat, nil).Times(1)

		err := s.Set(&state.SetRequest{Key: "foo", Value: "bar"})
		assert.NoError(t, err, "Key must be set")
	})
	t.Run("With key and version", func(t *testing.T) {
		conn.EXPECT().Set("foo", []byte("\"bar\""), int32(123)).Return(stat, nil).Times(1)

		err := s.Set(&state.SetRequest{Key: "foo", Value: "bar", ETag: &etag})
		assert.NoError(t, err, "Key must be set")
	})
	t.Run("With key and concurrency", func(t *testing.T) {
		conn.EXPECT().Set("foo", []byte("\"bar\""), int32(anyVersion)).Return(stat, nil).Times(1)

		err := s.Set(&state.SetRequest{
			Key:     "foo",
			Value:   "bar",
			ETag:    &etag,
			Options: state.SetStateOption{Concurrency: state.LastWrite},
		})
		assert.NoError(t, err, "Key must be set")
	})

	t.Run("With error", func(t *testing.T) {
		conn.EXPECT().Set("foo", []byte("\"bar\""), int32(anyVersion)).Return(nil, zk.ErrUnknown).Times(1)

		err := s.Set(&state.SetRequest{Key: "foo", Value: "bar"})
		assert.EqualError(t, err, "zk: unknown error")
	})
	t.Run("With NoNode error and retry", func(t *testing.T) {
		conn.EXPECT().Set("foo", []byte("\"bar\""), int32(anyVersion)).Return(nil, zk.ErrNoNode).Times(1)
		conn.EXPECT().Create("foo", []byte("\"bar\""), int32(0), nil).Return("/foo", nil).Times(1)

		err := s.Set(&state.SetRequest{Key: "foo", Value: "bar"})
		assert.NoError(t, err, "Key must be create")
	})
}

// BulkSet.
func TestBulkSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockConn(ctrl)
	s := StateStore{conn: conn}

	t.Run("With keys", func(t *testing.T) {
		conn.EXPECT().Multi([]interface{}{
			&zk.SetDataRequest{Path: "foo", Data: []byte("\"bar\""), Version: int32(anyVersion)},
			&zk.SetDataRequest{Path: "bar", Data: []byte("\"foo\""), Version: int32(anyVersion)},
		}).Return([]zk.MultiResponse{{}, {}}, nil).Times(1)

		err := s.BulkSet([]state.SetRequest{
			{Key: "foo", Value: "bar"},
			{Key: "bar", Value: "foo"},
		})
		assert.NoError(t, err, "Key must be set")
	})

	t.Run("With keys and error", func(t *testing.T) {
		conn.EXPECT().Multi([]interface{}{
			&zk.SetDataRequest{Path: "foo", Data: []byte("\"bar\""), Version: int32(anyVersion)},
			&zk.SetDataRequest{Path: "bar", Data: []byte("\"foo\""), Version: int32(anyVersion)},
		}).Return([]zk.MultiResponse{
			{Error: zk.ErrUnknown}, {Error: zk.ErrNoAuth},
		}, nil).Times(1)

		err := s.BulkSet([]state.SetRequest{
			{Key: "foo", Value: "bar"},
			{Key: "bar", Value: "foo"},
		})
		assert.Equal(t, err.(*multierror.Error).Errors, []error{zk.ErrUnknown, zk.ErrNoAuth})
	})
	t.Run("With keys and retry NoNode error", func(t *testing.T) {
		conn.EXPECT().Multi([]interface{}{
			&zk.SetDataRequest{Path: "foo", Data: []byte("\"bar\""), Version: int32(anyVersion)},
			&zk.SetDataRequest{Path: "bar", Data: []byte("\"foo\""), Version: int32(anyVersion)},
		}).Return([]zk.MultiResponse{
			{Error: zk.ErrNoNode}, {},
		}, nil).Times(1)
		conn.EXPECT().Multi([]interface{}{
			&zk.CreateRequest{Path: "foo", Data: []byte("\"bar\"")},
		}).Return([]zk.MultiResponse{{}, {}}, nil).Times(1)

		err := s.BulkSet([]state.SetRequest{
			{Key: "foo", Value: "bar"},
			{Key: "bar", Value: "foo"},
		})
		assert.NoError(t, err, "Key must be set")
	})
}
