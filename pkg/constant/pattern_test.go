package constant

import (
	"strings"
	"testing"
)

func TestShortenKubeNameKeepsShortNames(t *testing.T) {
	got := ShortenKubeName("demo-valkey-sentinel", KubeNameMaxLength)
	if got != "demo-valkey-sentinel" {
		t.Fatalf("shortened name = %s, want unchanged short name", got)
	}
}

func TestShortenKubeNameShortensLongNames(t *testing.T) {
	raw := "vlk-smoke-rerun-202206-restore-202950-valkey-sentinel-valkey-sentinel"

	got := ShortenKubeName(raw, KubeNameMaxLength)
	if len(got) > KubeNameMaxLength {
		t.Fatalf("shortened name length = %d, want <= %d: %s", len(got), KubeNameMaxLength, got)
	}
	if !strings.HasPrefix(got, "vlk-smoke-rerun-202206-restore-202950-valkey") {
		t.Fatalf("shortened name prefix = %s, want stable readable prefix", got)
	}
	if got == raw {
		t.Fatalf("name was not shortened: %s", got)
	}
}

func TestShortenKubeNameAvoidsCollisionsForSamePrefix(t *testing.T) {
	rawA := "vlk-smoke-rerun-202206-restore-202950-valkey-sentinel-alpha"
	rawB := "vlk-smoke-rerun-202206-restore-202950-valkey-sentinel-bravo"

	nameA := ShortenKubeName(rawA, 40)
	nameB := ShortenKubeName(rawB, 40)
	if nameA == nameB {
		t.Fatalf("different long inputs collided: %s", nameA)
	}
}

func TestShortenKubeNameIsDeterministic(t *testing.T) {
	raw := "vlk-smoke-rerun-202206-restore-202950-valkey-sentinel-valkey-sentinel"

	name1 := ShortenKubeName(raw, KubeNameMaxLength)
	name2 := ShortenKubeName(raw, KubeNameMaxLength)
	if name1 != name2 {
		t.Fatalf("shortened name generation is not deterministic: %s != %s", name1, name2)
	}
}

func TestShortenKubeNameWithSuffixPreservesSuffix(t *testing.T) {
	raw := "vlk-smoke-rerun-202206-restore-202950-valkey-sentinel-valkey-sentinel"

	got := ShortenKubeNameWithSuffix(raw, "headless", KubeNameMaxLength)
	if len(got) > KubeNameMaxLength {
		t.Fatalf("shortened name length = %d, want <= %d: %s", len(got), KubeNameMaxLength, got)
	}
	if !strings.HasSuffix(got, "headless") {
		t.Fatalf("shortened name = %s, want suffix headless", got)
	}
}
