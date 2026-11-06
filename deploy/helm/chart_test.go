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

package helm

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const dataProtectionContainerName = "dataprotection"

// leaderElectionTimingArgPrefixes are the Data Protection arguments whose value comes from
// `dataProtection.leaderElection` in values.yaml.
var leaderElectionTimingArgPrefixes = []string{
	"--leader-elect-lease-duration",
	"--leader-elect-renew-deadline",
	"--leader-elect-retry-period",
}

// renderDataProtectionContainerArgs renders the chart from the current directory and returns the
// args of the Data Protection container. The chart is rendered with the CI values plus the given
// `--set` overrides.
func renderDataProtectionContainerArgs(t *testing.T, setValues ...string) []string {
	t.Helper()

	helmBin, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is not installed, skipping the chart rendering test")
	}

	args := []string{
		"template", "test-release", ".",
		"-f", "ci/ci-values.yaml",
		"-s", "templates/dataprotection.yaml",
	}
	for _, setValue := range setValues {
		args = append(args, "--set", setValue)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(helmBin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm %s failed: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}

	deployment := &appsv1.Deployment{}
	if err := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(stdout.Bytes()), 4096).Decode(deployment); err != nil {
		t.Fatalf("decode the rendered Deployment: %v\n%s", err, stdout.String())
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == dataProtectionContainerName {
			return container.Args
		}
	}
	t.Fatalf("container %q not found in the rendered Deployment", dataProtectionContainerName)
	return nil
}

func findArg(args []string, prefix string) (string, bool) {
	for _, arg := range args {
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
			return arg, true
		}
	}
	return "", false
}

func requireArg(t *testing.T, args []string, want string) {
	t.Helper()
	if _, ok := findArg(args, want); !ok {
		t.Errorf("expected arg %q, got %v", want, args)
	}
}

func requireNoTimingArgs(t *testing.T, args []string) {
	t.Helper()
	for _, prefix := range leaderElectionTimingArgPrefixes {
		if arg, ok := findArg(args, prefix); ok {
			t.Errorf("did not expect %q, got args %v", arg, args)
		}
	}
}

// TestDataProtectionLeaderElectionDefault verifies that an installation without overrides keeps
// leader election enabled with its own ID, and renders no timing argument so the binary defaults
// (15/10/2 seconds) apply.
func TestDataProtectionLeaderElectionDefault(t *testing.T) {
	args := renderDataProtectionContainerArgs(t)

	requireArg(t, args, "--leader-elect")
	requireArg(t, args, "--leader-elect-id=abd03fda")
	requireNoTimingArgs(t, args)
}

// TestDataProtectionLeaderElectionOverride verifies that all three timings are rendered into the
// Deployment arguments when they are set.
func TestDataProtectionLeaderElectionOverride(t *testing.T) {
	args := renderDataProtectionContainerArgs(t,
		"dataProtection.leaderElection.leaseDuration=45",
		"dataProtection.leaderElection.renewDeadline=30",
		"dataProtection.leaderElection.retryPeriod=10",
	)

	requireArg(t, args, "--leader-elect")
	requireArg(t, args, "--leader-elect-lease-duration=45")
	requireArg(t, args, "--leader-elect-renew-deadline=30")
	requireArg(t, args, "--leader-elect-retry-period=10")
}

// TestDataProtectionLeaderElectionPartialOverride verifies that a single overridden timing does
// not prevent the others from falling back to the binary defaults.
func TestDataProtectionLeaderElectionPartialOverride(t *testing.T) {
	args := renderDataProtectionContainerArgs(t, "dataProtection.leaderElection.renewDeadline=30")

	requireArg(t, args, "--leader-elect-renew-deadline=30")
	if arg, ok := findArg(args, "--leader-elect-lease-duration"); ok {
		t.Errorf("did not expect %q, got args %v", arg, args)
	}
	if arg, ok := findArg(args, "--leader-elect-retry-period"); ok {
		t.Errorf("did not expect %q, got args %v", arg, args)
	}
}

// TestDataProtectionLeaderElectionAbsentValues verifies that an upgrade reusing values from a
// release that predates these settings still renders, falling back to the binary defaults.
func TestDataProtectionLeaderElectionAbsentValues(t *testing.T) {
	args := renderDataProtectionContainerArgs(t, "dataProtection.leaderElection=null")

	requireArg(t, args, "--leader-elect")
	requireArg(t, args, "--leader-elect-id=abd03fda")
	requireNoTimingArgs(t, args)
}

// TestDataProtectionLeaderElectIdPreserved verifies that overriding the timings keeps the existing
// `dataProtection.leaderElectId` configuration working.
func TestDataProtectionLeaderElectIdPreserved(t *testing.T) {
	args := renderDataProtectionContainerArgs(t,
		"dataProtection.leaderElectId=deadbeef",
		"dataProtection.leaderElection.renewDeadline=30",
	)

	requireArg(t, args, "--leader-elect-id=deadbeef")
	requireArg(t, args, "--leader-elect-renew-deadline=30")
}

// TestManagerLeaderElectionDoesNotAffectDataProtection verifies that the Data Protection timings
// stay independent of `manager.leaderElection`.
func TestManagerLeaderElectionDoesNotAffectDataProtection(t *testing.T) {
	args := renderDataProtectionContainerArgs(t,
		"manager.leaderElection.leaseDuration=45",
		"manager.leaderElection.renewDeadline=30",
		"manager.leaderElection.retryPeriod=10",
	)

	requireArg(t, args, "--leader-elect-id=abd03fda")
	requireNoTimingArgs(t, args)
}

// TestManagerLeaderElectionStillRendered verifies that the main manager keeps its own leader
// election timings, so adding the Data Protection settings did not break the existing wiring.
func TestManagerLeaderElectionStillRendered(t *testing.T) {
	helmBin, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is not installed, skipping the chart rendering test")
	}

	args := []string{
		"template", "test-release", ".",
		"-f", "ci/ci-values.yaml",
		"-s", "templates/deployment.yaml",
		"--set", "manager.leaderElection.leaseDuration=45",
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(helmBin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm %s failed: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}

	deployment := &appsv1.Deployment{}
	if err := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(stdout.Bytes()), 4096).Decode(deployment); err != nil {
		t.Fatalf("decode the rendered Deployment: %v\n%s", err, stdout.String())
	}
	var managerArgs []string
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			managerArgs = container.Args
			break
		}
	}
	if managerArgs == nil {
		t.Fatalf("container %q not found in the rendered Deployment", "manager")
	}

	requireArg(t, managerArgs, "--leader-elect-id=001c317f")
	requireArg(t, managerArgs, "--leader-elect-lease-duration=45")
}
