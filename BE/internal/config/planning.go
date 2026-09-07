package config

import "time"

const (
	ModelHTTPTimeout    = 120 * time.Second
	PlannerRPCTimeout   = 130 * time.Second
	JobExecutionTimeout = 300 * time.Second
)
