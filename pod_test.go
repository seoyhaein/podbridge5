package podbridge5

import (
	"context"
	"errors"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/specgen"
	"strings"
	"testing"
)

func TestNewPodSpec_NoOptions(t *testing.T) {
	spec, err := NewPodSpec()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if spec.PodSpecGen.Name != "" {
		t.Errorf("expected empty Name, got %q", spec.PodSpecGen.Name)
	}
	if spec.PodSpecGen.NoInfra {
		t.Errorf("expected NoInfra=false, got true")
	}
	if spec.PodSpecGen.Labels != nil {
		t.Errorf("expected nil Labels, got %v", spec.PodSpecGen.Labels)
	}
	if len(spec.PodSpecGen.SharedNamespaces) != 0 {
		t.Errorf("expected no SharedNamespaces, got %v", spec.PodSpecGen.SharedNamespaces)
	}
}

func TestNewPodSpec_WithOptions(t *testing.T) {
	labels := map[string]string{"app": "demo", "env": "test"}
	spec, err := NewPodSpec(
		WithPodName("test-pod"),
		WithPodNoInfra(true),
		WithPodLabels(labels),
		WithPodSharedNamespaces("net", "ipc"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.PodSpecGen.Name != "test-pod" {
		t.Errorf("expected Name=test-pod, got %q", spec.PodSpecGen.Name)
	}
	if !spec.PodSpecGen.NoInfra {
		t.Errorf("expected NoInfra=true, got false")
	}
	if len(spec.PodSpecGen.Labels) != len(labels) {
		t.Errorf("expected Labels length %d, got %d", len(labels), len(spec.PodSpecGen.Labels))
	}
	for k, v := range labels {
		if spec.PodSpecGen.Labels[k] != v {
			t.Errorf("expected Labels[%q]=%q, got %q", k, v, spec.PodSpecGen.Labels[k])
		}
	}
	expectedNS := []string{"net", "ipc"}
	if len(spec.PodSpecGen.SharedNamespaces) != len(expectedNS) {
		t.Errorf("expected %d SharedNamespaces, got %d", len(expectedNS), len(spec.PodSpecGen.SharedNamespaces))
	}
	for i, ns := range expectedNS {
		if spec.PodSpecGen.SharedNamespaces[i] != ns {
			t.Errorf("expected SharedNamespaces[%d]=%q, got %q", i, ns, spec.PodSpecGen.SharedNamespaces[i])
		}
	}
}

func TestNewPodSpec_OptionError(t *testing.T) {
	bogus := func(*entities.PodSpec) error { return errors.New("boom") }
	_, err := NewPodSpec(bogus)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestNewPod_InvalidOption verifies that NewPod returns an error
// when one of the options fails.
func TestNewPod_InvalidOption(t *testing.T) {
	// bogus option always returns an error
	badOpt := func(_ *entities.PodSpec) error {
		return errors.New("boom")
	}
	_, err := NewPod(context.Background(), badOpt)
	if err == nil {
		t.Fatal("expected error from NewPod, got nil")
	}
	if err.Error() != "boom" {
		t.Fatalf("expected error 'boom', got %q", err.Error())
	}
}

// TestRemove_NilPod verifies that calling Remove on a nil Pod returns an error.
func TestRemove_NilPod(t *testing.T) {
	var p *Pod
	err := p.Remove(context.Background(), false)
	if err == nil {
		t.Fatal("expected error when calling Remove on nil Pod, got nil")
	}
	if err.Error() != "pod is nil" {
		t.Fatalf("expected 'pod is nil' error, got %q", err.Error())
	}
}

// TestRemove_InvalidID verifies that Remove returns an error for a non-existent Pod ID.
func TestRemove_InvalidID(t *testing.T) {
	// Use a Pod with an ID unlikely to exist
	p := &Pod{ID: "nonexistent-pod-id"}
	err := p.Remove(context.Background(), true)
	if err == nil {
		t.Fatal("expected error when removing non-existent pod, got nil")
	}
	// The error message should contain our prefix
	if !strings.Contains(err.Error(), "pod removal failed") {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

// TestWithPodNameSpec ensures that the WithPodName option correctly sets the spec field.
func TestWithPodNameSpec(t *testing.T) {
	spec := &entities.PodSpec{PodSpecGen: specgen.PodSpecGenerator{}}
	opt := WithPodName("testpod")
	if err := opt(spec); err != nil {
		t.Fatalf("WithPodName returned error: %v", err)
	}
	if spec.PodSpecGen.Name != "testpod" {
		t.Errorf("expected spec.PodSpecGen.Name 'testpod', got %q", spec.PodSpecGen.Name)
	}
}
