package ampel

import (
	"encoding/json"
)

// AmpelPolicy represents an Ampel verification policy that uses CEL
// (Common Expression Language) to verify attestations in the in-toto format.
//
// Ampel policies consist of tenets (individual verification checks) that are
// evaluated against attestations to ensure supply chain security requirements
// are met.
type AmpelPolicy struct {
	// Name is the policy identifier
	Name string `json:"name"`

	// Description provides context about the policy's purpose
	Description string `json:"description,omitempty"`

	// Version specifies the policy version
	Version string `json:"version,omitempty"`

	// Metadata contains additional policy metadata (e.g., RACI contacts, org info)
	Metadata map[string]string `json:"metadata,omitempty"`

	// Imports lists other policies or resources referenced by this policy
	Imports []string `json:"imports,omitempty"`

	// Tenets are the individual verification checks that make up this policy
	Tenets []Tenet `json:"tenets"`

	// Rule is the overall evaluation logic, typically "all(tenets)" meaning
	// all tenets must pass for the policy to pass
	Rule string `json:"rule"`
}

// Tenet represents a single verification check with CEL code that evaluates
// attestations to verify compliance with a specific requirement.
type Tenet struct {
	// ID uniquely identifies this tenet within the policy
	ID string `json:"id"`

	// Name provides a human-readable name for the tenet
	Name string `json:"name"`

	// Description explains what this tenet verifies
	Description string `json:"description,omitempty"`

	// Code contains the CEL expression that performs the verification
	// The expression should evaluate to a boolean (true = pass, false = fail)
	Code string `json:"code"`

	// AttestationTypes lists the expected attestation predicate types
	// this tenet operates on (e.g., "https://slsa.dev/provenance/v1")
	AttestationTypes []string `json:"attestationTypes,omitempty"`

	// Parameters contains configurable values for this tenet
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// PolicySet contains multiple related Ampel policies that can be
// evaluated together. Supports both inline policies and policy references.
type PolicySet struct {
	// Name is an optional identifier for the policy set
	Name string `json:"name,omitempty"`

	// Description provides context about the policy set's purpose
	Description string `json:"description,omitempty"`

	// Version specifies the policy set version
	Version string `json:"version,omitempty"`

	// Metadata contains additional policy set metadata
	Metadata map[string]string `json:"metadata,omitempty"`

	// Policies contains the list of policies, which can be inline or references
	Policies []PolicyReference `json:"policies"`
}

// PolicyReference represents either an inline policy or a reference to an external policy.
type PolicyReference struct {
	// ID uniquely identifies this policy within the set
	ID string `json:"id"`

	// Source specifies the location of an external policy (optional)
	// If Source is nil, the policy is inline
	Source *PolicySource `json:"source,omitempty"`

	// Policy contains the inline policy definition (optional)
	// If Policy is nil, Source must be provided
	Policy *AmpelPolicy `json:"policy,omitempty"`

	// Meta contains metadata about the policy
	Meta *PolicyMeta `json:"meta,omitempty"`
}

// PolicySource specifies where to load a policy from.
type PolicySource struct {
	// Location contains the URI or path to the policy
	Location PolicyLocation `json:"location"`
}

// PolicyLocation specifies the exact location of a policy.
type PolicyLocation struct {
	// URI is the location of the policy (e.g., "git+https://github.com/...")
	URI string `json:"uri"`
}

// PolicyMeta contains metadata about a policy in a policy set.
type PolicyMeta struct {
	// Controls lists the compliance frameworks this policy addresses
	Controls []ControlReference `json:"controls,omitempty"`

	// Enforce specifies the enforcement mode (e.g., "ON", "OFF", "WARN")
	Enforce string `json:"enforce,omitempty"`

	// Additional metadata fields
	AdditionalMetadata map[string]interface{} `json:"additionalMetadata,omitempty"`
}

// ControlReference identifies a specific control in a compliance framework.
type ControlReference struct {
	// Framework is the compliance framework name (e.g., "SLSA", "NIST")
	Framework string `json:"framework"`

	// Class is the category or class within the framework (e.g., "BUILD")
	Class string `json:"class,omitempty"`

	// ID is the specific control identifier (e.g., "LEVEL_3")
	ID string `json:"id"`
}

// ToJSON serializes the Ampel policy to JSON format.
func (p *AmpelPolicy) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ToJSONCompact serializes the Ampel policy to compact JSON format.
func (p *AmpelPolicy) ToJSONCompact() ([]byte, error) {
	return json.Marshal(p)
}

// ToJSON serializes the PolicySet to JSON format.
func (ps *PolicySet) ToJSON() ([]byte, error) {
	return json.MarshalIndent(ps, "", "  ")
}

// ToJSONCompact serializes the PolicySet to compact JSON format.
func (ps *PolicySet) ToJSONCompact() ([]byte, error) {
	return json.Marshal(ps)
}

// Validate checks if the PolicySet is well-formed.
func (ps *PolicySet) Validate() error {
	if len(ps.Policies) == 0 {
		return &ValidationError{Field: "policies", Message: "policy set must have at least one policy"}
	}
	for i, policyRef := range ps.Policies {
		if err := policyRef.Validate(); err != nil {
			return &ValidationError{Field: "policies", Index: i, Cause: err}
		}
	}
	return nil
}

// Validate checks if the PolicyReference is well-formed.
func (pr *PolicyReference) Validate() error {
	if pr.ID == "" {
		return &ValidationError{Field: "id", Message: "policy reference ID is required"}
	}

	// Must have either Source or Policy, but not both
	hasSource := pr.Source != nil
	hasPolicy := pr.Policy != nil

	if !hasSource && !hasPolicy {
		return &ValidationError{
			Field:   "policy reference",
			Message: "must have either source or inline policy",
		}
	}

	// Validate inline policy if present
	if hasPolicy {
		if err := pr.Policy.Validate(); err != nil {
			return &ValidationError{Field: "policy", Cause: err}
		}
	}

	// Validate source if present
	if hasSource {
		if pr.Source.Location.URI == "" {
			return &ValidationError{
				Field:   "source.location.uri",
				Message: "policy source URI is required",
			}
		}
	}

	return nil
}

// Validate checks if the Ampel policy is well-formed.
func (p *AmpelPolicy) Validate() error {
	if p.Name == "" {
		return &ValidationError{Field: "name", Message: "policy name is required"}
	}
	if len(p.Tenets) == 0 {
		return &ValidationError{Field: "tenets", Message: "policy must have at least one tenet"}
	}
	if p.Rule == "" {
		return &ValidationError{Field: "rule", Message: "policy rule is required"}
	}
	for i, tenet := range p.Tenets {
		if err := tenet.Validate(); err != nil {
			return &ValidationError{Field: "tenets", Index: i, Cause: err}
		}
	}
	return nil
}

// Validate checks if the Tenet is well-formed.
func (t *Tenet) Validate() error {
	if t.ID == "" {
		return &ValidationError{Field: "id", Message: "tenet ID is required"}
	}
	if t.Name == "" {
		return &ValidationError{Field: "name", Message: "tenet name is required"}
	}
	if t.Code == "" {
		return &ValidationError{Field: "code", Message: "tenet code (CEL expression) is required"}
	}
	return nil
}

// ValidationError represents an error during policy validation.
type ValidationError struct {
	Field   string
	Index   int
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Index >= 0 {
		if e.Cause != nil {
			return "validation error in " + e.Field + "[" + string(rune(e.Index)) + "]: " + e.Cause.Error()
		}
		return "validation error in " + e.Field + "[" + string(rune(e.Index)) + "]: " + e.Message
	}
	if e.Cause != nil {
		return "validation error in " + e.Field + ": " + e.Cause.Error()
	}
	return "validation error in " + e.Field + ": " + e.Message
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}
