package podbridge5

import (
	"context"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/pods"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/specgen"
)

// TODO 여기서 부터 시작.
// pod 같은 경우는 일단 파이프라인의 규격이 정해지면 그것에 따라서 api 의 스펙이 확정될 것으로 판단.

// infra container 확인 하고, volume 을 pod 에 넣어두는 것을 생각해보자. 실제로는 infra container 에 넣는 것인데..

// Pod wraps the entities.PodSpec and the created pod's ID.
type Pod struct {
	Spec *entities.PodSpec
	ID   string
}

// PodOption defines a functional option for PodSpecGen.
type PodOption func(*entities.PodSpec) error

// NewPodSpec creates a new PodSpec using functional options.
func NewPodSpec(opts ...PodOption) (*entities.PodSpec, error) {
	spec := &entities.PodSpec{
		PodSpecGen: specgen.PodSpecGenerator{},
	}
	for _, opt := range opts {
		if err := opt(spec); err != nil {
			return nil, err
		}
	}
	return spec, nil
}

// NewPod creates and returns a Pod by building the spec with options and creating it.
func NewPod(ctx context.Context, opts ...PodOption) (*Pod, error) {
	spec := &entities.PodSpec{
		PodSpecGen: specgen.PodSpecGenerator{},
	}
	for _, opt := range opts {
		if err := opt(spec); err != nil {
			return nil, err
		}
	}

	report, err := pods.CreatePodFromSpec(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("pod creation failed: %w", err)
	}

	return &Pod{Spec: spec, ID: report.Id}, nil
}

func (p *Pod) Remove(ctx context.Context, force bool) error {
	if p == nil {
		return errors.New("pod is nil")
	}
	// 7초 기다린 뒤 강제 제거
	var timeout uint = 7

	opts := &pods.RemoveOptions{
		Force:   &force,
		Timeout: &timeout,
	}

	_, err := pods.Remove(ctx, p.ID, opts)
	if err != nil {
		return fmt.Errorf("pod removal failed: %w", err)
	}
	return nil
}

// WithPodName sets the pod's name.
func WithPodName(name string) PodOption {
	return func(gen *entities.PodSpec) error {
		gen.PodSpecGen.Name = name
		return nil
	}
}

// WithPodNoInfra configures whether to skip creating an infra container.
func WithPodNoInfra(noInfra bool) PodOption {
	return func(gen *entities.PodSpec) error {
		gen.PodSpecGen.NoInfra = noInfra
		return nil
	}
}

// WithPodLabels sets labels for the pod.
func WithPodLabels(labels map[string]string) PodOption {
	return func(gen *entities.PodSpec) error {
		gen.PodSpecGen.Labels = labels
		return nil
	}
}

// WithPodSharedNamespaces sets the namespaces to share (e.g., []string{"net","ipc"}).
func WithPodSharedNamespaces(namespaces ...string) PodOption {
	return func(gen *entities.PodSpec) error {
		gen.PodSpecGen.SharedNamespaces = namespaces
		return nil
	}
}

// CreatePod creates a new pod using a prepared PodSpec.
// It assumes the context has been initialized with a Podman client connection.
func CreatePod(ctx context.Context, podSpec *entities.PodSpec) (string, error) {
	report, err := pods.CreatePodFromSpec(ctx, podSpec)
	if err != nil {
		return "", fmt.Errorf("pod creation failed: %w", err)
	}
	return report.Id, nil
}

// RemovePod deletes a pod and all its containers and resources.
// If force is true, running containers will be removed as well.
func RemovePod(ctx context.Context, podID string, force bool) error {
	// 10초 기다린 뒤 강제 제거
	var timeout uint = 10

	opts := &pods.RemoveOptions{
		Force:   &force,
		Timeout: &timeout,
	}
	_, err := pods.Remove(ctx, podID, opts)
	if err != nil {
		return fmt.Errorf("pod removal failed: %w", err)
	}
	return nil
}
