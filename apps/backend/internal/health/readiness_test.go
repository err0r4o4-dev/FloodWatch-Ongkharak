package health

import (
	"context"
	"errors"
	"testing"
)

func TestServiceReportsNotReadyWhenDependencyFails(t *testing.T) {
	service := NewService([]Checker{
		fixedChecker{name: "postgres"},
		fixedChecker{name: "redis", err: errors.New("unavailable")},
		fixedChecker{name: "minio"},
	})

	report := service.Check(context.Background())
	if report.Ready {
		t.Fatal("report.Ready = true, want false")
	}
	if report.Dependencies["redis"].Status != "unavailable" {
		t.Fatalf("redis status = %q, want unavailable", report.Dependencies["redis"].Status)
	}
}

func TestServiceReportsReadyWhenAllDependenciesPass(t *testing.T) {
	service := NewService([]Checker{
		fixedChecker{name: "postgres"},
		fixedChecker{name: "redis"},
		fixedChecker{name: "minio"},
	})

	report := service.Check(context.Background())
	if !report.Ready {
		t.Fatalf("report.Ready = false, dependencies = %#v", report.Dependencies)
	}
}

type fixedChecker struct {
	name string
	err  error
}

func (checker fixedChecker) Name() string { return checker.name }

func (checker fixedChecker) Check(context.Context) error { return checker.err }
