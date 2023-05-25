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

package preflight

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/yaml"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("concat_spec_test", func() {

	It("ConcatPreflightSpec Test", func() {
		targetByte := `
apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
 name: sample
spec:
 analyzers:
   - nodeResources:
       checkName: Must have at least 3 nodes in the cluster
       outcomes:
         - fail:
             when: "< 3"
             message: This application requires at least 3 nodes
         - warn:
             when: "< 5"
             message: This application recommends at last 5 nodes.
         - pass:
             message: This cluster has enough nodes.`
		sourceByte := `
apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
 name: sample
spec:
 collectors:
   - redis:
       collectorName: my-redis
       uri: rediss://default:replicated@server:6380
       tls:
         skipVerify: true
 analyzers:
   - redis:
       checkName: Must be redis 5.x or later
       collectorName: my-redis
       outcomes:
         - fail:
             when: "connected == false"
             message: Cannot connect to redis server
         - fail:
             when: "version < 5.0.0"
             message: The redis server must be at least version 5
         - pass:
             message: The redis connection checks out.`
		targetSpec := new(preflightv1beta2.Preflight)
		sourceSpec := new(preflightv1beta2.Preflight)
		Expect(yaml.Unmarshal([]byte(targetByte), targetSpec)).Should(Succeed())
		Expect(yaml.Unmarshal([]byte(sourceByte), sourceSpec)).Should(Succeed())
		var newSpec = ConcatPreflightSpec(nil, sourceSpec)
		Expect(newSpec).Should(Equal(sourceSpec))
		newSpec = ConcatPreflightSpec(targetSpec, nil)
		Expect(newSpec).Should(Equal(targetSpec))
		newSpec = ConcatPreflightSpec(targetSpec, sourceSpec)
		Expect(len(newSpec.Spec.Analyzers)).Should(Equal(2))
	})
	It("ConcatHostPreflightSpec Test", func() {
		targetByte := `
apiVersion: troubleshoot.sh/v1beta2
kind: HostPreflight
metadata:
 name: cpu
spec:
 collectors:
   - cpu: {}
 analyzers:
   - cpu:
       outcomes:
         - fail:
             when: "physical < 4"
             message: At least 4 physical CPU cores are required
         - fail:
             when: "logical < 8"
             message: At least 8 CPU cores are required
         - warn:
             when: "count < 16"
             message: At least 16 CPU cores preferred
         - pass:
             message: This server has sufficient CPU cores.`
		sourceByte := `
apiVersion: troubleshoot.sh/v1beta2
kind: HostPreflight
metadata:
 name: http
spec:
 collectors:
   - http:
       collectorName: registry
       get:
         url: https://registry.replicated.com
 analyzers:
   - http:
       collectorName: registry
       outcomes:
         - fail:
             when: "error"
             message: Error connecting to registry
         - pass:
             when: "statusCode == 404"
             message: Connected to registry
         - fail:
             message: "Unexpected response"`
		targetSpec := new(preflightv1beta2.HostPreflight)
		sourceSpec := new(preflightv1beta2.HostPreflight)
		Expect(yaml.Unmarshal([]byte(targetByte), targetSpec)).Should(Succeed())
		Expect(yaml.Unmarshal([]byte(sourceByte), sourceSpec)).Should(Succeed())
		var newSpec = ConcatHostPreflightSpec(nil, sourceSpec)
		Expect(newSpec).Should(Equal(sourceSpec))
		newSpec = ConcatHostPreflightSpec(targetSpec, nil)
		Expect(newSpec).Should(Equal(targetSpec))
		newSpec = ConcatHostPreflightSpec(targetSpec, sourceSpec)
		Expect(len(newSpec.Spec.Analyzers)).Should(Equal(2))
	})

	It("ExtractHostPreflightSpec Test", func() {
		sourceByte := `
apiVersion: troubleshoot.sh/v1beta2
kind: HostPreflight
metadata:
 name: http
spec:
 collectors:
   - http:
       collectorName: registry
       get:
         url: https://registry.replicated.com
 analyzers:
   - http:
       collectorName: registry
       outcomes:
         - fail:
             when: "error"
             message: Error connecting to registry
         - pass:
             when: "statusCode == 404"
             message: Connected to registry
         - fail:
             message: "Unexpected response"`
		sourceSpec := new(preflightv1beta2.HostPreflight)
		Expect(yaml.Unmarshal([]byte(sourceByte), sourceSpec)).Should(Succeed())
		Expect(ExtractHostPreflightSpec(nil)).Should(BeNil())
		newSpec := ExtractHostPreflightSpec(sourceSpec)
		Expect(len(newSpec.Spec.Collectors)).Should(Equal(1))
	})
})
