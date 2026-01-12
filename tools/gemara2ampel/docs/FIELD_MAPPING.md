# Field Mapping: Gemara Layer-3 to Ampel Policy

This document describes the complete field mapping from Gemara Layer-3 policy documents to Ampel verification policies.

## Overview

Gemara Layer-3 policies are organization-specific governance documents that define risk-informed rules. Ampel policies are supply chain verification policies that use CEL (Common Expression Language) to verify in-toto attestations.

## Top-Level Policy Mapping

| Gemara Field | Ampel Field | Transformation | Notes |
|--------------|-------------|----------------|-------|
| `title` | `name` | Direct copy | Policy identifier |
| `metadata.description` | `description` | Direct copy | Policy purpose |
| `metadata.version` | `version` | Direct copy | Semantic version |
| N/A | `rule` | Default: `"all(tenets)"` | Overall evaluation rule |
| `adherence.assessment-plans[]` | `tenets[]` | Transform (see below) | One-to-many mapping |
| `imports.policies[]` | `imports[]` | Direct copy | Policy references |
| Multiple fields | `metadata{}` | Aggregate (see below) | Flattened metadata map |

## Metadata Field Mapping

Gemara metadata and contact information is flattened into the Ampel `metadata` map:

| Gemara Field | Ampel Metadata Key | Example Value |
|--------------|-------------------|---------------|
| `metadata.id` | `policy-id` | `"policy-001"` |
| `metadata.author.name` | `author` | `"Security Team"` |
| `metadata.author.id` | `author-id` | `"security-team"` |
| `contacts.responsible[].name` | `responsible` | `"IT Director, Compliance Officer"` (comma-separated) |
| `contacts.accountable[].name` | `accountable` | `"CISO"` (comma-separated) |
| `scope.in.technologies[]` | `scope-technologies` | `"Cloud Computing, Web Applications"` |
| `scope.in.geopolitical[]` | `scope-regions` | `"United States, European Union"` |
| `imports.catalogs[].reference-id` | `catalog-references` | `"NIST-800-53, ISO-27001"` |
| `imports.guidance[].reference-id` | `guidance-references` | `"CIS-CONTROLS"` |

**Note:** RACI contacts (Responsible, Accountable, Consulted, Informed) are concatenated with comma separators.

## Assessment Plan to Tenet Mapping

Each **automated** evaluation method in an assessment plan generates one Ampel tenet:

### Tenet Identification

| Gemara Field | Ampel Field | Transformation |
|--------------|-------------|----------------|
| `assessment-plans[].requirement-id` + `assessment-plans[].id` + method index | `tenets[].id` | Format: `"{requirement-id}-{plan-id}-{index}"` |
| `assessment-plans[].evaluation-methods[].description` | `tenets[].name` | Direct copy or generated from evidence |
| `assessment-plans[].evidence-requirements` + catalog lookup | `tenets[].description` | Combined text |

**Example:**
```yaml
# Gemara
requirement-id: "SC-01.01"
id: "slsa-check"
# Index: 0 (first automated method)

# Ampel
id: "SC-01.01-slsa-check-0"
```

### Tenet CEL Code Generation

The `code` field contains a CEL expression generated from multiple Gemara fields:

| Gemara Source | CEL Component | Example |
|---------------|---------------|---------|
| `evidence-requirements` (keyword matching) | Predicate type check | `attestation.predicateType == "https://slsa.dev/provenance/v1"` |
| `evidence-requirements` (pattern matching) | Specific field checks | `attestation.predicate.builder.id == "..."` |
| `parameters[].accepted-values[]` | Value constraints | Builder ID from parameters |
| `scope.in.*` (if enabled) | Scope filters | `subject.type in ["cloud-app"]` |

**CEL Generation Logic:**

1. **Attestation Type Inference** (from `evidence-requirements`):
   - Keywords `"slsa"`, `"provenance"` → `https://slsa.dev/provenance/v1`
   - Keywords `"sbom"`, `"spdx"` → `https://spdx.dev/Document`
   - Keywords `"cyclonedx"` → `https://cyclonedx.org/bom`
   - Keywords `"vulnerability"`, `"cve"` → `https://in-toto.io/Statement/v0.1`

2. **Template Selection** (from `evidence-requirements`):
   - `"builder"` → `slsa-provenance-builder` template
   - `"materials"` → `slsa-provenance-materials` template
   - `"sbom"` → `sbom-present` template
   - `"critical"`, `"no critical"` → `vulnerability-scan-no-critical` template

3. **Parameter Substitution** (from `parameters[]`):
   - First `accepted-values` becomes default parameter value
   - All `accepted-values` formatted as CEL list for `in` operator

### Attestation Types

| Gemara Field | Ampel Field | Transformation |
|--------------|-------------|----------------|
| Inferred from `evidence-requirements` | `tenets[].attestationTypes[]` | List of predicate type URLs |

### Parameters

| Gemara Field | Ampel Field | Transformation |
|--------------|-------------|----------------|
| `parameters[].id` | `tenets[].parameters.{id}` | Key-value map |
| `parameters[].accepted-values[0]` | Value | First accepted value as default |

## Evaluation Method Filtering

Only certain evaluation method types are converted to automated tenets:

| Method Type | Included in Ampel? | Rationale |
|-------------|-------------------|-----------|
| `automated` | ✅ Yes | Directly automatable with CEL |
| `gate` | ✅ Yes | Pre-deployment checks |
| `behavioral` | ✅ Yes | Runtime verification |
| `autoremediation` | ✅ Yes | Post-verification actions |
| `manual` | ❌ No | Cannot be automated |

## Scope to CEL Filter Mapping

When `WithScopeFilters(true)` is enabled, scope dimensions are converted to CEL filters:

| Gemara Scope Dimension | CEL Filter Pattern | Example |
|------------------------|-------------------|---------|
| `scope.in.technologies[]` | `subject.type in [...]` | `subject.type in ["cloud-app", "web-app"]` |
| `scope.in.geopolitical[]` | `subject.annotations.region in [...]` | `subject.annotations.region in ["us", "eu"]` |
| `scope.in.sensitivity[]` | `subject.annotations.classification in [...]` | `subject.annotations.classification in ["confidential"]` |
| `scope.in.groups[]` | `subject.annotations.group in [...]` | `subject.annotations.group in ["engineering"]` |

**Region Normalization:**
- `"United States"` → `"us"`
- `"European Union"` → `"eu"`
- `"Canada"` → `"ca"`
- `"United Kingdom"` → `"uk"`

**Technology Normalization:**
- `"Cloud Computing"` → `"cloud-computing"`
- `"Web Applications"` → `"web-applications"`
- Spaces replaced with hyphens, lowercase

## Fields Not Mapped

The following Gemara fields are **not mapped** to Ampel policies:

| Gemara Field | Reason |
|--------------|--------|
| `implementation-plan` | Ampel focuses on verification, not implementation timelines |
| `risks.mitigated` | Risk management is policy-level, not verification-level |
| `risks.accepted` | Risk acceptance is organizational decision, not technical verification |
| `adherence.evaluation-methods` (top-level) | Only plan-specific methods are used |
| `adherence.enforcement-methods` | Ampel doesn't model enforcement actions |
| `adherence.non-compliance` | Policy metadata only, added to Ampel metadata if needed |
| `contacts.consulted` | Not directly relevant to automated verification |
| `contacts.informed` | Not directly relevant to automated verification |
| `scope.out` | Ampel uses positive assertions; exclusions not modeled |

## Complete Example Mapping

### Input (Gemara Layer-3 YAML)

```yaml
title: "Supply Chain Security Policy"
metadata:
  id: "policy-001"
  version: "1.0.0"
  description: "Verify software supply chain security"
  author:
    id: "security-team"
    name: "Security Team"

contacts:
  responsible:
    - name: "DevOps Manager"
  accountable:
    - name: "CISO"

scope:
  in:
    technologies: ["Container Images"]
    geopolitical: ["United States"]

imports:
  catalogs:
    - reference-id: "SLSA-CONTROLS"

adherence:
  assessment-plans:
    - id: "slsa-prov-check"
      requirement-id: "SC-01.01"
      frequency: "every build"
      evaluation-methods:
        - type: "automated"
          description: "Verify SLSA provenance"
      evidence-requirements: "SLSA provenance with trusted builder"
      parameters:
        - id: "builder-id"
          label: "Trusted Builder"
          accepted-values:
            - "https://github.com/actions/runner"
```

### Output (Ampel Policy JSON)

```json
{
  "name": "Supply Chain Security Policy",
  "description": "Verify software supply chain security",
  "version": "1.0.0",
  "metadata": {
    "policy-id": "policy-001",
    "author": "Security Team",
    "author-id": "security-team",
    "responsible": "DevOps Manager",
    "accountable": "CISO",
    "scope-technologies": "Container Images",
    "scope-regions": "United States",
    "catalog-references": "SLSA-CONTROLS"
  },
  "imports": [],
  "tenets": [
    {
      "id": "SC-01.01-slsa-prov-check-0",
      "name": "Verify SLSA provenance",
      "description": "SLSA provenance with trusted builder",
      "code": "attestation.predicateType == \"https://slsa.dev/provenance/v1\" && attestation.predicate.builder.id == \"https://github.com/actions/runner\"",
      "attestationTypes": [
        "https://slsa.dev/provenance/v1"
      ],
      "parameters": {
        "builder-id": "https://github.com/actions/runner"
      }
    }
  ],
  "rule": "all(tenets)"
}
```

## Mapping with Scope Filters

When using `WithScopeFilters(true)`:

### Input (Same as above)

### Output (with scope filters in CEL)

```json
{
  "tenets": [
    {
      "id": "SC-01.01-slsa-prov-check-0",
      "name": "Verify SLSA provenance",
      "code": "(subject.type in [\"container-images\"] && subject.annotations.region in [\"us\"]) && (attestation.predicateType == \"https://slsa.dev/provenance/v1\" && attestation.predicate.builder.id == \"https://github.com/actions/runner\")",
      "attestationTypes": ["https://slsa.dev/provenance/v1"]
    }
  ]
}
```

## Mapping with Catalog Enrichment

When using `WithCatalog(catalog)`:

### Input Catalog

```yaml
# catalog.yaml
controls:
  - id: "CTRL-01"
    assessment-requirements:
      - id: "SC-01.01"
        text: "Verify build provenance is present and contains builder identity"
```

### Output (with enriched description)

```json
{
  "tenets": [
    {
      "id": "SC-01.01-slsa-prov-check-0",
      "description": "Verify build provenance is present and contains builder identity - SLSA provenance with trusted builder"
    }
  ]
}
```

## CEL Template Library Mappings

The following templates are used for CEL generation:

| Template Name | Triggered By (Evidence Keywords) | Generated CEL |
|---------------|----------------------------------|---------------|
| `slsa-provenance-builder` | "slsa", "provenance", "builder" | `attestation.predicateType == "https://slsa.dev/provenance/v1" && attestation.predicate.builder.id == "{{.BuilderId}}"` |
| `slsa-provenance-materials` | "slsa", "material" | `attestation.predicateType == "https://slsa.dev/provenance/v1" && all(attestation.predicate.materials, m, m.digest.sha256 != "")` |
| `slsa-provenance-buildtype` | "buildtype", "build type" | `attestation.predicateType == "https://slsa.dev/provenance/v1" && attestation.predicate.buildType == "{{.BuildType}}"` |
| `sbom-present` | "sbom" (generic) | `attestation.predicateType == "https://spdx.dev/Document" \|\| attestation.predicateType == "https://cyclonedx.org/bom"` |
| `sbom-spdx` | "spdx" | `attestation.predicateType == "https://spdx.dev/Document"` |
| `sbom-cyclonedx` | "cyclonedx" | `attestation.predicateType == "https://cyclonedx.org/bom"` |
| `vulnerability-scan-no-critical` | "critical", "no critical" | `attestation.predicateType == "https://in-toto.io/Statement/v0.1" && attestation.predicate.scanner.result.summary.critical == 0` |
| `vulnerability-scan-threshold` | "threshold" | `attestation.predicateType == "https://in-toto.io/Statement/v0.1" && attestation.predicate.scanner.result.summary.critical == 0 && attestation.predicate.scanner.result.summary.high < {{.MaxHigh}}` |
| `vulnerability-scanner` | "scanner" | `attestation.predicateType == "https://in-toto.io/Statement/v0.1" && attestation.predicate.scanner.vendor == "{{.Scanner}}"` |

## Implementation Details

### Assessment Plan Iteration

```
FOR EACH assessment-plan IN policy.adherence.assessment-plans:
  FOR EACH method IN assessment-plan.evaluation-methods:
    IF method.type IN ["automated", "gate", "behavioral", "autoremediation"]:
      CREATE Tenet:
        id = "{requirement-id}-{plan-id}-{method-index}"
        name = method.description OR evidence-requirements
        description = catalog-lookup(requirement-id) + evidence-requirements
        code = generate_cel(method, evidence-requirements, parameters)
        attestationTypes = infer_types(evidence-requirements)
        parameters = map_parameters(plan.parameters)
```

### CEL Generation Algorithm

```
FUNCTION generate_cel(method, evidence_req, parameters):
  1. attestation_type = infer_type(evidence_req)
  2. template_name = select_template(evidence_req)
  3. template = get_template(template_name)
  4. params = extract_params(parameters, evidence_req)
  5. cel = render_template(template, params)
  6. IF scope_filters_enabled:
       scope_cel = generate_scope_filter(policy.scope.in)
       cel = "({scope_cel}) && ({cel})"
  7. RETURN cel
```

## Field Cardinality

| Mapping | Cardinality | Notes |
|---------|-------------|-------|
| Policy → Ampel Policy | 1:1 | One Gemara policy creates one Ampel policy |
| Assessment Plan → Tenet | 1:N | One plan can create multiple tenets (one per automated method) |
| Evaluation Method → Tenet | 1:1 or 1:0 | Only automated methods create tenets |
| Parameter → Tenet Parameter | 1:1 | Direct mapping |
| Evidence Requirement → CEL Code | 1:1 | Transformed via templates |
| Scope Dimension → CEL Filter | 1:1 | Each dimension creates one filter |

## Validation Rules

After transformation, the following validations are performed:

1. **Policy must have a name** (`policy.name != ""`)
2. **Policy must have at least one tenet** (`len(policy.tenets) > 0`)
3. **Policy must have a rule** (`policy.rule != ""`)
4. **Each tenet must have an ID** (`tenet.id != ""`)
5. **Each tenet must have a name** (`tenet.name != ""`)
6. **Each tenet must have CEL code** (`tenet.code != ""`)

If any assessment plan produces zero automated methods, no tenet is created, which may cause validation failure if no other plans exist.

## Usage in Code

```go
import "gemara2ampel/go/ampel"

// Basic transformation
ampelPolicy, err := ampel.FromPolicy(gemaraPolicy)

// With catalog enrichment
ampelPolicy, err := ampel.FromPolicy(gemaraPolicy,
    ampel.WithCatalog(catalog))

// With scope filters
ampelPolicy, err := ampel.FromPolicy(gemaraPolicy,
    ampel.WithScopeFilters(true))

// With custom CEL templates
templates := map[string]string{
    "custom-check": `attestation.predicate.custom == "{{.Value}}"`,
}
ampelPolicy, err := ampel.FromPolicy(gemaraPolicy,
    ampel.WithCELTemplates(templates))
```

## References

- **Gemara Layer-3 Schema**: https://github.com/ossf/gemara/blob/main/schemas/layer-3.cue
- **Ampel Policy Spec**: https://github.com/carabiner-dev/ampel
- **in-toto Attestations**: https://github.com/in-toto/attestation
- **SLSA Provenance**: https://slsa.dev/provenance/v1
- **CEL Language**: https://github.com/google/cel-spec
