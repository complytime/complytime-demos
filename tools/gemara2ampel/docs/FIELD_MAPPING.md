# Field Mapping: Gemara Layer-3 to Ampel Policy

This document describes the complete field mapping from Gemara Layer-3 policy documents to Ampel verification policies.

**Schema Versions:**
- Gemara: Latest from main branch (github.com/gemaraproj/go-gemara)
- Ampel: github.com/carabiner-dev/ampel
- Ampel Policy API: github.com/carabiner-dev/policy/api/v1 (official format)

## Overview

Gemara Layer-3 policies are organization-specific governance documents that define risk-informed rules. Ampel policies are supply chain verification policies that use CEL (Common Expression Language) to verify in-toto attestations.

## Top-Level Policy Mapping

### Single Policy Transformation

The output follows the [official Ampel Policy API format](https://github.com/carabiner-dev/policy/tree/main/api/v1):

| Gemara Field | Ampel Field | Transformation | Notes |
| ------------ | ----------- | -------------- | ----- |
| `metadata.id` | `id` | Direct copy | Policy identifier (required) |
| N/A | `meta.runtime` | Default: `"cel@v14.0"` | CEL runtime version |
| `metadata.description` | `meta.description` | Direct copy | Policy purpose |
| `metadata.version` | `meta.version` | Parse to int64 | Major version only (e.g., "1.0.0" → 1) |
| N/A | `meta.assert_mode` | Default: `"AND"` | "AND" (all tenets) or "OR" (any tenet) |
| N/A | `meta.enforce` | Default: `"ON"` | "ON", "OFF", or "WARN" |
| `adherence.assessment-plans[]` | `tenets[]` | Transform (see below) | One-to-many mapping |
| `adherence.assessment-plans[].parameters[]` | `context{}` | Transform (see below) | Parameters → ContextVal entries |
| N/A | `identities[]` | Not currently mapped | Valid signer identities (for future use) |
| N/A | `predicates` | Not currently mapped | Policy-level predicate specification (for future use) |

### Additional Policy Fields

The official Ampel policy format includes additional top-level fields:

**Context:** `map[string]*ContextVal` ✅ **Populated from Gemara parameters**
- Maps Gemara assessment plan parameters to runtime context values
- Each parameter becomes a ContextVal with type, required flag, default value, and description
- Example: `{"builder-id": {"type": "string", "required": false, "value": "...", "default": "...", "description": "Expected builder identity"}}`
- ContextVal fields: `type`, `required`, `default`, `value`, `description`
- See "Parameter to Context Mapping" section below for details

**Identities:** `[]*Identity`
- Defines valid signer identities for attestations
- Supports exact matching or regex patterns
- Example: `[{"type": "exact", "issuer": "https://accounts.google.com", "identity": "builder@example.com"}]`
- Identity fields: `type`, `issuer`, `identity`, `publicKey`

**Predicates:** `*PredicateSpec`
- Policy-level specification of which attestation types to evaluate
- Can set limit on number of predicates to load
- Example: `{"types": ["https://slsa.dev/provenance/v1"], "limit": 10}`
- PredicateSpec fields: `types[]`, `limit`

### PolicySet Structure

When transforming multiple policies or policies with imports, an Ampel **PolicySet** is generated:

| Gemara Source | PolicySet Field | Transformation | Notes |
| ------------- | --------------- | -------------- | ----- |
| Custom or policy metadata | `id` | Configurable or from main policy | PolicySet identifier |
| Custom or policy metadata | `meta.description` | Configurable or from main policy | PolicySet description |
| Custom or policy metadata | `meta.version` | Parse to int64 | PolicySet version as integer (e.g., "1.0.0" → 1) |
| Policy(ies) + imports | `policies[]` | Array of Policy | Contains inline and/or external policies |

### Policy Structure in PolicySet

| Policy Field | Type | Description |
| ------------ | ---- | ----------- |
| `id` | string | Policy identifier within the set |
| `tenets[]` | Tenet[] (optional) | Inline policy tenets (if inline policy) |
| `meta` | Meta (optional) | Policy metadata (if inline policy) |
| `context` | map[string]*ContextVal (optional) | Contextual data (if inline policy) |
| `source` | PolicyRef (optional) | External policy reference with id and location |
| `source.id` | string | Policy identifier for external reference |
| `source.location.uri` | string | External policy URI (e.g., `git+https://github.com/org/repo#path/to/policy.json`) |

**When to use inline vs. external:**
- **Inline policy**: Includes `tenets` array with full tenet definitions
- **External reference**: Includes `source` field with `PolicyRef` containing `id` and `location`

**PolicySetMeta Fields:**

| Field | Type | Description |
| ----- | ---- | ----------- |
| `description` | string | PolicySet description |
| `version` | int64 | PolicySet version number (parsed from version string) |

**Meta Fields (for inline policies):**

| Field | Description |
| ----- | ----------- |
| `runtime` | Runtime identifier (e.g., "cel@v14.0") |
| `description` | Policy description |
| `assert_mode` | "AND" or "OR" (note: snake_case) |
| `version` | Integer version number (int64) |
| `controls[]` | Array of Control objects |
| `enforce` | Enforcement mode: "ON", "OFF", "WARN" |

**Control Fields:**

| Field | Description |
| ----- | ----------- |
| `id` | Control identifier (e.g., "LEVEL_3") |
| `title` | Human-readable control name |
| `framework` | Compliance framework name (e.g., "SLSA", "NIST") |
| `class` | Category within framework (e.g., "BUILD") |
| `item` | Optional sub-item within the control |

## Metadata Field Mapping

### Policy Metadata

The official Ampel policy format uses a simplified metadata structure. Gemara policy metadata fields are mapped as follows:

| Gemara Field | Ampel Field | Transformation | Notes |
| ------------ | ----------- | -------------- | ----- |
| `metadata.id` | `id` | Direct copy | Policy identifier |
| `metadata.description` | `meta.description` | Direct copy | Policy description |
| `metadata.version` | `meta.version` | Parse major version | String "1.0.0" → int64 1 |
| N/A | `meta.runtime` | Default value | Always "cel@v14.0" |
| N/A | `meta.assert_mode` | From options | "AND" or "OR" |
| N/A | `meta.enforce` | Optional | "ON", "OFF", or "WARN" |

**Note:** The following Gemara metadata fields are **not mapped** to Ampel policies:
- `metadata.author.*` - Author information not included in official Ampel format
- `contacts.*` - RACI contacts not included in official Ampel format
- `scope.*` - Scope information not included in metadata (may be used in CEL filters if enabled)
- `imports.*` - Import references not included in metadata (handled via PolicySet)

These fields contain valuable organizational context but are not part of the verification policy structure.

## Assessment Plan to Tenet Mapping

Each **automated** evaluation method in an assessment plan generates one Ampel tenet. Only methods with type `"automated"`, `"gate"`, `"behavioral"`, or `"autoremediation"` are mapped.

### Tenet Identification

| Gemara Field | Ampel Field | Transformation | Notes |
| ------------ | ----------- | -------------- | ----- |
| `assessment-plans[].requirement-id` + `assessment-plans[].id` + method index | `tenets[].id` | Format: `"{requirement-id}-{plan-id}-{method-index}"` | Unique tenet identifier |
| `assessment-plans[].evaluation-methods[].description` | `tenets[].title` | Direct copy, or generated from `evidence-requirements` if empty | Human-readable name |
| N/A | `tenets[].runtime` | Default: `"cel@v14.0"` | Runtime identifier |
| Inferred from `evidence-requirements` | `tenets[].predicates` | PredicateSpec object | Attestation types to evaluate |
| N/A | `tenets[].error` | Not currently mapped | Error messaging (for future use) |
| N/A | `tenets[].assessment` | Not currently mapped | Assessment results (runtime field) |

**Tenet ID Generation:**
The method index starts at 0 and only counts **automated** methods (manual methods are skipped).

**Example:**
```yaml
# Gemara
requirement-id: "SC-01.01"
id: "slsa-check"
# Method index: 0 (first automated method)

# Ampel
id: "SC-01.01-slsa-check-0"
```

### Tenet Sub-structures

**PredicateSpec:**
Defines which attestation predicate types this tenet evaluates:

| Field | Type | Description |
| ----- | ---- | ----------- |
| `types[]` | string[] | List of predicate type URIs (e.g., "https://slsa.dev/provenance/v1") |
| `limit` | int32 | Maximum number of predicates to load (optional) |

**Error:**
Defines error messaging for failed tenets (not currently populated):

| Field | Type | Description |
| ----- | ---- | ----------- |
| `message` | string | Error message to display |
| `guidance` | string | Additional context or remediation steps |

**Assessment:**
Contains tenet assessment results (runtime field, not populated during transformation):

| Field | Type | Description |
| ----- | ---- | ----------- |
| `message` | string | Assessment message |

### Tenet CEL Code Generation

The `code` field contains a CEL expression generated from multiple Gemara fields:

| Gemara Source | CEL Component | Example |
| ------------- | ------------- | ------- |
| `evidence-requirements` (keyword matching) | Predicate type check | `attestation.predicateType == "https://slsa.dev/provenance/v1"` |
| `evidence-requirements` (pattern matching) | Specific field checks | `attestation.predicate.builder.id == "..."` |
| `parameters[].accepted-values[]` | Value constraints | Builder ID from parameters |
| `scope.in.*` (if scope filters enabled) | Scope filters | `subject.type in ["cloud-app"]` |

## Parameter to Context Mapping

Gemara assessment plan parameters are mapped to Ampel Policy.Context as ContextVal entries. All unique parameters from all assessment plans are collected and converted.

### Parameter Field Mapping

| Gemara Field | Ampel Field | Transformation | Notes |
| ------------ | ----------- | -------------- | ----- |
| `parameters[].id` | `context.{id}` | Key in context map | Preserves original parameter ID (including hyphens) |
| `parameters[].description` | `context.{id}.description` | Direct copy | Falls back to `label` if `description` is empty |
| `parameters[].accepted-values[0]` | `context.{id}.value` | First accepted value | Default runtime value |
| `parameters[].accepted-values[0]` | `context.{id}.default` | First accepted value | Default if not provided at runtime |
| Inferred from `accepted-values` | `context.{id}.type` | Default: `"string"` | Currently all parameters use type "string" |
| Inferred from `accepted-values` | `context.{id}.required` | `true` if no accepted values, `false` otherwise | Runtime-only parameters are required |

### Parameter Types

**1. Static/Default Parameters** (with `accepted-values`):
```yaml
# Gemara
parameters:
  - id: builder-id
    description: Expected SLSA builder ID
    accepted-values:
      - https://github.com/actions/runner
```

**Ampel Context:**
```json
{
  "builder-id": {
    "type": "string",
    "required": false,
    "value": "https://github.com/actions/runner",
    "default": "https://github.com/actions/runner",
    "description": "Expected SLSA builder ID"
  }
}
```

**2. Runtime-Only Parameters** (no `accepted-values`):
```yaml
# Gemara
parameters:
  - id: runtime-threshold
    description: Threshold value provided at runtime
```

**Ampel Context:**
```json
{
  "runtime-threshold": {
    "type": "string",
    "required": true,
    "value": null,
    "default": null,
    "description": "Threshold value provided at runtime"
  }
}
```

### Multi-Value Parameters

When a parameter has multiple `accepted-values`, the allowed values are compiled into the CEL expression as hardcoded validation constraints. The context stores only the first value as the default.

**Gemara:**
```yaml
parameters:
  - id: scanner
    description: Approved vulnerability scanner
    accepted-values:
      - trivy
      - grype
```

**Ampel Context (stores default):**
```json
{
  "scanner": {
    "type": "string",
    "required": false,
    "value": "trivy",
    "default": "trivy",
    "description": "Approved vulnerability scanner"
  }
}
```

**Generated CEL (hardcoded constraint):**
```cel
attestation.predicate.scanner.vendor in ["trivy", "grype"]
```

**Design Rationale:**
- Context stores the **default runtime value** (single string, type "string")
- CEL expression encodes the **validation constraint** (allowed list)
- Allowed values are compiled at policy generation time
- Runtime can override context with any value (CEL validates against allowed list)

### CEL Context References

Parameters are referenced in CEL expressions using the `context["param-id"]` syntax:

**Example CEL:**
```cel
attestation.predicate.builder.id == context["builder-id"]
```

### Tenet Structure with Context

**Complete Example:**
```json
{
  "id": "slsa-policy",
  "context": {
    "builder-id": {
      "type": "string",
      "required": false,
      "value": "https://github.com/actions/runner",
      "default": "https://github.com/actions/runner",
      "description": "Expected SLSA builder ID"
    }
  },
  "tenets": [
    {
      "id": "SC-01.01-slsa-check-0",
      "title": "Verify SLSA provenance",
      "runtime": "cel@v14.0",
      "predicates": {
        "types": ["https://slsa.dev/provenance/v1"]
      },
      "code": "attestation.predicateType == \"https://slsa.dev/provenance/v1\" && attestation.predicate.builder.id == context[\"builder-id\"]"
    }
  ]
}
```

**Notes:**
- `predicates.types[]` filters which attestations this tenet evaluates
- `context` provides runtime-configurable parameters
- CEL `code` references context using `context["param-id"]` syntax
- Parameter values can be provided/overridden at policy evaluation time

## Evaluation Method Filtering

In go-gemara v0.5.0, evaluation methods have two separate fields:
- **`Type`** (`MethodType`): The category of evaluation — `Behavioral`, `Intent`, `Remediation`, `Gate`
- **`Mode`** (`ModeType`): Whether the method is `Manual` or `Automated`

Only methods with `Mode: Automated` are converted to Ampel tenets. The `Type` field
determines which CEL template is used:

| Mode | Included in Ampel? | Rationale |
| ---- | ------------------ | --------- |
| `Automated` | Yes | Can be expressed as CEL verification logic |
| `Manual` | No | Requires human evaluation |

| Method Type | Default CEL Template | Description |
| ----------- | -------------------- | ----------- |
| `Gate` | `generic-predicate-type` | Pre-deployment pass/fail checks |
| `Behavioral` | `generic-field-equals` | Runtime behavior verification |
| `Intent` | `generic-field-equals` | Intent/state verification |
| `Remediation` | `generic-field-equals` | Post-verification remediation actions |

## Scope to CEL Filter Mapping

Scope dimensions can be converted to CEL filters that are prepended to tenet verification code:

### Supported Scope Dimensions

| Gemara Scope Dimension | CEL Filter Pattern | Example | Normalization |
| ---------------------- | ------------------ | ------- | ------------- |
| `scope.in.technologies[]` | `subject.type in [...]` | `subject.type in ["cloud-computing", "web-applications"]` | Lowercase, spaces→hyphens |
| `scope.in.geopolitical[]` | `subject.annotations.region in [...]` | `subject.annotations.region in ["us", "eu"]` | Region codes (see below) |
| `scope.in.sensitivity[]` | `subject.annotations.classification in [...]` | `subject.annotations.classification in ["confidential", "secret"]` | Lowercase |
| `scope.in.groups[]` | `subject.annotations.group in [...]` | `subject.annotations.group in ["engineering"]` | No normalization |

**Note:** The `scope.in.users[]` dimension is **not** currently mapped to CEL filters.

### Normalization Rules

| Dimension | Normalization | Examples |
| --------- | ------------- | -------- |
| **technologies** | Lowercase, spaces→hyphens | `"Cloud Computing"` → `"cloud-computing"` |
| **geopolitical** | ISO-style codes | `"United States"` → `"us"`, `"European Union"` → `"eu"`, `"Canada"` → `"ca"`, `"United Kingdom"` → `"uk"` |
| **sensitivity** | Lowercase | `"Confidential"` → `"confidential"` |
| **groups** | No normalization | Used as-is |

## Fields Not Mapped

The following Gemara Layer-3 fields are **not mapped** to Ampel policies:

### Policy-Level Fields

| Gemara Field | Reason |
| ------------ | ------ |
| `implementation-plan` | Ampel focuses on verification, not implementation timelines or rollout schedules |
| `implementation-plan.notification-process` | Implementation detail, not verification logic |
| `implementation-plan.evaluation-timeline` | Timeline is organizational, not technical verification |
| `implementation-plan.enforcement-timeline` | Timeline is organizational, not technical verification |
| `risks` | Risk management is policy-level context, not verification logic |
| `risks.mitigated[]` | Risk mappings are organizational context |
| `risks.accepted[]` | Risk acceptance is organizational decision, not technical verification |
| `adherence.evaluation-methods` (top-level) | Only assessment-plan-specific methods are transformed |
| `adherence.enforcement-methods[]` | Ampel doesn't model enforcement actions or remediation workflows |
| `adherence.non-compliance` | Non-compliance handling is organizational policy, not verification rule |

### Contact Fields

| Gemara Field | Reason |
| ------------ | ------ |
| `contacts.responsible[]` | Not included in official Ampel format; organizational context only |
| `contacts.accountable[]` | Not included in official Ampel format; organizational context only |
| `contacts.consulted[]` | Not included in official Ampel format; organizational context only |
| `contacts.informed[]` | Not included in official Ampel format; organizational context only |

### Scope Fields

| Gemara Field | Reason |
| ------------ | ------ |
| `scope.out` | Ampel uses positive assertions (what to verify); exclusions are not modeled |
| `scope.in.users[]` | Not currently mapped to CEL filters (may be added in future versions) |

### Metadata Fields

| Gemara Field | Reason |
| ------------ | ------ |
| `metadata.date` | Not preserved in Ampel policy metadata |
| `metadata.draft` | Status flag not relevant to runtime verification |
| `metadata.lexicon` | Terminology reference not used in verification |
| `metadata.mapping-references[]` | Not transformed (may contain catalog references that are flattened) |
| `metadata.applicability-categories[]` | Not used in verification logic |
| `metadata.author.*` | Author information not included in official Ampel format |

### Import Fields

| Gemara Field | Reason |
| ------------ | ------ |
| `imports.catalogs[].exclusions[]` | Exclusions are processed during policy authoring, not verification |
| `imports.catalogs[].constraints[]` | Constraints modify requirements but aren't directly mapped |
| `imports.catalogs[].assessment-requirement-modifications[]` | Modifications are applied during transformation, not preserved |
| `imports.guidance[].exclusions[]` | Exclusions are processed during policy authoring |
| `imports.guidance[].constraints[]` | Constraints modify guidelines but aren't directly mapped |

### Assessment Plan Fields

| Gemara Field | Reason |
| ------------ | ------ |
| `assessment-plans[].frequency` | Assessment schedule is execution context, not verification logic |
| `evaluation-methods[].actor` | Execution context (who performs assessment), not verification rule |

### Parameter Fields

| Gemara Field | Reason |
| ------------ | ------ |
| `parameters[].label` | Used as fallback for `description` if description is empty, otherwise not mapped |
| `parameters[].accepted-values[1..n]` | Only first value stored as default; remaining values compiled into CEL expression constraints |

**Note:** Many of these fields contain valuable organizational context and governance information, but they are not directly translated into verification rules.

## Evidence Requirements to CEL Template Mapping

CEL code generation is based on keyword detection in the `evidence-requirements` field:

| Evidence Keywords | Attestation Type Inferred | Template Category |
| ----------------- | ------------------------- | ----------------- |
| "slsa", "provenance" | `https://slsa.dev/provenance/v1` | SLSA Provenance |
| "vulnerability", "cve" | `https://in-toto.io/Statement/v0.1` | Vulnerability Scan |

**Specific Template Selection:**
- "builder" → Builder identity verification
- "materials" → Materials verification
- "buildtype" → Build type verification
- "critical", "no critical" → No critical vulnerabilities
- "threshold" → Vulnerability threshold checks
- "scanner" → Scanner vendor verification

## Field Cardinality

| Mapping | Cardinality | Notes |
| ------- | ----------- | ----- |
| Policy → Ampel Policy | 1:1 | One Gemara policy creates one Ampel policy |
| Assessment Plan → Tenet | 1:N | One plan can create multiple tenets (one per automated method) |
| Evaluation Method → Tenet | 1:1 or 1:0 | Only automated methods create tenets |
| Parameter → Context Entry | 1:1 | Each unique parameter ID creates one ContextVal in Policy.Context |
| Parameter (multi-value) → CEL constraint | 1:1 | Multiple accepted-values compiled into single CEL "in" expression |
| Evidence Requirement → CEL Code | 1:1 | Transformed via templates |
| Scope Dimension → CEL Filter | 1:1 | Each dimension creates one filter |

## Design Notes

**Official Ampel Format Compliance:**
The transformation follows the official Ampel Policy API format from `github.com/carabiner-dev/policy/api/v1`. Key design principles:

1. **Minimal Metadata**: Only essential verification metadata is included. Organizational context (author, contacts, scope) is not part of the policy format.

2. **CEL Runtime**: All policies use CEL (Common Expression Language) runtime version `cel@v14.0` for verification logic.

3. **Snake Case Fields**: Official Ampel uses snake_case for certain fields (e.g., `assert_mode` not `assertMode`) to align with common API conventions.

4. **Parameter Context**: Gemara assessment plan parameters are mapped to Policy.Context as ContextVal entries, providing runtime-configurable values that can be referenced in CEL expressions via `context["param-id"]`.

5. **Predicate Specification**: Attestation types are specified using PredicateSpec objects at both policy and tenet levels.

**Catalog Enrichment:**
When a catalog is provided via the `--catalog` flag, the tool enriches the generated policy with data from Gemara Layer-2 control catalogs:

- **Tenet Titles:** Uses requirement text from the catalog instead of generic evidence requirement descriptions
  - Example: "Build provenance MUST be generated by a trusted builder" (from catalog) instead of "Verify SLSA provenance" (generic)

- **Control Metadata:** Adds control references to `policy.meta.controls`:
  ```json
  {
    "id": "SLSA-BUILD-L3",
    "title": "SLSA Build Level 3",
    "framework": "Build Security",
    "class": "BUILD"
  }
  ```

- **Lookup Process:** For each assessment plan, looks up `requirement-id` in the catalog's controls and their assessment requirements

- **Benefits:**
  - More descriptive tenet titles with specific requirements
  - Traceability from policy → catalog controls
  - Framework and control family information in metadata

## References

### Schemas and Specifications

- **Gemara Project**: https://github.com/gemaraproj/gemara
  - Layer-3 Schema: https://github.com/gemaraproj/gemara/blob/main/schemas/layer-3.cue
  - Go Package: github.com/gemaraproj/go-gemara
- **Ampel Policy API**: https://github.com/carabiner-dev/policy
  - Official Types: https://github.com/carabiner-dev/policy/tree/main/api/v1
- **Ampel Verification Engine**: https://github.com/carabiner-dev/ampel
- **in-toto Attestation Framework**: https://github.com/in-toto/attestation
  - Attestation Spec: https://github.com/in-toto/attestation/tree/main/spec
- **SLSA (Supply chain Levels for Software Artifacts)**:
  - SLSA Framework: https://slsa.dev
  - SLSA Provenance v1.0: https://slsa.dev/provenance/v1
- **Common Expression Language (CEL)**: https://github.com/google/cel-spec
