package ampel

import (
	"strings"

	"github.com/ossf/gemara"
)

// AttestationTypeInference analyzes a policy to determine which attestation
// types are required for verification.
type AttestationTypeInference struct {
	// RequiresProvenance indicates SLSA provenance attestations are needed
	RequiresProvenance bool

	// RequiresSBOM indicates SBOM attestations are needed
	RequiresSBOM bool

	// RequiresVulnScan indicates vulnerability scan attestations are needed
	RequiresVulnScan bool

	// CustomTypes lists any custom attestation predicate types detected
	CustomTypes []string
}

// AllTypes returns all inferred attestation types as a list of predicate type URLs.
func (inf *AttestationTypeInference) AllTypes() []string {
	var types []string

	if inf.RequiresProvenance {
		types = append(types, "https://slsa.dev/provenance/v1")
	}
	if inf.RequiresSBOM {
		// Include both SPDX and CycloneDX
		types = append(types, "https://spdx.dev/Document", "https://cyclonedx.org/bom")
	}
	if inf.RequiresVulnScan {
		types = append(types, "https://in-toto.io/Statement/v0.1")
	}

	types = append(types, inf.CustomTypes...)
	return types
}

// InferAttestationTypes analyzes a Gemara policy to determine what attestation
// types are required based on evidence requirements and assessment plans.
func InferAttestationTypes(policy *gemara.Policy) AttestationTypeInference {
	inf := AttestationTypeInference{}

	// Analyze assessment plans for evidence requirements
	for _, plan := range policy.Adherence.AssessmentPlans {
		if plan.EvidenceRequirements != "" {
			analyzeEvidenceRequirement(plan.EvidenceRequirements, &inf)
		}
	}

	// Analyze evaluation methods for attestation hints
	for _, method := range policy.Adherence.EvaluationMethods {
		if method.Description != "" {
			analyzeEvidenceRequirement(method.Description, &inf)
		}
	}

	return inf
}

// InferAttestationType infers a single attestation predicate type URL from
// an evidence requirement string.
func InferAttestationType(evidenceReq string) string {
	lowerReq := strings.ToLower(evidenceReq)

	// Check for SLSA provenance keywords
	for _, keyword := range []string{"slsa", "provenance", "builder", "build provenance"} {
		if strings.Contains(lowerReq, keyword) {
			return "https://slsa.dev/provenance/v1"
		}
	}

	// Check for SBOM keywords
	if strings.Contains(lowerReq, "spdx") {
		return "https://spdx.dev/Document"
	}
	if strings.Contains(lowerReq, "cyclonedx") {
		return "https://cyclonedx.org/bom"
	}
	if strings.Contains(lowerReq, "sbom") || strings.Contains(lowerReq, "software bill of materials") {
		// Default to SPDX if SBOM type not specified
		return "https://spdx.dev/Document"
	}

	// Check for vulnerability scan keywords
	for _, keyword := range []string{"vulnerabilit", "cve", "security scan", "vuln scan"} {
		if strings.Contains(lowerReq, keyword) {
			return "https://in-toto.io/Statement/v0.1"
		}
	}

	// Check for in-toto attestation keywords
	if strings.Contains(lowerReq, "in-toto") || strings.Contains(lowerReq, "attestation") {
		return "https://in-toto.io/Statement/v0.1"
	}

	// No specific type detected
	return ""
}

// analyzeEvidenceRequirement updates the inference based on an evidence requirement string.
func analyzeEvidenceRequirement(evidenceReq string, inf *AttestationTypeInference) {
	lowerReq := strings.ToLower(evidenceReq)

	// Check for SLSA provenance
	for _, keyword := range []string{"slsa", "provenance", "builder", "build provenance", "build attestation"} {
		if strings.Contains(lowerReq, keyword) {
			inf.RequiresProvenance = true
			break
		}
	}

	// Check for SBOM
	for _, keyword := range []string{"sbom", "software bill of materials", "spdx", "cyclonedx", "dependency"} {
		if strings.Contains(lowerReq, keyword) {
			inf.RequiresSBOM = true
			break
		}
	}

	// Check for vulnerability scans
	for _, keyword := range []string{"vulnerabilit", "cve", "security scan", "vuln scan", "vulnerability scan"} {
		if strings.Contains(lowerReq, keyword) {
			inf.RequiresVulnScan = true
			break
		}
	}

	// Check for explicit predicate type URLs
	if strings.Contains(evidenceReq, "https://") {
		// Extract URL-like strings
		parts := strings.Fields(evidenceReq)
		for _, part := range parts {
			if strings.HasPrefix(part, "https://") {
				// This might be a custom attestation type
				if !isStandardAttestationType(part) {
					inf.CustomTypes = append(inf.CustomTypes, part)
				}
			}
		}
	}
}

// isStandardAttestationType checks if a URL is one of the standard attestation types.
func isStandardAttestationType(url string) bool {
	standardTypes := []string{
		"https://slsa.dev/provenance/v1",
		"https://spdx.dev/Document",
		"https://cyclonedx.org/bom",
		"https://in-toto.io/Statement/v0.1",
		"https://in-toto.io/Statement/v1",
	}

	for _, stdType := range standardTypes {
		if strings.Contains(url, stdType) {
			return true
		}
	}
	return false
}

// EvidenceKeywordMapping maps common evidence requirement keywords to
// attestation predicate type URLs.
var EvidenceKeywordMapping = map[string]string{
	"slsa":                      "https://slsa.dev/provenance/v1",
	"provenance":                "https://slsa.dev/provenance/v1",
	"build provenance":          "https://slsa.dev/provenance/v1",
	"sbom":                      "https://spdx.dev/Document",
	"software bill of materials": "https://spdx.dev/Document",
	"spdx":                      "https://spdx.dev/Document",
	"cyclonedx":                 "https://cyclonedx.org/bom",
	"vulnerability":             "https://in-toto.io/Statement/v0.1",
	"vulnerability scan":        "https://in-toto.io/Statement/v0.1",
	"cve":                       "https://in-toto.io/Statement/v0.1",
	"security scan":             "https://in-toto.io/Statement/v0.1",
	"in-toto":                   "https://in-toto.io/Statement/v0.1",
}
