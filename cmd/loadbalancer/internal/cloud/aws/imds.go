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

package aws

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
)

type imdsService struct {
	IMDS
}

func (i *imdsService) GetMetadataWithContext(ctx context.Context, p string) (string, error) {
	return i.IMDS.GetMetadataWithContext(ctx, p)
}

func (i *imdsService) Region() (string, error) {
	return i.IMDS.Region()
}

func (i *imdsService) getList(ctx context.Context, key string) ([]string, error) {
	data, err := i.GetMetadataWithContext(ctx, key)
	if err != nil {
		return nil, err
	}
	return strings.Fields(data), nil
}

func (i *imdsService) getSecurityGroupIds(ctx context.Context, mac string) ([]string, error) {
	key := fmt.Sprintf("network/interfaces/macs/%s/security-group-ids", mac)
	return i.getList(ctx, key)
}

func (i *imdsService) getAZ(ctx context.Context) (string, error) {
	return i.GetMetadataWithContext(ctx, "placement/availability-zone")
}

func (i *imdsService) getLocalIPv4(ctx context.Context) (string, error) {
	return i.GetMetadataWithContext(ctx, "local-ipv4")
}

func (i *imdsService) getInstanceID(ctx context.Context) (string, error) {
	return i.GetMetadataWithContext(ctx, "instance-id")
}

func (i *imdsService) getInstanceType(ctx context.Context) (string, error) {
	return i.GetMetadataWithContext(ctx, "instance-type")
}

func (i *imdsService) getPrimaryMAC(ctx context.Context) (string, error) {
	return i.GetMetadataWithContext(ctx, "mac")
}

func (i *imdsService) getInterfaceIDByMAC(ctx context.Context, mac string) (string, error) {
	key := fmt.Sprintf("network/interfaces/macs/%s/interface-id", mac)
	return i.GetMetadataWithContext(ctx, key)
}

func (i *imdsService) getSubnetID(ctx context.Context, mac string) (string, error) {
	key := fmt.Sprintf("network/interfaces/macs/%s/subnet-id", mac)
	return i.GetMetadataWithContext(ctx, key)
}

func (i *imdsService) getMACs(ctx context.Context) ([]string, error) {
	macs, err := i.getList(ctx, "network/interfaces/macs")
	if err != nil {
		return nil, err
	}
	for index, item := range macs {
		macs[index] = strings.TrimSuffix(item, "/")
	}
	return macs, nil
}

func (i *imdsService) getInterfaceDeviceNumber(ctx context.Context, mac string) (int, error) {
	key := fmt.Sprintf("network/interfaces/macs/%s/device-number", mac)
	data, err := i.GetMetadataWithContext(ctx, key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(data)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (i *imdsService) getSubnetIPv4CIDRBlock(ctx context.Context, mac string) (string, error) {
	key := fmt.Sprintf("network/interfaces/macs/%s/subnet-ipv4-cidr-block", mac)
	return i.GetMetadataWithContext(ctx, key)
}

func (i *imdsService) getInterfacePrivateAddresses(ctx context.Context, mac string) ([]string, error) {
	key := fmt.Sprintf("network/interfaces/macs/%s/local-ipv4s", mac)
	return i.getList(ctx, key)
}

// imdsRequestError to provide the caller on the request status
type imdsRequestError struct {
	requestKey string
	err        error
}

func (e *imdsRequestError) Error() string {
	return fmt.Sprintf("failed to retrieve %s from instance metadata %v", e.requestKey, e.err)
}

func newIMDSRequestError(requestKey string, err error) *imdsRequestError {
	return &imdsRequestError{
		requestKey: requestKey,
		err:        err,
	}
}

var _ error = &imdsRequestError{}

// fakeIMDS is a trivial implementation of EC2MetadataIface using an in-memory map - for testing.
type fakeIMDS map[string]interface{}

func (f fakeIMDS) Region() (string, error) {
	return "mock", nil
}

// GetMetadataWithContext implements the EC2MetadataIface interface.
func (f fakeIMDS) GetMetadataWithContext(ctx context.Context, p string) (string, error) {
	result, ok := f[p]
	if !ok {
		result, ok = f[p+"/"] // Metadata API treats foo/ as foo
	}
	if !ok {
		notFoundErr := awserr.NewRequestFailure(awserr.New("NotFound", "not found", nil), http.StatusNotFound, "dummy-reqid")
		return "", newIMDSRequestError(p, notFoundErr)
	}
	switch v := result.(type) {
	case string:
		return v, nil
	case error:
		return "", v
	default:
		panic(fmt.Sprintf("unknown test metadata value type %T for %s", result, p))
	}
}
