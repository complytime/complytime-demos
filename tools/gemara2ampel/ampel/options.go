package ampel

import "github.com/gemaraproj/go-gemara"

// TransformOptions configures the transformation from Gemara Layer-3 policies
// to Ampel verification policies.
type TransformOptions struct {
	// Catalog is an optional catalog used to enrich tenets with control details
	Catalog *gemara.ControlCatalog

	// CELTemplates provides custom CEL code templates for generating verification logic
	// Key: template name, Value: CEL template string with {{.Parameter}} placeholders
	CELTemplates map[string]string

	// DefaultAttestationTypes specifies the attestation types to expect if not
	// automatically inferred from evidence requirements
	DefaultAttestationTypes []string

	// IncludeScopeFilters determines whether to generate CEL filters based on
	// the policy's scope dimensions (technologies, geopolitical, sensitivity, etc.)
	IncludeScopeFilters bool

	// DefaultRule specifies the overall policy rule if not provided
	// Default: "all(tenets)" meaning all tenets must pass
	DefaultRule string
}

// TransformOption is a function that configures TransformOptions.
type TransformOption func(*TransformOptions)

// WithCatalog sets a catalog to use for enriching tenets with control details.
// When provided, the transformation will:
//   - Look up requirement IDs in the catalog
//   - Use requirement text as tenet titles (more descriptive than evidence requirements)
//   - Add control references to policy metadata (Meta.Controls)
//   - Include control family information (framework and class)
func WithCatalog(catalog *gemara.ControlCatalog) TransformOption {
	return func(opts *TransformOptions) {
		opts.Catalog = catalog
	}
}

// WithCELTemplates provides custom CEL code templates for generating
// verification logic. Templates should use Go text/template syntax with
// parameters accessible via {{.ParameterName}}.
//
// Example:
//
//	templates := map[string]string{
//	    "custom-check": `attestation.predicate.field == "{{.ExpectedValue}}"`,
//	}
//	ampel.FromPolicy(policy, ampel.WithCELTemplates(templates))
func WithCELTemplates(templates map[string]string) TransformOption {
	return func(opts *TransformOptions) {
		opts.CELTemplates = templates
	}
}

// WithAttestationTypes specifies the expected attestation types to verify.
// This overrides automatic type inference from evidence requirements.
//
// Common attestation types:
//   - "https://slsa.dev/provenance/v1" - SLSA provenance
//   - "https://in-toto.io/Statement/v0.1" - Generic in-toto statement
func WithAttestationTypes(types []string) TransformOption {
	return func(opts *TransformOptions) {
		opts.DefaultAttestationTypes = types
	}
}

// WithScopeFilters enables generation of CEL filtering expressions based on
// the policy's scope dimensions. When enabled, tenets will include filters
// for technologies, geopolitical regions, sensitivity levels, etc.
//
// Example: If scope includes technologies=["Cloud Computing"], the generated
// CEL will include: subject.type == "cloud-app"
func WithScopeFilters(include bool) TransformOption {
	return func(opts *TransformOptions) {
		opts.IncludeScopeFilters = include
	}
}

// WithDefaultRule sets the overall policy evaluation rule.
// Default is "all(tenets)" which requires all tenets to pass.
//
// Other examples:
//   - "any(tenets)" - at least one tenet must pass
//   - "tenets[0] && tenets[1]" - specific tenets must pass
func WithDefaultRule(rule string) TransformOption {
	return func(opts *TransformOptions) {
		opts.DefaultRule = rule
	}
}

// applyDefaults sets default values for any unset options.
func (opts *TransformOptions) applyDefaults() {
	if opts.DefaultRule == "" {
		opts.DefaultRule = "all(tenets)"
	}
	if opts.CELTemplates == nil {
		opts.CELTemplates = make(map[string]string)
	}
	// Merge default templates with custom templates
	for k, v := range DefaultCELTemplates {
		if _, exists := opts.CELTemplates[k]; !exists {
			opts.CELTemplates[k] = v
		}
	}
}

// PolicySetOptions configures the transformation from Gemara Layer-3 policies
// to Ampel PolicySet.
type PolicySetOptions struct {
	// Name is the identifier for the policy set
	Name string

	// Description provides context about the policy set's purpose
	Description string

	// Version specifies the policy set version
	Version string

	// Metadata contains additional policy set custom metadata
	Metadata map[string]interface{}

	// Meta maps policy IDs to their metadata (controls, enforcement, etc.)
	Meta map[string]*Meta

	// TransformOptions are passed to FromPolicy for each policy transformation
	TransformOptions []TransformOption
}

// PolicySetOption is a function that configures PolicySetOptions.
type PolicySetOption func(*PolicySetOptions)

// WithPolicySetMetadata sets the name, description, and version for the PolicySet.
//
// Example:
//
//	ampel.FromPolicies(policies,
//	    ampel.WithPolicySetMetadata("My Policy Set", "Collection of security policies", "1.0.0"),
//	)
func WithPolicySetMetadata(name, description, version string) PolicySetOption {
	return func(opts *PolicySetOptions) {
		opts.Name = name
		opts.Description = description
		opts.Version = version
	}
}

// WithPolicySetCustomMetadata sets custom metadata fields for the PolicySet.
//
// Example:
//
//	metadata := map[string]interface{}{
//	    "author": "Security Team",
//	    "organization": "ACME Corp",
//	}
//	ampel.FromPolicies(policies, ampel.WithPolicySetCustomMetadata(metadata))
func WithPolicySetCustomMetadata(metadata map[string]interface{}) PolicySetOption {
	return func(opts *PolicySetOptions) {
		if opts.Metadata == nil {
			opts.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			opts.Metadata[k] = v
		}
	}
}

// WithMeta sets metadata for a specific policy in the policy set.
// This allows you to specify controls, enforcement mode, and other metadata
// for individual policies.
//
// Example:
//
//	meta := &ampel.Meta{
//	    Controls: []ampel.ControlReference{
//	        {Framework: "SLSA", Class: "BUILD", ID: "LEVEL_3"},
//	    },
//	    Enforce: "ON",
//	}
//	ampel.FromPolicies(policies, ampel.WithMeta("policy-001", meta))
func WithMeta(policyID string, meta *Meta) PolicySetOption {
	return func(opts *PolicySetOptions) {
		if opts.Meta == nil {
			opts.Meta = make(map[string]*Meta)
		}
		opts.Meta[policyID] = meta
	}
}

// WithTransformOptions sets the transformation options to use for converting
// individual policies within the policy set.
//
// Example:
//
//	ampel.FromPolicies(policies,
//	    ampel.WithTransformOptions(
//	        ampel.WithCatalog(catalog),
//	        ampel.WithScopeFilters(true),
//	    ),
//	)
func WithTransformOptions(transformOpts ...TransformOption) PolicySetOption {
	return func(opts *PolicySetOptions) {
		opts.TransformOptions = append(opts.TransformOptions, transformOpts...)
	}
}
