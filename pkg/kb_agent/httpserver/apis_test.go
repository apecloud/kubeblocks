/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package httpserver

import (
	"encoding/json"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"testing"
)

func TestEndpoints(t *testing.T) {
	eps := Endpoints()
	assert.NotNil(t, eps)
}

func TestActionHandler(t *testing.T) {
	actionHandlerSpecs := map[string]util.HandlerSpec{
		"success": {
			Command: []string{"echo", "hello"},
		},
		"test": {
			GPRC: map[string]string{"test": "test"},
		},
		"failed": {
			TimeoutSeconds: 0,
			Command:        nil,
			GPRC:           nil,
			CronJob:        nil,
		},
	}
	actionJson, _ := json.Marshal(actionHandlerSpecs)
	viper.Set(constant.KBEnvActionHandlers, string(actionJson))
	assert.Nil(t, handlers.InitHandlers())

	t.Run("unmarshal HTTP body failed", func(t *testing.T) {
		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.Header.SetMethod(fasthttp.MethodPost)
		reqCtx.Request.Header.SetContentType("application/json")
		reqCtx.Request.SetBody([]byte(`{"action":"test"`)) // Malformed JSON
		actionHandler(reqCtx)
		assert.Equal(t, fasthttp.StatusBadRequest, reqCtx.Response.StatusCode())
		assert.JSONEq(t, `{"errorCode":"ERR_MALFORMED_REQUEST","message":"unmarshal HTTP body failed: unexpected end of JSON input"}`, string(reqCtx.Response.Body()))
	})

	//t.Run("marshal request data field", func(t *testing.T) {
	//	reqCtx := &fasthttp.RequestCtx{}
	//	reqCtx.Request.Header.SetMethod(fasthttp.MethodPost)
	//	reqCtx.Request.Header.SetContentType("application/json")
	//	reqCtx.Request.SetBody([]byte(`{"data":}`))
	//	actionHandler(reqCtx)
	//	assert.Equal(t, fasthttp.StatusInternalServerError, reqCtx.Response.StatusCode())
	//	assert.JSONEq(t, `{"errorCode":"ERR_MALFORMED_REQUEST_DATA","message":"marshal request data field: json: unsupported type: func()"}`, string(reqCtx.Response.Body()))
	//})

	t.Run("no action in request", func(t *testing.T) {
		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.Header.SetMethod(fasthttp.MethodPost)
		reqCtx.Request.Header.SetContentType("application/json")
		reqCtx.Request.SetBody([]byte(`{}`))
		actionHandler(reqCtx)
		assert.Equal(t, fasthttp.StatusBadRequest, reqCtx.Response.StatusCode())
		assert.JSONEq(t, `{"errorCode":"ERR_MALFORMED_REQUEST_DATA","message":"no action in request"}`, string(reqCtx.Response.Body()))
	})

	t.Run("action not implemented", func(t *testing.T) {
		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.Header.SetMethod(fasthttp.MethodPost)
		reqCtx.Request.Header.SetContentType("application/json")
		reqCtx.Request.SetBody([]byte(`{"action":"test"}`))
		actionJson, _ := json.Marshal(actionHandlerSpecs)
		viper.Set(constant.KBEnvActionHandlers, string(actionJson))
		assert.Nil(t, handlers.InitHandlers())
		actionHandler(reqCtx)
		assert.Equal(t, fasthttp.StatusNotImplemented, reqCtx.Response.StatusCode())
		assert.JSONEq(t, `{"errorCode":"ERR_ACTION_FAILED","message":"action exec failed: not implemented"}`, string(reqCtx.Response.Body()))
	})

	t.Run("action exec failed", func(t *testing.T) {
		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.Header.SetMethod(fasthttp.MethodPost)
		reqCtx.Request.Header.SetContentType("application/json")
		reqCtx.Request.SetBody([]byte(`{"action":"failed"}`))
		actionJson, _ := json.Marshal(actionHandlerSpecs)
		viper.Set(constant.KBEnvActionHandlers, string(actionJson))
		assert.Nil(t, handlers.InitHandlers())
		actionHandler(reqCtx)
		assert.Equal(t, fasthttp.StatusInternalServerError, reqCtx.Response.StatusCode())
	})

	// TODO Possible unreachable branch
	//t.Run("action exec empty", func(t *testing.T) {
	//	reqCtx := &fasthttp.RequestCtx{}
	//	reqCtx.Request.Header.SetMethod(fasthttp.MethodPost)
	//	reqCtx.Request.Header.SetContentType("application/json")
	//	reqCtx.Request.SetBody([]byte(`{"action":"empty"}`))
	//	actionHandlerSpecs := map[string]util.HandlerSpec{
	//		"empty": {
	//			Command: []string{},
	//		},
	//	}
	//	actionJson, _ := json.Marshal(actionHandlerSpecs)
	//	viper.Set(constant.KBEnvActionHandlers, string(actionJson))
	//	assert.Nil(t, handlers.InitHandlers())
	//	actionHandler(reqCtx)
	//	assert.Equal(t, fasthttp.StatusNoContent, reqCtx.Response.StatusCode())
	//	assert.JSONEq(t, `null`, string(reqCtx.Response.Body()))
	//})

	t.Run("action exec success", func(t *testing.T) {
		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.Header.SetMethod(fasthttp.MethodPost)
		reqCtx.Request.Header.SetContentType("application/json")
		reqCtx.Request.SetBody([]byte(`{"action":"success"}`))
		actionHandler(reqCtx)
		assert.Equal(t, fasthttp.StatusOK, reqCtx.Response.StatusCode())
		assert.JSONEq(t, `{"message":"hello\n"}`, string(reqCtx.Response.Body()))
	})
}
