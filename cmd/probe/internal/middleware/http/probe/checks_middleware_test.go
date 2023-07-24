package probe

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/spf13/viper"

	"github.com/stretchr/testify/assert"

	json "github.com/json-iterator/go"
)

func TestGetRequestBody(t *testing.T) {
	mock := make(map[string][]string)
	mock["sql"] = []string{"dd"}
	operation := "exec"
	body := GetRequestBody(operation, mock)

	meta := RequestMeta{
		Operation: operation,
		Metadata:  map[string]string{},
	}
	meta.Metadata["sql"] = "dd"
	marshal, err := json.Marshal(meta)
	assert.Nil(t, err)
	assert.Equal(t, marshal, body)
}

func TestSetMiddleware(t *testing.T) {
	t.Run("Rewrite Get", func(t *testing.T) {
		mockHandler := func(writer http.ResponseWriter, request *http.Request) {
			assert.Equal(t, request.Method, http.MethodPost)
		}

		request := httptest.NewRequest("GET", "/v1.0/bindings", nil)
		recorder := httptest.NewRecorder()

		middleware := SetMiddleware(mockHandler)
		middleware(recorder, request)
	})

	t.Run("Rewrite StatusCode", func(t *testing.T) {
		viper.Set("KB_PROBE_TOKEN", "ok")

		mockHandler := func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Add(statusCodeHeader, strconv.Itoa(http.StatusNotFound))
		}

		request := httptest.NewRequest("Post", "/v1.0/bindings", nil)
		request.Header.Add(tokenKeyInHeader, "ok")
		recorder := httptest.NewRecorder()

		middleware := SetMiddleware(mockHandler)
		middleware(recorder, request)

		code := recorder.Code
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("Token missing", func(t *testing.T) {
		viper.Set("KB_PROBE_TOKEN", "nothing")

		mockHandler := func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Add(statusCodeHeader, strconv.Itoa(http.StatusNotFound))
		}

		request := httptest.NewRequest("Post", "/v1.0/bindings", nil)
		recorder := httptest.NewRecorder()

		middleware := SetMiddleware(mockHandler)
		middleware(recorder, request)

		code := recorder.Code
		assert.Equal(t, http.StatusUnauthorized, code)

		s := recorder.Body.String()
		assert.Equal(t, "token missing... request refused!", s)
	})

	t.Run("Token mismatch", func(t *testing.T) {
		viper.Set("KB_PROBE_TOKEN", "nothing")

		mockHandler := func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Add(statusCodeHeader, strconv.Itoa(http.StatusNotFound))
		}

		request := httptest.NewRequest("Post", "/v1.0/bindings", nil)
		request.Header.Add(tokenKeyInHeader, "something")
		recorder := httptest.NewRecorder()

		middleware := SetMiddleware(mockHandler)
		middleware(recorder, request)

		code := recorder.Code
		assert.Equal(t, http.StatusUnauthorized, code)

		s := recorder.Body.String()
		assert.Equal(t, "token mismatch: something is invalid.", s)
	})
}
