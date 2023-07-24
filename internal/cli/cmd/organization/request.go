package organization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/pkg/errors"
)

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

func NewRequest(method, path, token string, requestBody []byte) ([]byte, error) {
	client := cleanhttp.DefaultClient()
	req, err := http.NewRequest(method, path, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", path)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to perform request for %s", path)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errorResponse ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResponse)
		if err != nil {
			return nil, errors.Wrapf(err, "code: %d, failed to decode error response body for %s", resp.StatusCode, path)
		}
		return nil, fmt.Errorf("request failed with status code: %d\nreason:%s %s for %s", resp.StatusCode, errorResponse.Reason, errorResponse.Message, path)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read response body for %s", path)
	}

	return responseBody, nil
}
