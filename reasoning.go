package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type ReActStep struct {
	StepType  string
	Content   string
	Timestamp time.Time
}

type ReActTrace struct {
	IncidentID string
	PodID      string
	Steps      []ReActStep
	Conclusion string
}

// Updated interface to support both "Read" and "Action" tools
type SRETool interface {
	Name() string
	Execute(target string, cmd string) (string, error)
}

type HealthCheckTool struct {
	RDB *redis.Client
}
func (t *HealthCheckTool) Name() string { return "HealthCheck" }
func (t *HealthCheckTool) Execute(podID string, _ string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	status, err := t.RDB.Get(ctx, "sim:health:"+podID).Result()
	if err == redis.Nil { return "HEALTHY", nil }
	return status, err
}

type LogScannerTool struct {
	RDB *redis.Client
}
func (t *LogScannerTool) Name() string { return "LogScanner" }
func (t *LogScannerTool) Execute(podID string, _ string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	logs, err := t.RDB.Get(ctx, "sim:logs:"+podID).Result()
	if err == redis.Nil { return "CLEAN", nil }
	return logs, err
}

type ResourceTool struct {
	RDB *redis.Client
}
func (t *ResourceTool) Name() string { return "ResourceMonitor" }
func (t *ResourceTool) Execute(podID string, _ string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cpu, err := t.RDB.Get(ctx, "sim:cpu:"+podID).Result()
	if err == redis.Nil { return "20", nil }
	return cpu, err
}

type RemediationTool struct {
	RDB *redis.Client
}
func (t *RemediationTool) Name() string { return "Remediator" }
func (t *RemediationTool) Execute(podID string, action string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	switch action {
	case "REDEPLOY":
		t.RDB.Set(ctx, "sim:health:"+podID, "HEALTHY", 0)
		return "SUCCESS: Pod re-provisioned.", nil
	case "UPSCALE_MEM":
		t.RDB.Set(ctx, "sim:memory_limit:"+podID, "4Gi", 0)
		return "SUCCESS: Memory raised to 4Gi.", nil
	case "OPEN_CIRCUIT_BREAKER":
		t.RDB.Set(ctx, "sim:circuit_breaker:"+podID, "OPEN", 5*time.Minute)
		return "SUCCESS: Circuit Breaker OPENED.", nil
	case "RESTART_CPU":
		t.RDB.Set(ctx, "sim:cpu:"+podID, "20", 0)
		return "SUCCESS: CPU normalized to 20%.", nil
	default:
		return "NO_ACTION_TAKEN", nil
	}
}

type Investigator struct {
	Toolbox map[string]SRETool
}

func NewInvestigator(rdb *redis.Client) *Investigator {
	return &Investigator{
		Toolbox: map[string]SRETool{
			"health":    &HealthCheckTool{RDB: rdb},
			"logs":      &LogScannerTool{RDB: rdb},
			"cpu":       &ResourceTool{RDB: rdb},
			"remediate": &RemediationTool{RDB: rdb},
		},
	}
}

func (i *Investigator) Run(incidentID, podID string) ReActTrace {
	trace := ReActTrace{IncidentID: incidentID, PodID: podID, Steps: []ReActStep{}}

	// 1. Health
	health, _ := i.Toolbox["health"].Execute(podID, "")
	if health != "HEALTHY" {
		outcome, _ := i.Toolbox["remediate"].Execute(podID, "REDEPLOY")
		trace.Conclusion = "FIXED: Pod was " + health + ". " + outcome
		return trace
	}

	// 2. Logs
	logs, _ := i.Toolbox["logs"].Execute(podID, "")
	if logs != "CLEAN" {
		if strings.Contains(logs, "OOM") {
			outcome, _ := i.Toolbox["remediate"].Execute(podID, "UPSCALE_MEM")
			trace.Conclusion = "FIXED: OOM detected. " + outcome
		} else if strings.Contains(logs, "DB_TIMEOUT") {
			outcome, _ := i.Toolbox["remediate"].Execute(podID, "OPEN_CIRCUIT_BREAKER")
			trace.Conclusion = "FIXED: DB Timeout. " + outcome
		} else {
			trace.Conclusion = "ANALYZED: Logs show: " + logs
		}
		return trace
	}

	// 3. CPU
	cpuStr, _ := i.Toolbox["cpu"].Execute(podID, "")
	cpuVal, _ := strconv.Atoi(cpuStr)
	if cpuVal > 90 {
		outcome, _ := i.Toolbox["remediate"].Execute(podID, "RESTART_CPU")
		trace.Conclusion = fmt.Sprintf("FIXED: High CPU (%d). %s", cpuVal, outcome)
	} else {
		trace.Conclusion = fmt.Sprintf("HEALTHY: All metrics normal (CPU: %d%%).", cpuVal)
	}

	return trace
}