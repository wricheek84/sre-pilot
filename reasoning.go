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
	MTTR_ms    int
}

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
	if err == redis.Nil {
		return "HEALTHY", nil
	}
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
	if err == redis.Nil {
		return "CLEAN", nil
	}
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
	if err == redis.Nil {
		return "20", nil
	}
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
	startTime := time.Now()
	trace := ReActTrace{IncidentID: incidentID, PodID: podID, Steps: []ReActStep{}}

	health, _ := i.Toolbox["health"].Execute(podID, "")

	trace.Steps = append(trace.Steps, ReActStep{
		StepType:  "OBSERVATION",
		Content:   "Health Check result: " + health,
		Timestamp: time.Now(),
	})
	if health != "HEALTHY" {
		trace.Steps = append(trace.Steps, ReActStep{
			StepType:  "THOUGHT",
			Content:   fmt.Sprintf("Pod %s reported status %s. A non-healthy status requires an immediate fresh start to clear potential deadlocks or corrupted state.", podID, health),
			Timestamp: time.Now(),
		})
		trace.Steps = append(trace.Steps, ReActStep{
			StepType:  "ACTION",
			Content:   "Pod unhealthy. Executing REDEPLOY on " + podID,
			Timestamp: time.Now(),
		})
		outcome, _ := i.Toolbox["remediate"].Execute(podID, "REDEPLOY")
		trace.Conclusion = "FIXED: Pod was " + health + ". " + outcome
		trace.MTTR_ms = int(time.Since(startTime).Milliseconds())
		return trace

	}

	// 2. Logs
	logs, _ := i.Toolbox["logs"].Execute(podID, "")
	trace.Steps = append(trace.Steps, ReActStep{
		StepType:  "OBSERVATION",
		Content:   "Log analysis result: " + logs,
		Timestamp: time.Now(),
	})
	if logs != "CLEAN" {
		var action string
		if strings.Contains(logs, "OOM") {
			action = "UPSCALE_MEM"
		} else if strings.Contains(logs, "DB_TIMEOUT") {
			action = "OPEN_CIRCUIT_BREAKER"
		}
		if action != "" {
			trace.Steps = append(trace.Steps, ReActStep{
				StepType:  "THOUGHT",
				Content:   "Logs indicate a specific fault (" + logs + "). This suggests a targeted remediation rather than a full redeploy.",
				Timestamp: time.Now(),
			})
			trace.Steps = append(trace.Steps, ReActStep{
				StepType:  "ACTION",
				Content:   "Detected specific fault. Executing " + action,
				Timestamp: time.Now(),
			})
			outcome, _ := i.Toolbox["remediate"].Execute(podID, action)
			trace.Conclusion = "FIXED: " + logs + ". " + outcome

		} else {
			trace.Conclusion = "ANALYZED: Logs show: " + logs
		}
		trace.MTTR_ms = int(time.Since(startTime).Milliseconds())

		return trace

	}

	// 3. CPU
	cpuStr, _ := i.Toolbox["cpu"].Execute(podID, "")
	cpuVal, _ := strconv.Atoi(cpuStr)
	trace.Steps = append(trace.Steps, ReActStep{
		StepType:  "OBSERVATION",
		Content:   fmt.Sprintf("CPU Monitor: %d%% utilization", cpuVal),
		Timestamp: time.Now(),
	})

	if cpuVal > 90 {
		trace.Steps = append(trace.Steps, ReActStep{
			StepType:  "THOUGHT",
			Content:   fmt.Sprintf("CPU is at %d%%, exceeding the 90%% safety threshold.need to restart the process to clear dead threads or runaway tasks.", cpuVal),
			Timestamp: time.Now(),
		})
		trace.Steps = append(trace.Steps, ReActStep{
			StepType:  "ACTION",
			Content:   "CPU threshold exceeded. Executing RESTART_CPU",
			Timestamp: time.Now(),
		})
		outcome, _ := i.Toolbox["remediate"].Execute(podID, "RESTART_CPU")
		trace.Conclusion = fmt.Sprintf("FIXED: High CPU (%d). %s", cpuVal, outcome)

	} else {
		trace.Conclusion = fmt.Sprintf("ANALYZED: CPU at %d%%, within normal limits.", cpuVal)
	}
	trace.MTTR_ms = int(time.Since(startTime).Milliseconds())

	return trace
}
