package runner

import (
	"fmt"
	"strings"

	"github.com/gregberns/adze/internal/step"
)

// FormatSummary produces the run summary text from a RunResult.
//
// Format:
//
//	=== Run Summary ===
//
//	Steps: <total> total
//	  ✓ <N> succeeded (<applied> applied, <satisfied> already satisfied)
//	  ✗ <N> failed
//	  - <N> skipped
//
//	Failed:
//	  <step-name> (partial: <succeeded>/<total> items)
//	    ✗ <item> — <error message>
//	      Log: <log-path>
//
//	Skipped:
//	  <step-name> — <skip reason>
//
//	Re-run: adze apply (completed steps skip automatically)
func FormatSummary(result *RunResult) string {
	var b strings.Builder

	// Count statuses.
	var applied, satisfied, failed, skipped int
	for _, sr := range result.StepResults {
		switch sr.Status {
		case step.StatusApplied:
			applied++
		case step.StatusSatisfied:
			satisfied++
		case step.StatusFailed, step.StatusVerifyFailed:
			failed++
		case step.StatusPartial:
			failed++ // partial counts as failed in the summary
		case step.StatusSkipped:
			skipped++
		}
	}
	succeeded := applied + satisfied
	total := len(result.StepResults)

	b.WriteString("=== Run Summary ===\n\n")
	b.WriteString(fmt.Sprintf("Steps: %d total\n", total))
	b.WriteString(fmt.Sprintf("  \u2713 %d succeeded (%d applied, %d already satisfied)\n", succeeded, applied, satisfied))
	b.WriteString(fmt.Sprintf("  \u2717 %d failed\n", failed))
	b.WriteString(fmt.Sprintf("  - %d skipped\n", skipped))

	// Failed section.
	if failed > 0 {
		b.WriteString("\nFailed:\n")
		for _, sr := range result.StepResults {
			switch sr.Status {
			case step.StatusFailed, step.StatusVerifyFailed:
				b.WriteString(fmt.Sprintf("  %s\n", sr.Name))
				if sr.Reason != "" {
					b.WriteString(fmt.Sprintf("    \u2717 %s\n", sr.Reason))
				}
				if sr.LogPath != "" {
					b.WriteString(fmt.Sprintf("      Log: %s\n", sr.LogPath))
				}
			case step.StatusPartial:
				// Count succeeded and total items for the partial line.
				succeededItems := 0
				totalItems := len(sr.Items)
				for _, ir := range sr.Items {
					if ir.Status == step.StatusApplied || ir.Status == step.StatusSatisfied {
						succeededItems++
					}
				}
				b.WriteString(fmt.Sprintf("  %s (partial: %d/%d items)\n", sr.Name, succeededItems, totalItems))
				for _, ir := range sr.Items {
					if ir.Status == step.StatusFailed {
						b.WriteString(fmt.Sprintf("    \u2717 %s \u2014 %s\n", ir.Item.Name, ir.Reason))
						// Log path for batch items.
						logPath := logPathForItem(sr.Name, ir.Item.Name, result)
						if logPath != "" {
							b.WriteString(fmt.Sprintf("      Log: %s\n", logPath))
						}
					}
				}
			}
		}
	}

	// Skipped section.
	if skipped > 0 {
		b.WriteString("\nSkipped:\n")
		for _, sr := range result.StepResults {
			if sr.Status == step.StatusSkipped {
				b.WriteString(fmt.Sprintf("  %s \u2014 %s\n", sr.Name, sr.Reason))
			}
		}
	}

	// Re-run line only when there are failures.
	if failed > 0 {
		b.WriteString("\nRe-run: adze apply (completed steps skip automatically)\n")
	}

	return b.String()
}

// logPathForItem returns the expected log path for a failed batch item.
// It reconstructs the path since the runner stores it in the step-level LogPath field.
func logPathForItem(stepName, itemName string, result *RunResult) string {
	// The runner writes batch item logs as <step-name>-<item>.log.
	// We look for the step's logDir from the step-level result, but since we
	// don't store per-item log paths in the result struct, we reconstruct it
	// from the step's LogPath (which is the last failed item's path).
	for _, sr := range result.StepResults {
		if sr.Name == stepName && sr.LogPath != "" {
			// Extract the directory from the step's log path.
			// The log path format is: <logDir>/<step-name>-<item>.log
			// We reconstruct the expected path for this specific item.
			dir := sr.LogPath[:strings.LastIndex(sr.LogPath, "/")]
			return fmt.Sprintf("%s/%s-%s.log", dir, stepName, itemName)
		}
	}
	return ""
}
