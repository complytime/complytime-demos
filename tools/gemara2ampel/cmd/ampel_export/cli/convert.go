package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gemara2ampel/go/ampel"

	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/fetcher"
)

// convertPolicy handles the main policy conversion logic
func convertPolicy(path string) error {
	// Calculate default output filename based on input YAML file
	defaultOutputFile := getDefaultOutputFilename(path)

	// Load the policy
	ctx := context.Background()
	f := &fetcher.URI{}
	policy, err := gemara.Load[gemara.Policy](ctx, f, path)
	if err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}

	// Prepare transformation options
	var transformOpts []ampel.TransformOption

	// Load catalog if provided
	if catalogPath != "" {
		catalog, err := gemara.Load[gemara.ControlCatalog](ctx, f, catalogPath)
		if err != nil {
			return fmt.Errorf("failed to load catalog: %w", err)
		}
		transformOpts = append(transformOpts, ampel.WithCatalog(catalog))
	}

	// Add scope filters option
	if scopeFilters {
		transformOpts = append(transformOpts, ampel.WithScopeFilters(true))
	}

	// Generate PolicySet or single Policy based on flag
	if policySet {
		return convertToPolicySet(policy, transformOpts, defaultOutputFile)
	}

	return convertToPolicy(policy, transformOpts, defaultOutputFile)
}

// convertToPolicySet generates a PolicySet
func convertToPolicySet(policy *gemara.Policy, transformOpts []ampel.TransformOption, defaultOutputFile string) error {
	// Set default output filename if not specified
	finalOutputFile := outputFile
	if finalOutputFile == "" {
		finalOutputFile = defaultOutputFile
	}

	// Generate PolicySet with imports
	var psOpts []ampel.PolicySetOption

	// Add PolicySet metadata if provided
	if policySetName != "" || policySetDesc != "" || policySetVersion != "" {
		psOpts = append(psOpts, ampel.WithPolicySetMetadata(policySetName, policySetDesc, policySetVersion))
	}

	// Add transform options
	if len(transformOpts) > 0 {
		psOpts = append(psOpts, ampel.WithTransformOptions(transformOpts...))
	}

	// Transform the policy to PolicySet
	ampelPolicySet, err := ampel.FromPolicyWithImports(policy, psOpts...)
	if err != nil {
		return fmt.Errorf("failed to transform policy to PolicySet: %w", err)
	}

	// Serialize to JSON
	ampelJSON, err := json.MarshalIndent(ampelPolicySet, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize PolicySet to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(finalOutputFile, ampelJSON, 0600); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Successfully wrote Ampel PolicySet to %s\n", finalOutputFile)
	fmt.Printf("PolicySet: %s\n", ampelPolicySet.Id)
	fmt.Printf("Policies: %d\n", len(ampelPolicySet.Policies))

	return nil
}

// convertToPolicy generates a single Ampel policy
func convertToPolicy(policy *gemara.Policy, transformOpts []ampel.TransformOption, defaultOutputFile string) error {
	// Transform the policy to single Ampel policy
	ampelPolicy, err := ampel.FromPolicy(policy, transformOpts...)
	if err != nil {
		return fmt.Errorf("failed to transform policy: %w", err)
	}

	// Check if workspace mode is enabled
	if workspacePath != "" {
		return handleWorkspaceMode(ampelPolicy, defaultOutputFile)
	}

	return handleStandardMode(ampelPolicy, defaultOutputFile)
}

// handleWorkspaceMode handles policy conversion in workspace mode
func handleWorkspaceMode(ampelPolicy *ampel.Policy, defaultOutputFile string) error {
	// Workspace mode
	ws, err := ampel.NewWorkspace(workspacePath)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Determine policy ID and output path
	policyID := ampelPolicy.Id

	// Determine output path and check existence
	var outputPath string
	var policyExists bool

	if outputFile != "" {
		// Custom output filename provided
		outputPath = outputFile
		if !filepath.IsAbs(outputPath) {
			outputPath = filepath.Join(workspacePath, outputFile)
		}
		// Check if custom output file exists
		_, err := os.Stat(outputPath)
		policyExists = err == nil
	} else {
		// Use default policy ID-based filename
		outputPath = ws.GetPolicyPath(policyID)
		policyExists = ws.PolicyExists(policyID)
	}

	if policyExists && !forceOverwrite {
		// Load existing policy from the output path
		data, err := os.ReadFile(outputPath)
		if err != nil {
			return fmt.Errorf("failed to read existing policy: %w", err)
		}

		var existingPolicy *ampel.Policy
		if err := json.Unmarshal(data, &existingPolicy); err != nil {
			return fmt.Errorf("failed to parse existing policy JSON (try --force-overwrite to regenerate): %w", err)
		}

		// Merge policies
		mergedPolicy, stats, err := ampel.MergePolicy(existingPolicy, ampelPolicy)
		if err != nil {
			return fmt.Errorf("failed to merge policies: %w", err)
		}

		// Save merged policy
		mergedJSON, err := json.MarshalIndent(mergedPolicy, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to serialize merged policy: %w", err)
		}
		if err := os.WriteFile(outputPath, mergedJSON, 0600); err != nil {
			return fmt.Errorf("failed to write merged policy: %w", err)
		}

		// Print update message with stats
		fmt.Printf("Updated existing Ampel policy: %s\n", outputPath)
		fmt.Printf("Policy: %s\n", mergedPolicy.Id)
		totalTenets := len(mergedPolicy.Tenets)
		fmt.Printf("Tenets: %d (%d preserved, %d added, %d removed)\n",
			totalTenets, stats.TenetsPreserved, stats.TenetsAdded, stats.TenetsRemoved)
		if stats.TenetsPreserved > 0 {
			fmt.Println("Preserved manual changes to CEL code and parameters")
		}
	} else {
		// Create new or force overwrite
		// Serialize to JSON
		ampelJSON, err := json.MarshalIndent(ampelPolicy, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to serialize policy to JSON: %w", err)
		}

		// Write to file
		if err := os.WriteFile(outputPath, ampelJSON, 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		if forceOverwrite {
			fmt.Printf("Regenerated Ampel policy: %s\n", outputPath)
			fmt.Println("Warning: Manual changes were discarded (--force-overwrite used)")
		} else {
			fmt.Printf("Created new Ampel policy: %s\n", outputPath)
		}
		fmt.Printf("Policy: %s\n", ampelPolicy.Id)
		fmt.Printf("Tenets: %d\n", len(ampelPolicy.Tenets))
	}

	return nil
}

// handleStandardMode handles policy conversion in standard (non-workspace) mode
func handleStandardMode(ampelPolicy *ampel.Policy, defaultOutputFile string) error {
	// Original behavior (no workspace)
	// Set default output filename if not specified
	finalOutputFile := outputFile
	if finalOutputFile == "" {
		finalOutputFile = defaultOutputFile
	}

	// Serialize to JSON
	ampelJSON, err := json.MarshalIndent(ampelPolicy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize policy to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(finalOutputFile, ampelJSON, 0600); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Successfully wrote Ampel policy to %s\n", finalOutputFile)
	fmt.Printf("Policy: %s\n", ampelPolicy.Id)
	fmt.Printf("Tenets: %d\n", len(ampelPolicy.Tenets))

	return nil
}

// getDefaultOutputFilename derives the default output filename from the input YAML path.
// Example: "test_data/ampel-test-policy.yaml" -> "ampel-test-policy.json"
func getDefaultOutputFilename(yamlPath string) string {
	// Get the base filename without directory
	base := filepath.Base(yamlPath)

	// Remove extension and add .json
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}

	return base + ".json"
}
