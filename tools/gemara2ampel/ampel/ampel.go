package ampel

import (
	"fmt"
	"path"
	"strings"

	"github.com/ossf/gemara"
)

// FromPolicy converts a Gemara Layer-3 Policy to Ampel policy format.
//
// The function maps:
//   - Policy metadata to Ampel policy name/description/version
//   - Assessment plans to Ampel tenets with CEL expressions
//   - Evaluation methods (type: automated) to attestation verification logic
//   - Evidence requirements to expected attestation predicates
//   - Scope dimensions to CEL filtering expressions
//
// Options:
//   - WithCatalog: Include catalog data to enrich tenet descriptions
//   - WithCELTemplates: Custom CEL code templates for method types
//   - WithAttestationTypes: Specify expected attestation types
//   - WithScopeFilters: Generate scope-based CEL filters
//   - WithDefaultRule: Set overall policy rule (default: "all(tenets)")
func FromPolicy(policy *gemara.Policy, opts ...TransformOption) (AmpelPolicy, error) {
	options := &TransformOptions{}
	for _, opt := range opts {
		opt(options)
	}
	options.applyDefaults()

	ampelPolicy := AmpelPolicy{
		Name:     policy.Title,
		Metadata: make(map[string]string),
		Tenets:   []Tenet{},
		Rule:     options.DefaultRule,
	}

	// Transform metadata
	if err := buildMetadata(policy, &ampelPolicy); err != nil {
		return ampelPolicy, fmt.Errorf("error building metadata: %w", err)
	}

	// Process imports
	if err := processImports(&policy.Imports, &ampelPolicy, options); err != nil {
		return ampelPolicy, fmt.Errorf("error processing imports: %w", err)
	}

	// Convert assessment plans to tenets
	for _, plan := range policy.Adherence.AssessmentPlans {
		tenets, err := assessmentPlanToTenets(plan, policy, options)
		if err != nil {
			return ampelPolicy, fmt.Errorf("error converting assessment plan %s: %w", plan.Id, err)
		}
		ampelPolicy.Tenets = append(ampelPolicy.Tenets, tenets...)
	}

	// Validate the generated policy
	if err := ampelPolicy.Validate(); err != nil {
		return ampelPolicy, fmt.Errorf("generated policy validation failed: %w", err)
	}

	return ampelPolicy, nil
}

// buildMetadata extracts metadata from Gemara policy and populates Ampel policy metadata.
func buildMetadata(policy *gemara.Policy, ampelPolicy *AmpelPolicy) error {
	// Set version and description
	ampelPolicy.Version = policy.Metadata.Version
	ampelPolicy.Description = policy.Metadata.Description

	// Add policy ID to metadata
	ampelPolicy.Metadata["policy-id"] = policy.Metadata.Id

	// Add author information
	ampelPolicy.Metadata["author"] = policy.Metadata.Author.Name
	if policy.Metadata.Author.Id != "" {
		ampelPolicy.Metadata["author-id"] = policy.Metadata.Author.Id
	}

	// Add RACI contacts to metadata
	if len(policy.Contacts.Responsible) > 0 {
		var responsible []string
		for _, contact := range policy.Contacts.Responsible {
			responsible = append(responsible, contact.Name)
		}
		ampelPolicy.Metadata["responsible"] = strings.Join(responsible, ", ")
	}

	if len(policy.Contacts.Accountable) > 0 {
		var accountable []string
		for _, contact := range policy.Contacts.Accountable {
			accountable = append(accountable, contact.Name)
		}
		ampelPolicy.Metadata["accountable"] = strings.Join(accountable, ", ")
	}

	// Add scope summary to metadata
	if len(policy.Scope.In.Technologies) > 0 {
		ampelPolicy.Metadata["scope-technologies"] = strings.Join(policy.Scope.In.Technologies, ", ")
	}
	if len(policy.Scope.In.Geopolitical) > 0 {
		ampelPolicy.Metadata["scope-regions"] = strings.Join(policy.Scope.In.Geopolitical, ", ")
	}

	return nil
}

// processImports converts Gemara imports to Ampel policy imports.
func processImports(imports *gemara.Imports, ampelPolicy *AmpelPolicy, options *TransformOptions) error {
	if len(imports.Policies) > 0 {
		ampelPolicy.Imports = append(ampelPolicy.Imports, imports.Policies...)
	}

	// Add catalog and guidance references as metadata
	if len(imports.Catalogs) > 0 {
		var catalogRefs []string
		for _, catalog := range imports.Catalogs {
			catalogRefs = append(catalogRefs, catalog.ReferenceId)
		}
		ampelPolicy.Metadata["catalog-references"] = strings.Join(catalogRefs, ", ")
	}

	if len(imports.Guidance) > 0 {
		var guidanceRefs []string
		for _, guidance := range imports.Guidance {
			guidanceRefs = append(guidanceRefs, guidance.ReferenceId)
		}
		ampelPolicy.Metadata["guidance-references"] = strings.Join(guidanceRefs, ", ")
	}

	return nil
}

// assessmentPlanToTenets converts a single assessment plan to one or more Ampel tenets.
func assessmentPlanToTenets(
	plan gemara.AssessmentPlan,
	policy *gemara.Policy,
	options *TransformOptions,
) ([]Tenet, error) {
	var tenets []Tenet

	// Get evidence requirements
	evidenceReq := plan.EvidenceRequirements

	// Get requirement details from catalog if available
	requirementText := ""
	if options.Catalog != nil {
		requirementText = lookupRequirementText(plan.RequirementId, options.Catalog)
	}

	// Process each evaluation method
	methodIndex := 0
	for _, method := range plan.EvaluationMethods {
		// Only process automated methods
		if !isAutomatedMethod(method.Type) {
			continue
		}

		// Build tenet parameters from plan parameters
		tenetParams := make(map[string]interface{})
		celParams := make(map[string]interface{})

		for _, param := range plan.Parameters {
			if len(param.AcceptedValues) > 0 {
				// Use first accepted value as default
				tenetParams[param.Id] = param.AcceptedValues[0]
				celParams[param.Id] = param.AcceptedValues[0]

				// For multiple accepted values, format as list for CEL
				if len(param.AcceptedValues) > 1 {
					var quotedValues []string
					for _, val := range param.AcceptedValues {
						quotedValues = append(quotedValues, fmt.Sprintf(`"%s"`, val))
					}
					celParams[param.Id+"s"] = strings.Join(quotedValues, ", ")
				}
			}
		}

		// Generate CEL expression
		celCode, attestationTypes, err := GenerateCELFromMethod(method, evidenceReq, celParams, options.CELTemplates)
		if err != nil {
			return nil, fmt.Errorf("error generating CEL for method %d: %w", methodIndex, err)
		}

		// Apply scope filters if enabled
		if options.IncludeScopeFilters {
			scopeFilter := ScopeFilterToCEL(policy.Scope.In)
			if scopeFilter != "" {
				celCode = fmt.Sprintf("(%s) && (%s)", scopeFilter, celCode)
			}
		}

		// Build tenet description
		description := evidenceReq
		if requirementText != "" && evidenceReq != "" {
			description = requirementText + " - " + evidenceReq
		} else if requirementText != "" {
			description = requirementText
		}

		// Create tenet
		tenet := Tenet{
			ID:               fmt.Sprintf("%s-%s-%d", plan.RequirementId, plan.Id, methodIndex),
			Name:             getTenetName(method, evidenceReq),
			Description:      description,
			Code:             celCode,
			AttestationTypes: attestationTypes,
			Parameters:       tenetParams,
		}

		tenets = append(tenets, tenet)
		methodIndex++
	}

	return tenets, nil
}

// getTenetName determines an appropriate name for a tenet based on the method and evidence.
func getTenetName(method gemara.AcceptedMethod, evidenceReq string) string {
	if method.Description != "" {
		return method.Description
	}

	// Generate name from evidence requirement
	if evidenceReq != "" {
		// Capitalize first letter and limit length
		name := strings.TrimSpace(evidenceReq)
		if len(name) > 80 {
			name = name[:77] + "..."
		}
		return name
	}

	// Fallback to method type
	return fmt.Sprintf("%s verification", method.Type)
}

// isAutomatedMethod checks if an evaluation method type can be automated.
func isAutomatedMethod(methodType string) bool {
	automatedTypes := map[string]bool{
		"automated":        true,
		"gate":             true,
		"behavioral":       true,
		"autoremediation": true,
	}
	return automatedTypes[methodType]
}

// lookupRequirementText finds the requirement text from a catalog.
func lookupRequirementText(requirementId string, catalog *gemara.Catalog) string {
	// Search through all control families and controls in the catalog
	for _, family := range catalog.ControlFamilies {
		for _, control := range family.Controls {
			for _, req := range control.AssessmentRequirements {
				if req.Id == requirementId {
					return req.Text
				}
			}
		}
	}
	return ""
}

// FromPolicies converts multiple Gemara Layer-3 Policies to an Ampel PolicySet.
//
// This function creates a PolicySet with inline policies from the provided Gemara policies.
// Each policy is transformed using FromPolicy and embedded directly in the PolicySet.
//
// Options:
//   - WithPolicySetMetadata: Set name, description, and version for the PolicySet
//   - WithCatalog: Include catalog data to enrich tenet descriptions
//   - WithCELTemplates: Custom CEL code templates for method types
//   - WithAttestationTypes: Specify expected attestation types
//   - WithScopeFilters: Generate scope-based CEL filters
//   - WithDefaultRule: Set overall policy rule (default: "all(tenets)")
func FromPolicies(policies []*gemara.Policy, opts ...PolicySetOption) (PolicySet, error) {
	if len(policies) == 0 {
		return PolicySet{}, fmt.Errorf("at least one policy is required")
	}

	policySet := PolicySet{
		Policies: []PolicyReference{},
		Metadata: make(map[string]string),
	}

	// Apply policy set options
	psOptions := &PolicySetOptions{}
	for _, opt := range opts {
		opt(psOptions)
	}

	// Set PolicySet metadata
	policySet.Name = psOptions.Name
	policySet.Description = psOptions.Description
	policySet.Version = psOptions.Version
	if psOptions.Metadata != nil {
		policySet.Metadata = psOptions.Metadata
	}

	// Convert each Gemara policy to Ampel policy
	for _, gemaraPolicy := range policies {
		ampelPolicy, err := FromPolicy(gemaraPolicy, psOptions.TransformOptions...)
		if err != nil {
			return policySet, fmt.Errorf("error converting policy %s: %w", gemaraPolicy.Metadata.Id, err)
		}

		// Create inline policy reference
		policyRef := PolicyReference{
			ID:     gemaraPolicy.Metadata.Id,
			Policy: &ampelPolicy,
		}

		// Add metadata if provided
		if psOptions.PolicyMetadata != nil {
			if meta, ok := psOptions.PolicyMetadata[gemaraPolicy.Metadata.Id]; ok {
				policyRef.Meta = meta
			}
		}

		policySet.Policies = append(policySet.Policies, policyRef)
	}

	// Validate the generated policy set
	if err := policySet.Validate(); err != nil {
		return policySet, fmt.Errorf("generated policy set validation failed: %w", err)
	}

	return policySet, nil
}

// FromPolicyWithImports converts a Gemara Layer-3 Policy and its imports to an Ampel PolicySet.
//
// This function creates a PolicySet where:
//   - The main policy is converted to an inline Ampel policy
//   - Imported policies are added as external references
//
// Options:
//   - WithPolicySetMetadata: Set name, description, and version for the PolicySet
//   - WithCatalog: Include catalog data to enrich tenet descriptions
//   - WithCELTemplates: Custom CEL code templates for method types
func FromPolicyWithImports(policy *gemara.Policy, opts ...PolicySetOption) (PolicySet, error) {
	policySet := PolicySet{
		Policies: []PolicyReference{},
		Metadata: make(map[string]string),
	}

	// Apply policy set options
	psOptions := &PolicySetOptions{}
	for _, opt := range opts {
		opt(psOptions)
	}

	// Set PolicySet metadata (default to main policy metadata if not provided)
	if psOptions.Name != "" {
		policySet.Name = psOptions.Name
	} else {
		policySet.Name = policy.Title
	}

	if psOptions.Description != "" {
		policySet.Description = psOptions.Description
	} else {
		policySet.Description = policy.Metadata.Description
	}

	if psOptions.Version != "" {
		policySet.Version = psOptions.Version
	} else {
		policySet.Version = policy.Metadata.Version
	}

	if psOptions.Metadata != nil {
		policySet.Metadata = psOptions.Metadata
	}

	// Convert the main policy
	ampelPolicy, err := FromPolicy(policy, psOptions.TransformOptions...)
	if err != nil {
		return policySet, fmt.Errorf("error converting main policy: %w", err)
	}

	mainPolicyRef := PolicyReference{
		ID:     policy.Metadata.Id,
		Policy: &ampelPolicy,
	}

	// Add metadata for main policy if provided
	if psOptions.PolicyMetadata != nil {
		if meta, ok := psOptions.PolicyMetadata[policy.Metadata.Id]; ok {
			mainPolicyRef.Meta = meta
		}
	}

	policySet.Policies = append(policySet.Policies, mainPolicyRef)

	// Add imported policies as references
	for _, importedPolicyRef := range policy.Imports.Policies {
		policyRef := PolicyReference{
			ID: extractPolicyIdFromReference(importedPolicyRef),
			Source: &PolicySource{
				Location: PolicyLocation{
					URI: importedPolicyRef,
				},
			},
		}

		// Add metadata for imported policy if provided
		if psOptions.PolicyMetadata != nil {
			if meta, ok := psOptions.PolicyMetadata[policyRef.ID]; ok {
				policyRef.Meta = meta
			}
		}

		policySet.Policies = append(policySet.Policies, policyRef)
	}

	// Validate the generated policy set
	if err := policySet.Validate(); err != nil {
		return policySet, fmt.Errorf("generated policy set validation failed: %w", err)
	}

	return policySet, nil
}

// extractPolicyIdFromReference extracts a policy ID from a reference string.
// For URIs like "git+https://github.com/org/repo#path/to/policy.json",
// it extracts "policy" from the filename.
func extractPolicyIdFromReference(reference string) string {
	// Try to extract from fragment first (after #)
	parts := strings.Split(reference, "#")
	if len(parts) > 1 {
		// Get the last part (the path after #)
		policyPath := parts[len(parts)-1]
		// Extract filename without extension
		base := path.Base(policyPath)
		ext := path.Ext(base)
		if ext != "" {
			return strings.TrimSuffix(base, ext)
		}
		return base
	}

	// Fallback: use the whole reference as ID
	return reference
}
