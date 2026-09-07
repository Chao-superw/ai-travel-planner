package config

import (
	"testing"
	"time"
)

func TestPlanningTimeoutBudgetLeavesLayerAndRepairMargins(t *testing.T) {
	if ModelHTTPTimeout != 120*time.Second {
		t.Fatalf("model HTTP timeout changed: %v", ModelHTTPTimeout)
	}
	if PlannerRPCTimeout != 130*time.Second {
		t.Fatalf("planner RPC timeout changed: %v", PlannerRPCTimeout)
	}
	if JobExecutionTimeout != 300*time.Second {
		t.Fatalf("job execution and queue budget must remain five minutes: %v", JobExecutionTimeout)
	}
	if margin := PlannerRPCTimeout - ModelHTTPTimeout; margin <= 0 {
		t.Fatalf("RPC must leave positive transport margin after model HTTP timeout: %v", margin)
	}
	if remainder := JobExecutionTimeout - 2*ModelHTTPTimeout; remainder <= 0 {
		t.Fatalf("job must fit initial and repair model calls with time left for validation and persistence: %v", remainder)
	}
}
