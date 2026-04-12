package step

import (
	"context"
	"fmt"
	"log"
	"time"
)

// EnvChecker is a callback the executor uses to check env var availability.
type EnvChecker func(name string) (value string, required bool, present bool)

// ExecuteStep runs the full step lifecycle (pre-flight -> check -> apply -> verify).
//
// The lifecycle is:
//  1. PRE-FLIGHT: Check env vars via envChecker. Missing required -> StatusSkipped.
//  2. CHECK: Run Check with CheckTimeout (default 5 min).
//     exit 0 -> StatusSatisfied, STOP.
//     non-zero -> unsatisfied, continue to APPLY.
//     timeout -> treated as unsatisfied (log warning), continue.
//     nil Check command -> always unsatisfied.
//     infra error -> return error, STOP.
//  3. APPLY: Run Apply with ApplyTimeout (default 15 min).
//     exit 0 -> continue to VERIFY.
//     non-zero -> StatusFailed, STOP.
//     timeout -> StatusFailed with "timed out" reason, STOP.
//     no apply command -> StatusSkipped with "no apply command for platform", STOP.
//     infra error -> return error, STOP.
//  4. VERIFY: Re-run Check with CheckTimeout.
//     exit 0 -> StatusApplied, STOP.
//     non-zero -> StatusVerifyFailed, STOP.
//     nil Check -> StatusVerifyFailed.
//     timeout -> StatusVerifyFailed with "verify timed out", STOP.
//     infra error -> return error, STOP.
func ExecuteStep(ctx context.Context, s Step, cfg StepConfig, platform string, envChecker EnvChecker) (StepResult, error) {
	start := time.Now()

	checkTimeout := cfg.CheckTimeout
	if checkTimeout == 0 {
		checkTimeout = DefaultCheckTimeout
	}
	applyTimeout := cfg.ApplyTimeout
	if applyTimeout == 0 {
		applyTimeout = DefaultApplyTimeout
	}

	// --- PRE-FLIGHT: check env vars ---
	if envChecker != nil {
		for _, envName := range cfg.Env {
			_, required, present := envChecker(envName)
			if required && !present {
				return StepResult{
					Status:   StatusSkipped,
					Reason:   fmt.Sprintf("missing required env var: %s", envName),
					Duration: time.Since(start),
				}, nil
			}
		}
	}

	// --- CHECK ---
	checkResult, err := runCheck(ctx, s, cfg, checkTimeout)
	if err != nil {
		return StepResult{Duration: time.Since(start)}, err
	}
	if checkResult.Status == StatusSatisfied {
		checkResult.Duration = time.Since(start)
		return checkResult, nil
	}
	// Check unsatisfied (non-zero, timeout, nil check) — continue to APPLY.

	// --- APPLY ---
	// Platform dispatch: resolve the apply command.
	applyCfg := cfg
	resolvedApply := resolveApplyCommand(cfg, platform)
	if resolvedApply == nil {
		return StepResult{
			Status:   StatusSkipped,
			Reason:   fmt.Sprintf("no apply command for platform %s", platform),
			Duration: time.Since(start),
		}, nil
	}
	applyCfg.Apply = resolvedApply

	applyCtx, applyCancel := context.WithTimeout(ctx, applyTimeout)
	defer applyCancel()

	applyResult, err := s.Apply(applyCtx, applyCfg)
	if err != nil {
		return StepResult{Duration: time.Since(start)}, err
	}

	// Check if apply timed out (context deadline exceeded).
	if applyCtx.Err() != nil {
		return StepResult{
			Status:   StatusFailed,
			Reason:   fmt.Sprintf("timed out after %s", applyTimeout),
			Duration: time.Since(start),
		}, nil
	}

	if applyResult.Status == StatusFailed {
		applyResult.Duration = time.Since(start)
		return applyResult, nil
	}
	if applyResult.Status == StatusSkipped {
		applyResult.Duration = time.Since(start)
		return applyResult, nil
	}

	// --- VERIFY ---
	verifyResult, err := runVerify(ctx, s, cfg, checkTimeout)
	if err != nil {
		return StepResult{Duration: time.Since(start)}, err
	}
	verifyResult.Duration = time.Since(start)
	return verifyResult, nil
}

// resolveApplyCommand implements platform dispatch for apply commands.
// Resolution order:
//  1. PlatformApply[platform] if present
//  2. Apply if non-nil
//  3. nil (caller handles StatusSkipped)
func resolveApplyCommand(cfg StepConfig, platform string) *ShellCommand {
	if cfg.PlatformApply != nil {
		if cmd, ok := cfg.PlatformApply[platform]; ok {
			return cmd
		}
	}
	return cfg.Apply
}

// runCheck executes the check phase with timeout handling.
func runCheck(ctx context.Context, s Step, cfg StepConfig, timeout time.Duration) (StepResult, error) {
	checkCtx, checkCancel := context.WithTimeout(ctx, timeout)
	defer checkCancel()

	result, err := s.Check(checkCtx, cfg)
	if err != nil {
		return StepResult{}, err
	}

	// If check timed out, treat as unsatisfied (not as a failure): log warning, continue to apply.
	if checkCtx.Err() != nil {
		log.Printf("[step:%s][check] timed out after %s, treating as unsatisfied", cfg.Name, timeout)
		return StepResult{
			Reason: "check timed out",
		}, nil
	}

	return result, nil
}

// runVerify re-runs the check command after apply to confirm desired state.
func runVerify(ctx context.Context, s Step, cfg StepConfig, timeout time.Duration) (StepResult, error) {
	if cfg.Check == nil {
		return StepResult{
			Status: StatusVerifyFailed,
			Reason: "no check command for verify",
		}, nil
	}

	verifyCtx, verifyCancel := context.WithTimeout(ctx, timeout)
	defer verifyCancel()

	result, err := s.Check(verifyCtx, cfg)
	if err != nil {
		return StepResult{}, err
	}

	// If verify timed out.
	if verifyCtx.Err() != nil {
		return StepResult{
			Status: StatusVerifyFailed,
			Reason: "verify timed out",
		}, nil
	}

	if result.Status == StatusSatisfied {
		return StepResult{
			Status: StatusApplied,
		}, nil
	}

	return StepResult{
		Status: StatusVerifyFailed,
		Reason: fmt.Sprintf("verify check failed: %s", result.Reason),
	}, nil
}

// ExecuteBatchStep runs the step lifecycle for each item in a batch.
// For each item:
//  1. Check(item) -> exit 0 = ItemResult{satisfied}
//  2. Apply(item) -> non-zero = ItemResult{failed}; continue to next
//  3. Verify(item) -> exit 0 = ItemResult{applied}; non-zero = ItemResult{failed, "verify failed"}
//
// Aggregate:
//   - Empty items -> satisfied
//   - All satisfied -> satisfied
//   - All applied/satisfied, none failed -> applied
//   - Mix with at least one failed and one success -> partial
//   - All failed -> failed
//
// A failed item does NOT halt iteration.
func ExecuteBatchStep(ctx context.Context, s Step, cfg StepConfig, platform string, envChecker EnvChecker) (StepResult, error) {
	start := time.Now()

	checkTimeout := cfg.CheckTimeout
	if checkTimeout == 0 {
		checkTimeout = DefaultCheckTimeout
	}
	applyTimeout := cfg.ApplyTimeout
	if applyTimeout == 0 {
		applyTimeout = DefaultApplyTimeout
	}

	// PRE-FLIGHT
	if envChecker != nil {
		for _, envName := range cfg.Env {
			_, required, present := envChecker(envName)
			if required && !present {
				return StepResult{
					Status:   StatusSkipped,
					Reason:   fmt.Sprintf("missing required env var: %s", envName),
					Duration: time.Since(start),
				}, nil
			}
		}
	}

	if len(cfg.Items) == 0 {
		return StepResult{
			Status:   StatusSatisfied,
			Duration: time.Since(start),
		}, nil
	}

	// Resolve apply command for platform.
	resolvedApply := resolveApplyCommand(cfg, platform)

	var itemResults []ItemResult

	for _, item := range cfg.Items {
		// Build per-item config: the check/apply commands may use item info
		// via environment variables.
		itemCfg := cfg
		itemCfg.Name = fmt.Sprintf("%s[%s]", cfg.Name, item.Name)

		// Add item info as environment variables for commands.
		itemEnv := []string{
			fmt.Sprintf("ADZE_ITEM_NAME=%s", item.Name),
			fmt.Sprintf("ADZE_ITEM_VERSION=%s", item.Version),
		}
		itemCfg.Env = append(append([]string{}, cfg.Env...), itemEnv...)

		// CHECK
		checkResult, err := runCheck(ctx, s, itemCfg, checkTimeout)
		if err != nil {
			return StepResult{Duration: time.Since(start)}, err
		}
		if checkResult.Status == StatusSatisfied {
			itemResults = append(itemResults, ItemResult{
				Item:   item,
				Status: StatusSatisfied,
			})
			continue
		}

		// APPLY
		if resolvedApply == nil {
			itemResults = append(itemResults, ItemResult{
				Item:   item,
				Status: StatusFailed,
				Reason: fmt.Sprintf("no apply command for platform %s", platform),
			})
			continue
		}

		itemCfg.Apply = resolvedApply
		applyCtx, applyCancel := context.WithTimeout(ctx, applyTimeout)

		applyResult, err := s.Apply(applyCtx, itemCfg)
		applyTimedOut := applyCtx.Err() != nil
		applyCancel()
		if err != nil {
			return StepResult{Duration: time.Since(start)}, err
		}

		if applyTimedOut || applyResult.Status == StatusFailed {
			reason := applyResult.Reason
			if applyTimedOut {
				reason = fmt.Sprintf("timed out after %s", applyTimeout)
			}
			itemResults = append(itemResults, ItemResult{
				Item:   item,
				Status: StatusFailed,
				Reason: reason,
			})
			continue
		}

		// VERIFY
		verifyResult, err := runVerify(ctx, s, itemCfg, checkTimeout)
		if err != nil {
			return StepResult{Duration: time.Since(start)}, err
		}

		if verifyResult.Status == StatusApplied {
			itemResults = append(itemResults, ItemResult{
				Item:   item,
				Status: StatusApplied,
			})
		} else {
			itemResults = append(itemResults, ItemResult{
				Item:   item,
				Status: StatusFailed,
				Reason: "verify failed",
			})
		}
	}

	return StepResult{
		Status:      aggregateItemResults(itemResults),
		ItemResults: itemResults,
		Duration:    time.Since(start),
	}, nil
}

// aggregateItemResults computes the overall status from item results.
func aggregateItemResults(results []ItemResult) StepStatus {
	if len(results) == 0 {
		return StatusSatisfied
	}

	var hasApplied, hasFailed, hasSatisfied bool
	for _, r := range results {
		switch r.Status {
		case StatusSatisfied:
			hasSatisfied = true
		case StatusApplied:
			hasApplied = true
		case StatusFailed:
			hasFailed = true
		}
	}

	switch {
	case !hasApplied && !hasFailed && hasSatisfied:
		// All satisfied.
		return StatusSatisfied
	case !hasFailed && (hasApplied || hasSatisfied):
		// All applied/satisfied, none failed.
		return StatusApplied
	case hasFailed && (hasApplied || hasSatisfied):
		// Mix with at least one failed and one success.
		return StatusPartial
	default:
		// All failed.
		return StatusFailed
	}
}
