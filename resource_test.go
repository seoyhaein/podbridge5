package podbridge5

import (
	"bytes"
	"context"
	"encoding/csv"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/opencontainers/runtime-spec/specs-go"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestLogContainerStatsToCSV(t *testing.T) {
	// 1) Podman 소켓 연결
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	// 2) CPU/메모리 제한 없이 짧게 실행할 수 있는 컨테이너 생성 (sleep 5s)
	_, id := createTestContainer(t, ctx, []string{"sleep", "5"})
	t.Cleanup(func() { cleanupContainer(t, ctx, id) })

	// 3) 컨테이너 시작
	if err := containers.Start(ctx, id, nil); err != nil {
		t.Fatalf("failed to start container %s: %v", id, err)
	}

	// 4) stats 기록용 CSV 함수 실행 (3초 뒤에 ctx.Cancel() 되도록 timeout Context 사용)
	statsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	go func() {
		if err := LogContainerStatsToCSV(statsCtx, id, 1); err != nil {
			t.Errorf("LogContainerStatsToCSV returned error: %v", err)
		}
	}()

	// 5) 약간 기다려서 몇 번의 레코드가 쌓이도록 함
	time.Sleep(4 * time.Second)

	// 6) CSV 파일 경로 및 존재 여부 확인
	csvPath := id + "_stats.csv"
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("failed to read CSV file %q: %v", csvPath, err)
	}
	defer os.Remove(csvPath)

	// 7) CSV 내용 파싱
	r := csv.NewReader(bytes.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	// 8) 헤더와 최소 한 개 이상의 데이터 레코드가 있는지 검증
	if len(records) < 2 {
		t.Fatalf("expected at least header + 1 record, got %d rows", len(records))
	}
	header := records[0]
	wantHeader := []string{"Timestamp", "CPUNano", "MemUsage"}
	if !reflect.DeepEqual(header, wantHeader) {
		t.Errorf("unexpected header: got %v, want %v", header, wantHeader)
	}

	// 9) 첫 데이터 레코드 검사: timestamp 형식, 숫자 필드 파싱
	rec := records[1]
	if len(rec) != 3 {
		t.Fatalf("expected 3 columns in record, got %d", len(rec))
	}
	if _, err := time.Parse(time.RFC3339, rec[0]); err != nil {
		t.Errorf("invalid timestamp %q: %v", rec[0], err)
	}
	if _, err := strconv.ParseUint(rec[1], 10, 64); err != nil {
		t.Errorf("invalid CPUNano %q: %v", rec[1], err)
	}
	if _, err := strconv.ParseUint(rec[2], 10, 64); err != nil {
		t.Errorf("invalid MemUsage %q: %v", rec[2], err)
	}
}

func TestResourceOptions(t *testing.T) {
	tests := []struct {
		name            string
		opts            []ContainerOptions
		wantResources   *specs.LinuxResources
		wantOOMScoreAdj *int
	}{
		{
			name: "Only CPU limits",
			opts: []ContainerOptions{
				WithCPULimits(50000, 100000, 1024),
			},
			wantResources: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Quota:  ptrInt64(50000),
					Period: ptrUint64(100000),
					Shares: ptrUint64(1024),
				},
			},
			wantOOMScoreAdj: nil,
		},
		{
			name: "Only memory limit",
			opts: []ContainerOptions{
				WithMemoryLimit(256 * 1024 * 1024),
			},
			wantResources: &specs.LinuxResources{
				Memory: &specs.LinuxMemory{
					Limit: ptrInt64(256 * 1024 * 1024),
				},
			},
			wantOOMScoreAdj: nil,
		},
		{
			name: "Only OOM score adj",
			opts: []ContainerOptions{
				WithOOMScoreAdj(-500),
			},
			wantResources:   nil,
			wantOOMScoreAdj: ptrInt(-500),
		},
		{
			name: "All combined",
			opts: []ContainerOptions{
				WithCPULimits(20000, 50000, 512),
				WithMemoryLimit(128 * 1024 * 1024),
				WithOOMScoreAdj(100),
			},
			wantResources: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Quota:  ptrInt64(20000),
					Period: ptrUint64(50000),
					Shares: ptrUint64(512),
				},
				Memory: &specs.LinuxMemory{
					Limit: ptrInt64(128 * 1024 * 1024),
				},
			},
			wantOOMScoreAdj: ptrInt(100),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			spec, err := NewSpec(tt.opts...)
			if err != nil {
				t.Fatalf("NewSpec(%v) error: %v", tt.opts, err)
			}

			// Check ResourceLimits
			if tt.wantResources == nil {
				if spec.ResourceLimits != nil {
					t.Errorf("expected no ResourceLimits, got %+v", spec.ResourceLimits)
				}
			} else {
				if spec.ResourceLimits == nil {
					t.Fatalf("expected ResourceLimits, got nil")
				}
				if !reflect.DeepEqual(spec.ResourceLimits, tt.wantResources) {
					t.Errorf("ResourceLimits mismatch:\ngot  %+v\nwant %+v",
						spec.ResourceLimits, tt.wantResources)
				}
			}

			// Check OOMScoreAdj
			if tt.wantOOMScoreAdj == nil {
				if spec.OOMScoreAdj != nil {
					t.Errorf("expected no OOMScoreAdj, got %v", *spec.OOMScoreAdj)
				}
			} else {
				if spec.OOMScoreAdj == nil {
					t.Fatalf("expected OOMScoreAdj, got nil")
				}
				if *spec.OOMScoreAdj != *tt.wantOOMScoreAdj {
					t.Errorf("OOMScoreAdj = %d, want %d",
						*spec.OOMScoreAdj, *tt.wantOOMScoreAdj)
				}
			}
		})
	}
}
