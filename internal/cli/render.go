package cli

import (
	"fmt"
	"os"

	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/render"
	"github.com/gregberns/adze/internal/step"
	"github.com/gregberns/adze/internal/steps"
	"github.com/spf13/cobra"
)

func newRenderCmdImpl() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Generate a standalone bash script from config",
		Long:  "Generate a standalone bash script from the resolved config. The script can be run independently without adze.",
		RunE:  runRender,
	}
	cmd.Flags().StringP("output", "o", "", "write to file (default: stdout)")
	return cmd
}

func runRender(cmd *cobra.Command, args []string) error {
	gf := ResolveGlobalFlags(cmd)
	outputPath, _ := cmd.Flags().GetString("output")

	configPath, err := ResolveConfig(gf.Config)
	if err != nil {
		return err
	}

	cfg, valErrs, _, loadErr := config.LoadConfig(configPath, false)
	if loadErr != nil {
		return fmt.Errorf("loading config: %w", loadErr)
	}
	if len(valErrs) > 0 || cfg == nil {
		return fmt.Errorf("config has validation errors; run 'adze validate' for details")
	}

	platform := resolvePlatform(cfg.Platform)
	reg := steps.NewRegistry()
	stepConfigs := steps.BuildStepConfigs(cfg, platform, reg)

	var dagInputs []dag.StepInput
	for _, sc := range stepConfigs {
		dagInputs = append(dagInputs, dag.StepInput{
			Name:      sc.Name,
			Provides:  sc.Provides,
			Requires:  sc.Requires,
			Platforms: sc.Platforms,
			BuiltIn:   !isCustomStep(sc.Name, cfg),
		})
	}

	knownBuiltIns := buildKnownBuiltIns(reg)
	graph, dagErrs := dag.Resolve(dagInputs, platform, knownBuiltIns)
	if len(dagErrs) > 0 {
		return fmt.Errorf("dependency resolution failed; run 'adze validate' for details")
	}

	// Convert step configs to the form render expects
	allStepConfigs := make([]step.StepConfig, len(stepConfigs))
	copy(allStepConfigs, stepConfigs)

	script, err := render.Render(cfg, graph, allStepConfigs, platform, configPath)
	if err != nil {
		return fmt.Errorf("rendering script: %w", err)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(script), 0755); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", outputPath)
		return nil
	}

	fmt.Fprint(cmd.OutOrStdout(), script)
	return nil
}
