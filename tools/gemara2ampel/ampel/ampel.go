package ampel

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/gemaraproj/go-gemara"
	"google.golang.org/protobuf/types/known/structpb"
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
func FromPolicy(policy *gemara.Policy, opts ...TransformOption) (*Policy, error) {
	options := &TransformOptions{}
	for _, opt := range opts {
		opt(options)
	}
	options.applyDefaults()

	ampelPolicy := &Policy{
		Id: policy.Metadata.Id,
		Meta: &Meta{
			Runtime:     "cel@v14.0",
			Description: policy.Metadata.Description,
			AssertMode:  ruleToAssertMode(options.DefaultRule),
		},
		Tenets: []*Tenet{},
	}

	// Transform metadata
	if err := buildMetadata(policy, ampelPolicy); err != nil {
		return nil, fmt.Errorf("error building metadata: %w", err)
	}

	// Build Policy.Context from Gemara parameters
	if err := buildContextFromParameters(policy, ampelPolicy); err != nil {
		return nil, fmt.Errorf("error building context from parameters: %w", err)
	}

	// Track catalog enrichments for adding control references to metadata
	var allEnrichments []*CatalogEnrichment

	// Convert assessment plans to tenets
	for _, plan := range policy.Adherence.AssessmentPlans {
		tenets, enrichments, err := assessmentPlanToTenets(plan, policy, options)
		if err != nil {
			return nil, fmt.Errorf("error converting assessment plan %s: %w", plan.Id, err)
		}
		ampelPolicy.Tenets = append(ampelPolicy.Tenets, tenets...)
		allEnrichments = append(allEnrichments, enrichments...)
	}

	// Add control references to policy metadata if catalog enrichment was used
	if len(allEnrichments) > 0 {
		controls := collectControlReferences(allEnrichments)
		if len(controls) > 0 {
			ampelPolicy.Meta.Controls = controls
		}
	}

	// Validate the generated policy
	if err := ampelPolicy.Validate(); err != nil {
		return nil, fmt.Errorf("generated policy validation failed: %w", err)
	}

	return ampelPolicy, nil
}

// ruleToAssertMode converts a policy rule string to AssertMode
// "all(tenets)" -> "AND", "any(tenets)" -> "OR"
func ruleToAssertMode(rule string) string {
	if strings.Contains(rule, "any") {
		return "OR"
	}
	return "AND"
}

// buildMetadata extracts metadata from Gemara policy and populates Ampel policy metadata.
func buildMetadata(policy *gemara.Policy, ampelPolicy *Policy) error {
	// Parse version string to int64
	version := int64(0)
	if policy.Metadata.Version != "" {
		// Try to parse version (e.g., "1.0.0" -> 1)
		parts := strings.Split(policy.Metadata.Version, ".")
		if len(parts) > 0 {
			if v, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
				version = v
			}
		}
	}
	ampelPolicy.Meta.Version = version

	// Note: The official Ampel policy format doesn't include author, contacts, or scope
	// These could be added to a custom metadata extension if needed

	return nil
}

// buildContextFromParameters collects all parameters from assessment plans and
// creates Policy.Context with ContextVal entries for each parameter.
func buildContextFromParameters(policy *gemara.Policy, ampelPolicy *Policy) error {
	// Collect all unique parameters from all assessment plans
	parametersMap := make(map[string]gemara.Parameter)

	for _, plan := range policy.Adherence.AssessmentPlans {
		for _, param := range plan.Parameters {
			// Use parameter ID as the key to avoid duplicates
			if _, exists := parametersMap[param.Id]; !exists {
				parametersMap[param.Id] = param
			}
		}
	}

	// If no parameters, skip context creation
	if len(parametersMap) == 0 {
		return nil
	}

	// Initialize Policy.Context if needed
	if ampelPolicy.Context == nil {
		ampelPolicy.Context = make(map[string]*ContextVal)
	}

	// Convert each parameter to ContextVal
	for paramId, param := range parametersMap {
		contextVal, err := parameterToContextVal(param)
		if err != nil {
			return fmt.Errorf("error converting parameter %s to ContextVal: %w", paramId, err)
		}
		ampelPolicy.Context[paramId] = contextVal
	}

	return nil
}

// parameterToContextVal converts a Gemara Parameter to an Ampel ContextVal.
func parameterToContextVal(param gemara.Parameter) (*ContextVal, error) {
	contextVal := &ContextVal{
		Type: "string", // Default to string type
	}

	// Set description from parameter description or label
	if param.Description != "" {
		desc := param.Description
		contextVal.Description = &desc
	} else if param.Label != "" {
		desc := param.Label
		contextVal.Description = &desc
	}

	// Set default value from first AcceptedValue if available
	// Note: When there are multiple accepted values, they are enforced as hardcoded
	// constraints in the CEL expression (e.g., field in ["val1", "val2"]).
	// The context stores the default runtime value (the first option).
	if len(param.AcceptedValues) > 0 {
		defaultValue, err := structpb.NewValue(param.AcceptedValues[0])
		if err != nil {
			return nil, fmt.Errorf("error creating default value: %w", err)
		}
		contextVal.Default = defaultValue

		// Also set as the current value
		contextVal.Value = defaultValue
	}

	// Mark as required if no accepted values are provided
	// (parameters without accepted values likely need runtime provision)
	required := len(param.AcceptedValues) == 0
	contextVal.Required = &required

	return contextVal, nil
}

// buildCELParams creates a parameter map for CEL template substitution.
// It generates context references for parameters to be used in CEL expressions.
func buildCELParams(parameters []gemara.Parameter) map[string]interface{} {
	celParams := make(map[string]interface{})

	for _, param := range parameters {
		// Use the original parameter ID as the key
		paramKey := param.Id

		if len(param.AcceptedValues) > 0 {
			// Generate context reference: context["param-id"]
			contextRef := fmt.Sprintf("context[\"%s\"]", param.Id)
			celParams[paramKey] = contextRef

			// For multiple accepted values, create a comma-separated list
			// This is used for "in" expressions: field in [value1, value2]
			if len(param.AcceptedValues) > 1 {
				var quotedValues []string
				for _, val := range param.AcceptedValues {
					quotedValues = append(quotedValues, fmt.Sprintf(`"%s"`, val))
				}
				// Store with "-list" suffix for template access
				celParams[paramKey+"-list"] = strings.Join(quotedValues, ", ")
			}
		} else {
			// For parameters without accepted values (runtime-provided),
			// still generate context reference
			contextRef := fmt.Sprintf("context[\"%s\"]", param.Id)
			celParams[paramKey] = contextRef
		}
	}

	return celParams
}

// assessmentPlanToTenets converts a single assessment plan to one or more Ampel tenets.
// Returns the tenets and catalog enrichments (if any) for tracking control references.
func assessmentPlanToTenets(
	plan gemara.AssessmentPlan,
	policy *gemara.Policy,
	options *TransformOptions,
) ([]*Tenet, []*CatalogEnrichment, error) {
	var tenets []*Tenet
	var enrichments []*CatalogEnrichment

	// Look up requirement in catalog if available
	var enrichment *CatalogEnrichment
	if options.Catalog != nil && plan.RequirementId != "" {
		enrichment = lookupRequirement(options.Catalog, plan.RequirementId)
	}

	// Get evidence requirements
	evidenceReq := plan.EvidenceRequirements

	// Process each evaluation method
	methodIndex := 0
	for _, method := range plan.EvaluationMethods {
		// Only process automated methods
		if !isAutomatedMethod(method) {
			continue
		}

		// Build CEL parameters for template substitution
		// Parameters are now stored in Policy.Context and referenced in CEL as context["param-id"]
		celParams := buildCELParams(plan.Parameters)

		// Generate CEL expression
		celCode, attestationTypes, err := GenerateCELFromMethod(method, evidenceReq, celParams, options.CELTemplates)
		if err != nil {
			return nil, nil, fmt.Errorf("error generating CEL for method %d: %w", methodIndex, err)
		}

		// Apply scope filters if enabled
		if options.IncludeScopeFilters {
			scopeFilter := ScopeFilterToCEL(policy.Scope.In)
			if scopeFilter != "" {
				celCode = fmt.Sprintf("(%s) && (%s)", scopeFilter, celCode)
			}
		}

		// Build tenet title - use catalog enrichment if available
		defaultTitle := getTenetName(method, evidenceReq)
		title := enrichTenetTitle(enrichment, defaultTitle)

		// Create tenet with official Ampel format
		tenet := &Tenet{
			Id:      fmt.Sprintf("%s-%s-%d", plan.RequirementId, plan.Id, methodIndex),
			Title:   title,
			Runtime: "cel@v14.0",
			Code:    celCode,
		}

		// Add PredicateSpec with attestation types
		if len(attestationTypes) > 0 {
			tenet.Predicates = &PredicateSpec{
				Types: attestationTypes,
			}
		}

		tenets = append(tenets, tenet)

		// Track enrichment if found (one per tenet)
		if enrichment != nil {
			enrichments = append(enrichments, enrichment)
		}

		methodIndex++
	}

	return tenets, enrichments, nil
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
	return fmt.Sprintf("%s verification", method.Type.String())
}

// isAutomatedMethod checks if an evaluation method can be automated.
// In go-gemara v0.5.0, automation is determined by the Mode field
// (ModeAutomated) rather than the Type field.
func isAutomatedMethod(method gemara.AcceptedMethod) bool {
	return method.Mode == gemara.ModeAutomated
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
func FromPolicies(policies []*gemara.Policy, opts ...PolicySetOption) (*PolicySet, error) {
	if len(policies) == 0 {
		return nil, fmt.Errorf("at least one policy is required")
	}

	// Apply policy set options
	psOptions := &PolicySetOptions{}
	for _, opt := range opts {
		opt(psOptions)
	}

	// Parse version string to int64 for PolicySetMeta
	version := int64(0)
	if psOptions.Version != "" {
		// Try to parse version (e.g., "1.0.0" -> 1)
		parts := strings.Split(psOptions.Version, ".")
		if len(parts) > 0 {
			if v, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
				version = v
			}
		}
	}

	policySet := &PolicySet{
		Id: psOptions.Name,
		Meta: &PolicySetMeta{
			Description: psOptions.Description,
			Version:     version,
		},
		Policies: []*Policy{},
	}

	// Convert each Gemara policy to Ampel policy
	for _, gemaraPolicy := range policies {
		ampelPolicy, err := FromPolicy(gemaraPolicy, psOptions.TransformOptions...)
		if err != nil {
			return nil, fmt.Errorf("error converting policy %s: %w", gemaraPolicy.Metadata.Id, err)
		}

		// Override metadata if provided in options
		if psOptions.Meta != nil {
			if meta, ok := psOptions.Meta[gemaraPolicy.Metadata.Id]; ok {
				ampelPolicy.Meta = meta
			}
		}

		policySet.Policies = append(policySet.Policies, ampelPolicy)
	}

	// Validate the generated policy set
	if err := policySet.Validate(); err != nil {
		return nil, fmt.Errorf("generated policy set validation failed: %w", err)
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
func FromPolicyWithImports(policy *gemara.Policy, opts ...PolicySetOption) (*PolicySet, error) {
	// Apply policy set options
	psOptions := &PolicySetOptions{}
	for _, opt := range opts {
		opt(psOptions)
	}

	// Set PolicySet metadata (default to main policy metadata if not provided)
	policySetId := psOptions.Name
	if policySetId == "" {
		policySetId = policy.Metadata.Id + "-set"
	}

	policySetDesc := psOptions.Description
	if policySetDesc == "" {
		policySetDesc = policy.Metadata.Description
	}

	policySetVersion := psOptions.Version
	if policySetVersion == "" {
		policySetVersion = policy.Metadata.Version
	}

	// Parse version string to int64 for PolicySetMeta
	setVersion := int64(0)
	if policySetVersion != "" {
		parts := strings.Split(policySetVersion, ".")
		if len(parts) > 0 {
			if v, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
				setVersion = v
			}
		}
	}

	policySet := &PolicySet{
		Id: policySetId,
		Meta: &PolicySetMeta{
			Description: policySetDesc,
			Version:     setVersion,
		},
		Policies: []*Policy{},
	}

	// Convert the main policy
	ampelPolicy, err := FromPolicy(policy, psOptions.TransformOptions...)
	if err != nil {
		return nil, fmt.Errorf("error converting main policy: %w", err)
	}

	// Override metadata for main policy if provided
	if psOptions.Meta != nil {
		if meta, ok := psOptions.Meta[policy.Metadata.Id]; ok {
			ampelPolicy.Meta = meta
		}
	}

	policySet.Policies = append(policySet.Policies, ampelPolicy)

	// Add imported policies as external references
	for _, importedPolicyRef := range policy.Imports.Policies {
		// Create a Policy with Source reference for external policies
		extPolicy := &Policy{
			Id: extractPolicyIdFromReference(importedPolicyRef.ReferenceId),
			Source: &PolicyRef{
				Id: extractPolicyIdFromReference(importedPolicyRef.ReferenceId),
				Location: &ResourceDescriptor{
					Uri: importedPolicyRef.ReferenceId,
				},
			},
		}

		// Add metadata for imported policy if provided
		if psOptions.Meta != nil {
			if meta, ok := psOptions.Meta[extPolicy.Id]; ok {
				extPolicy.Meta = meta
			}
		}

		policySet.Policies = append(policySet.Policies, extPolicy)
	}

	// Validate the generated policy set
	if err := policySet.Validate(); err != nil {
		return nil, fmt.Errorf("generated policy set validation failed: %w", err)
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
