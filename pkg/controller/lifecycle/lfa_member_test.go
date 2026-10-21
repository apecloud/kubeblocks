/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package lifecycle

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type memberParameterAgent struct{ requests []proto.ActionRequest }

func (a *memberParameterAgent) Close() error { return nil }
func (a *memberParameterAgent) Action(_ context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
	a.requests = append(a.requests, req)
	return proto.ActionResponse{}, nil
}

func TestMemberActionParametersUsePodDNS(t *testing.T) {
	for _, test := range []struct {
		name, hostname, subdomain string
	}{
		{name: "explicit Pod hostname", hostname: "database", subdomain: "standalone-headless"},
		{name: "default Pod hostname", subdomain: "standalone-headless"},
		{name: "legacy Component fallback", hostname: "ignored-without-subdomain"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "standalone-0", Namespace: "default"}, Spec: corev1.PodSpec{Hostname: test.hostname, Subdomain: test.subdomain}}
			candidate := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "standalone-1", Namespace: "default"}, Spec: corev1.PodSpec{Hostname: "candidate", Subdomain: test.subdomain}}
			action := &apps.Action{Exec: &apps.ExecAction{Command: []string{"true"}}}
			lc, err := New(pod.Namespace, "cluster", "component", &apps.ComponentLifecycleActions{MemberJoin: action, MemberLeave: action, Switchover: action}, nil, pod, []*corev1.Pod{pod, candidate})
			if err != nil {
				t.Fatal(err)
			}
			agent := &memberParameterAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			if err := lc.MemberJoin(ctx, nil, nil); err != nil {
				t.Fatal(err)
			}
			if err := lc.MemberLeave(ctx, nil, nil); err != nil {
				t.Fatal(err)
			}
			if err := lc.Switchover(ctx, nil, nil, candidate.Name); err != nil {
				t.Fatal(err)
			}
			if err := lc.Switchover(ctx, nil, nil, "unobserved-candidate"); err != nil {
				t.Fatal(err)
			}
			if err := lc.Switchover(ctx, nil, nil, ""); err != nil {
				t.Fatal(err)
			}
			if len(agent.requests) != 5 {
				t.Fatalf("expected five action requests, got %#v", agent.requests)
			}
			want := intctrlutil.PodFQDN(pod.Namespace, "cluster-component", pod.Name)
			wantCandidate := intctrlutil.PodFQDN(pod.Namespace, "cluster-component", candidate.Name)
			if test.subdomain != "" {
				hostname := test.hostname
				if hostname == "" {
					hostname = pod.Name
				}
				want = hostname + "." + intctrlutil.ServiceFQDN(pod.Namespace, test.subdomain)
				wantCandidate = candidate.Spec.Hostname + "." + intctrlutil.ServiceFQDN(candidate.Namespace, test.subdomain)
			}
			for i, key := range []string{joinMemberPodFQDNVar, leaveMemberPodFQDNVar, switchoverCurrentFQDN, switchoverCurrentFQDN, switchoverCurrentFQDN} {
				if got := agent.requests[i].Parameters[key]; got != want {
					t.Errorf("%s parameter %s = %q, want %q", agent.requests[i].Action, key, got, want)
				}
			}
			if got := agent.requests[2].Parameters[switchoverCandidateFQDN]; got != wantCandidate {
				t.Errorf("candidate FQDN = %q, want %q", got, wantCandidate)
			}
			if got, want := agent.requests[3].Parameters[switchoverCandidateFQDN], intctrlutil.PodFQDN(pod.Namespace, "cluster-component", "unobserved-candidate"); got != want {
				t.Errorf("unobserved candidate FQDN = %q, want legacy fallback %q", got, want)
			}
			if _, ok := agent.requests[4].Parameters[switchoverCandidateFQDN]; ok {
				t.Fatal("automatic switchover invented a candidate FQDN")
			}
		})
	}
}
