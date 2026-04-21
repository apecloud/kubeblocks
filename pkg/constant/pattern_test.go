package constant

import (
	"strings"
	"testing"
)

func TestGenerateComponentServiceNameShortensLongNames(t *testing.T) {
	clusterName := "vlk-smoke-rerun-202206-restore-202950"
	compName := "valkey-sentinel"
	svcName := "valkey-sentinel"

	got := GenerateComponentServiceName(clusterName, compName, svcName)
	if len(got) > 63 {
		t.Fatalf("service name length = %d, want <= 63: %s", len(got), got)
	}
	if !strings.HasPrefix(got, "vlk-smoke-rerun-202206-restore-202950-valkey") {
		t.Fatalf("service name prefix = %s, want stable readable prefix", got)
	}
	if got == clusterName+"-"+compName+"-"+svcName {
		t.Fatalf("service name was not shortened: %s", got)
	}
}

func TestGenerateComponentHeadlessServiceNameShortensLongNames(t *testing.T) {
	clusterName := "vlk-smoke-rerun-202206-restore-202950"
	compName := "valkey-sentinel"
	svcName := "valkey-sentinel"

	got := GenerateComponentHeadlessServiceName(clusterName, compName, svcName)
	if len(got) > 63 {
		t.Fatalf("headless service name length = %d, want <= 63: %s", len(got), got)
	}
	if !strings.HasSuffix(got, "headless") {
		t.Fatalf("headless service name = %s, want suffix headless", got)
	}
	if len(got) > 63 {
		t.Fatalf("headless service name length = %d, want <= 63: %s", len(got), got)
	}
}

func TestGenerateComponentServiceNameKeepsShortNames(t *testing.T) {
	got := GenerateComponentServiceName("demo", "valkey", "sentinel")
	want := "demo-valkey-sentinel"
	if got != want {
		t.Fatalf("service name = %s, want %s", got, want)
	}
}

func TestGenerateComponentServiceNameAvoidsCollisionsForSamePrefix(t *testing.T) {
	clusterName := "vlk-smoke-rerun-202206-restore-202950"
	compNameA := "valkey-sentinel-alpha"
	compNameB := "valkey-sentinel-bravo"
	svcName := "valkey-sentinel"

	nameA := GenerateComponentServiceName(clusterName, compNameA, svcName)
	nameB := GenerateComponentServiceName(clusterName, compNameB, svcName)
	if nameA == nameB {
		t.Fatalf("different long inputs collided: %s", nameA)
	}
	if len(nameA) > 63 || len(nameB) > 63 {
		t.Fatalf("service names must be <= 63: %s (%d), %s (%d)", nameA, len(nameA), nameB, len(nameB))
	}
}

func TestGenerateComponentServiceNameIsDeterministic(t *testing.T) {
	clusterName := "vlk-smoke-rerun-202206-restore-202950"
	compName := "valkey-sentinel"
	svcName := "valkey-sentinel"

	name1 := GenerateComponentServiceName(clusterName, compName, svcName)
	name2 := GenerateComponentServiceName(clusterName, compName, svcName)
	if name1 != name2 {
		t.Fatalf("service name generation is not deterministic: %s != %s", name1, name2)
	}
}
