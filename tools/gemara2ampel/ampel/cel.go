package ampel

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/ossf/gemara"
)

// DefaultCELTemplates provides CEL code templates for common attestation
// verification patterns. Templates use Go text/template syntax.
var DefaultCELTemplates = map[string]string{
	// SLSA provenance verification templates
	"slsa-provenance-builder": `attestation.predicateType == "https://slsa.dev/provenance/v1" && attestation.predicate.builder.id == "{{.BuilderId}}"`,

	"slsa-provenance-builder-in": `attestation.predicateType == "https://slsa.dev/provenance/v1" && attestation.predicate.builder.id in [{{.BuilderIds}}]`,

	"slsa-provenance-materials": `attestation.predicateType == "https://slsa.dev/provenance/v1" && all(attestation.predicate.materials, m, m.digest.sha256 != "")`,

	"slsa-provenance-buildtype": `attestation.predicateType == "https://slsa.dev/provenance/v1" && attestation.predicate.buildType == "{{.BuildType}}"`,

	// SBOM verification templates
	"sbom-present": `attestation.predicateType == "https://spdx.dev/Document" || attestation.predicateType == "https://cyclonedx.org/bom"`,

	"sbom-spdx": `attestation.predicateType == "https://spdx.dev/Document"`,

	"sbom-cyclonedx": `attestation.predicateType == "https://cyclonedx.org/bom"`,

	// Vulnerability scan templates
	"vulnerability-scan-no-critical": `attestation.predicateType == "https://in-toto.io/Statement/v0.1" && attestation.predicate.scanner.result.summary.critical == 0`,

	"vulnerability-scan-threshold": `attestation.predicateType == "https://in-toto.io/Statement/v0.1" && attestation.predicate.scanner.result.summary.critical == 0 && attestation.predicate.scanner.result.summary.high < {{.MaxHigh}}`,

	"vulnerability-scanner": `attestation.predicateType == "https://in-toto.io/Statement/v0.1" && attestation.predicate.scanner.vendor == "{{.Scanner}}"`,

	// Generic templates
	"generic-predicate-type": `attestation.predicateType == "{{.PredicateType}}"`,

	"generic-field-equals": `attestation.predicate.{{.FieldPath}} == "{{.ExpectedValue}}"`,

	"generic-field-in": `attestation.predicate.{{.FieldPath}} in [{{.AllowedValues}}]`,
}

// MethodTypeToCELTemplate maps Gemara evaluation method types to
// default CEL template names for common scenarios.
var MethodTypeToCELTemplate = map[string]string{
	"automated":        "generic-field-equals",
	"gate":             "generic-predicate-type",
	"behavioral":       "generic-field-equals",
	"autoremediation": "generic-field-equals",
}

// GenerateCEL creates a CEL expression from a template and parameters.
// The template should use Go text/template syntax.
func GenerateCEL(templateStr string, params map[string]interface{}) (string, error) {
	tmpl, err := template.New("cel").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse CEL template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("failed to execute CEL template: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}

// GenerateCELFromMethod creates a CEL expression based on the evaluation
// method type, description, and evidence requirements.
func GenerateCELFromMethod(
	method gemara.AcceptedMethod,
	evidenceReq string,
	params map[string]interface{},
	templates map[string]string,
) (string, []string, error) {
	// Infer attestation type from evidence requirements
	attestationType := InferAttestationType(evidenceReq)
	attestationTypes := []string{}
	if attestationType != "" {
		attestationTypes = append(attestationTypes, attestationType)
	}

	// Try to determine appropriate template based on evidence requirements
	templateName := selectTemplateFromEvidence(evidenceReq)
	if templateName == "" {
		// Fall back to method type mapping
		if defaultTemplate, ok := MethodTypeToCELTemplate[method.Type]; ok {
			templateName = defaultTemplate
		}
	}

	// Get the template
	templateStr, ok := templates[templateName]
	if !ok {
		// Generate a basic CEL expression as fallback
		return generateBasicCEL(attestationType, evidenceReq, params)
	}

	// Generate CEL from template
	cel, err := GenerateCEL(templateStr, params)
	if err != nil {
		return "", attestationTypes, fmt.Errorf("failed to generate CEL: %w", err)
	}

	return cel, attestationTypes, nil
}

// selectTemplateFromEvidence analyzes evidence requirements to select
// an appropriate CEL template.
func selectTemplateFromEvidence(evidenceReq string) string {
	lowerReq := strings.ToLower(evidenceReq)

	// SLSA provenance patterns
	if strings.Contains(lowerReq, "slsa") || strings.Contains(lowerReq, "provenance") {
		if strings.Contains(lowerReq, "builder") {
			return "slsa-provenance-builder"
		}
		if strings.Contains(lowerReq, "material") {
			return "slsa-provenance-materials"
		}
		if strings.Contains(lowerReq, "buildtype") || strings.Contains(lowerReq, "build type") {
			return "slsa-provenance-buildtype"
		}
	}

	// SBOM patterns
	if strings.Contains(lowerReq, "sbom") || strings.Contains(lowerReq, "software bill of materials") {
		if strings.Contains(lowerReq, "spdx") {
			return "sbom-spdx"
		}
		if strings.Contains(lowerReq, "cyclonedx") {
			return "sbom-cyclonedx"
		}
		return "sbom-present"
	}

	// Vulnerability scan patterns
	if strings.Contains(lowerReq, "vulnerabilit") || strings.Contains(lowerReq, "cve") {
		if strings.Contains(lowerReq, "critical") || strings.Contains(lowerReq, "no critical") {
			return "vulnerability-scan-no-critical"
		}
		if strings.Contains(lowerReq, "threshold") {
			return "vulnerability-scan-threshold"
		}
		if strings.Contains(lowerReq, "scanner") {
			return "vulnerability-scanner"
		}
	}

	return ""
}

// generateBasicCEL creates a basic CEL expression when no template matches.
func generateBasicCEL(attestationType, evidenceReq string, params map[string]interface{}) (string, []string, error) {
	attestationTypes := []string{}
	if attestationType != "" {
		attestationTypes = append(attestationTypes, attestationType)
		// Basic predicate type check
		cel := fmt.Sprintf(`attestation.predicateType == "%s"`, attestationType)
		return cel, attestationTypes, nil
	}

	// Generic placeholder CEL expression
	cel := `true /* TODO: Implement verification logic based on: ` + evidenceReq + ` */`
	return cel, attestationTypes, nil
}

// ScopeFilterToCEL converts Gemara scope dimensions to CEL filtering expressions.
func ScopeFilterToCEL(dimensions gemara.Dimensions) string {
	var filters []string

	// Convert technologies to CEL filter
	if len(dimensions.Technologies) > 0 {
		techList := make([]string, len(dimensions.Technologies))
		for i, tech := range dimensions.Technologies {
			// Normalize technology names to lowercase with hyphens
			normalized := strings.ToLower(strings.ReplaceAll(tech, " ", "-"))
			techList[i] = fmt.Sprintf(`"%s"`, normalized)
		}
		filters = append(filters, fmt.Sprintf(`subject.type in [%s]`, strings.Join(techList, ", ")))
	}

	// Convert geopolitical regions to CEL filter
	if len(dimensions.Geopolitical) > 0 {
		regionList := make([]string, len(dimensions.Geopolitical))
		for i, region := range dimensions.Geopolitical {
			// Normalize regions to lowercase codes
			normalized := normalizeRegion(region)
			regionList[i] = fmt.Sprintf(`"%s"`, normalized)
		}
		filters = append(filters, fmt.Sprintf(`subject.annotations.region in [%s]`, strings.Join(regionList, ", ")))
	}

	// Convert sensitivity levels to CEL filter
	if len(dimensions.Sensitivity) > 0 {
		sensitivityList := make([]string, len(dimensions.Sensitivity))
		for i, sensitivity := range dimensions.Sensitivity {
			normalized := strings.ToLower(sensitivity)
			sensitivityList[i] = fmt.Sprintf(`"%s"`, normalized)
		}
		filters = append(filters, fmt.Sprintf(`subject.annotations.classification in [%s]`, strings.Join(sensitivityList, ", ")))
	}

	// Convert user groups to CEL filter
	if len(dimensions.Groups) > 0 {
		groupList := make([]string, len(dimensions.Groups))
		for i, group := range dimensions.Groups {
			groupList[i] = fmt.Sprintf(`"%s"`, group)
		}
		filters = append(filters, fmt.Sprintf(`subject.annotations.group in [%s]`, strings.Join(groupList, ", ")))
	}

	if len(filters) == 0 {
		return ""
	}

	// Combine all filters with AND
	return strings.Join(filters, " && ")
}

// normalizeRegion converts region names to lowercase codes.
func normalizeRegion(region string) string {
	// Simple normalization - could be expanded with a full mapping
	regionMap := map[string]string{
		"united states":  "us",
		"european union": "eu",
		"canada":         "ca",
		"united kingdom": "uk",
		"california":     "us-ca",
	}

	lower := strings.ToLower(region)
	if code, ok := regionMap[lower]; ok {
		return code
	}
	return lower
}

// CombineCELExpressions combines multiple CEL expressions with a logical operator.
func CombineCELExpressions(expressions []string, operator string) string {
	if len(expressions) == 0 {
		return ""
	}
	if len(expressions) == 1 {
		return expressions[0]
	}

	// Wrap each expression in parentheses if using AND/OR
	if operator == "&&" || operator == "||" {
		wrapped := make([]string, len(expressions))
		for i, expr := range expressions {
			wrapped[i] = "(" + expr + ")"
		}
		return strings.Join(wrapped, " " + operator + " ")
	}

	return strings.Join(expressions, " "+operator+" ")
}
