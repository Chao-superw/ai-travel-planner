package store

import (
	"context"
	"testing"
	"time"

	"ai-travel/internal/config"
	"ai-travel/internal/domain"
)

func TestClaimKeepsQueueAndLeaseBoundsWhileUsingExecutionBudget(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	staleOwner := domain.User{ID: domain.ID(), Username: domain.ID(), Role: "user"}
	freshOwner := domain.User{ID: domain.ID(), Username: domain.ID(), Role: "user"}
	if err := s.CreateUser(ctx, staleOwner, "hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateUser(ctx, freshOwner, "hash"); err != nil {
		t.Fatal(err)
	}
	stale, err := s.CreateJob(ctx, staleOwner.ID, "generate", "new", "stale", "hash", domain.JobInput{}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE planning_jobs SET created_at=clock_timestamp()-interval '301 seconds' WHERE id=$1", stale.ID); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.CreateJob(ctx, freshOwner.ID, "generate", "new", "fresh", "hash", domain.JobInput{}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	blockers, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blockers.Rollback(ctx)
	if _, err = blockers.Exec(ctx, "SELECT id FROM planning_jobs WHERE status='queued' AND id<>$1 AND id<>$2 FOR UPDATE", fresh.ID, stale.ID); err != nil {
		t.Fatal(err)
	}

	before := time.Now()
	record, err := s.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil || record.Job.ID != fresh.ID {
		t.Fatalf("five-minute queue bound changed or wrong job claimed: %#v", record)
	}
	var deadline, lease time.Time
	if err = s.Pool.QueryRow(ctx, "SELECT deadline_at,lease_expires_at FROM planning_jobs WHERE id=$1", fresh.ID).Scan(&deadline, &lease); err != nil {
		t.Fatal(err)
	}
	if got := deadline.Sub(before); got < config.JobExecutionTimeout-time.Second || got > config.JobExecutionTimeout+time.Second {
		t.Fatalf("execution deadline=%v", got)
	}
	if got := lease.Sub(before); got < 29*time.Second || got > 31*time.Second {
		t.Fatalf("lease duration changed: %v", got)
	}
}
