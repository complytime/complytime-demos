package export

import (
	"flag"
	"fmt"
	"os"

	"github.com/ossf/gemara"
	"gemara2ampel/go/ampel"
)

// Policy converts a Gemara Layer-3 policy to Ampel format and writes it to a file.
func Policy(path string, args []string) error {
	cmd := flag.NewFlagSet("policy", flag.ExitOnError)
	outputFile := cmd.String("output", "ampel-policy.json", "Path to output file")
	catalogPath := cmd.String("catalog", "", "Path to catalog file for enriching policy details")
	scopeFilters := cmd.Bool("scope-filters", false, "Include scope-based CEL filters in tenets")
	policySet := cmd.Bool("policyset", false, "Generate a PolicySet with imports as external references")
	policySetName := cmd.String("policyset-name", "", "Name for the PolicySet (only used with -policyset)")
	policySetDesc := cmd.String("policyset-description", "", "Description for the PolicySet (only used with -policyset)")
	policySetVersion := cmd.String("policyset-version", "", "Version for the PolicySet (only used with -policyset)")

	if err := cmd.Parse(args); err != nil {
		return err
	}

	// Load the policy
	policy := &gemara.Policy{}
	pathWithScheme := fmt.Sprintf("file://%s", path)
	if err := policy.LoadFile(pathWithScheme); err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}

	// Prepare transformation options
	var transformOpts []ampel.TransformOption

	// Load catalog if provided
	if *catalogPath != "" {
		catalog := &gemara.Catalog{}
		catalogPathWithScheme := fmt.Sprintf("file://%s", *catalogPath)
		if err := catalog.LoadFile(catalogPathWithScheme); err != nil {
			return fmt.Errorf("failed to load catalog: %w", err)
		}
		transformOpts = append(transformOpts, ampel.WithCatalog(catalog))
	}

	// Add scope filters option
	if *scopeFilters {
		transformOpts = append(transformOpts, ampel.WithScopeFilters(true))
	}

	// Generate PolicySet or single Policy based on flag
	if *policySet {
		// Generate PolicySet with imports
		var psOpts []ampel.PolicySetOption

		// Add PolicySet metadata if provided
		if *policySetName != "" || *policySetDesc != "" || *policySetVersion != "" {
			psOpts = append(psOpts, ampel.WithPolicySetMetadata(*policySetName, *policySetDesc, *policySetVersion))
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
		ampelJSON, err := ampelPolicySet.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize PolicySet to JSON: %w", err)
		}

		// Write to file
		if err := os.WriteFile(*outputFile, ampelJSON, 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Printf("Successfully wrote Ampel PolicySet to %s\n", *outputFile)
		fmt.Printf("PolicySet: %s\n", ampelPolicySet.Name)
		fmt.Printf("Policies: %d\n", len(ampelPolicySet.Policies))
	} else {
		// Transform the policy to single Ampel policy
		ampelPolicy, err := ampel.FromPolicy(policy, transformOpts...)
		if err != nil {
			return fmt.Errorf("failed to transform policy: %w", err)
		}

		// Serialize to JSON
		ampelJSON, err := ampelPolicy.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize policy to JSON: %w", err)
		}

		// Write to file
		if err := os.WriteFile(*outputFile, ampelJSON, 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Printf("Successfully wrote Ampel policy to %s\n", *outputFile)
		fmt.Printf("Policy: %s\n", ampelPolicy.Name)
		fmt.Printf("Tenets: %d\n", len(ampelPolicy.Tenets))
	}

	return nil
}
