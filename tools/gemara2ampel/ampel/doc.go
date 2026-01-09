// Package ampel provides transformation of Gemara Layer-3 policies to
// Ampel attestation verification policies.
//
// Ampel (Amazing Multipurpose Policy Engine and L) is a lightweight supply
// chain policy engine designed to be embedded across the software development
// lifecycle. It verifies unforgeable metadata captured in signed attestations
// using CEL (Common Expression Language).
//
// This package converts Gemara Layer-3 organizational policies into Ampel-compatible
// policies with automated CEL-based verification of in-toto attestations.
//
// # Transformation Mapping
//
// The transformation maps Gemara policy components to Ampel policy structures:
//
//   - Policy → Ampel Policy document (JSON)
//   - AssessmentPlan → Ampel Tenet (individual verification check)
//   - AcceptedMethod (type: automated) → CEL expression for attestation verification
//   - EvidenceRequirements → Attestation predicate expectations
//   - Scope dimensions → CEL filtering expressions
//   - Parameters → Tenet parameters and constraints
//
// # Supported Attestation Types
//
// The package supports automatic inference and verification of common attestation types:
//
//   - SLSA Provenance (https://slsa.dev/provenance/v1)
//   - SPDX SBOM (https://spdx.dev/Document)
//   - CycloneDX SBOM (https://cyclonedx.org/bom)
//   - Vulnerability Scans (https://in-toto.io/Statement/v0.1)
//   - Custom in-toto attestations
//
// # Basic Usage
//
// Transform a Gemara policy to Ampel format:
//
//	policy := &gemara.Policy{}
//	policy.LoadFile("file:///path/to/policy.yaml")
//
//	ampelPolicy, err := ampel.FromPolicy(policy)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	json, err := ampelPolicy.ToJSON()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Println(string(json))
//
// # Advanced Usage with Options
//
// Customize the transformation with options:
//
//	catalog := &gemara.Catalog{}
//	catalog.LoadFile("file:///path/to/catalog.yaml")
//
//	ampelPolicy, err := ampel.FromPolicy(policy,
//	    ampel.WithCatalog(catalog),              // Enrich with control details
//	    ampel.WithScopeFilters(true),            // Generate scope-based filters
//	    ampel.WithAttestationTypes([]string{     // Specify expected types
//	        "https://slsa.dev/provenance/v1",
//	    }),
//	)
//
// # Custom CEL Templates
//
// Provide custom CEL templates for specific verification patterns:
//
//	templates := map[string]string{
//	    "custom-builder-check": `
//	        attestation.predicateType == "https://slsa.dev/provenance/v1" &&
//	        attestation.predicate.builder.id == "{{.BuilderId}}" &&
//	        attestation.predicate.buildType == "{{.BuildType}}"
//	    `,
//	}
//
//	ampelPolicy, err := ampel.FromPolicy(policy,
//	    ampel.WithCELTemplates(templates),
//	)
//
// # CEL Expression Generation
//
// The package automatically generates CEL expressions based on:
//
//   - Evidence requirement text (e.g., "SLSA provenance with trusted builder")
//   - Evaluation method type (automated, gate, behavioral)
//   - Assessment plan parameters
//   - Policy scope dimensions
//
// Generated CEL expressions verify attestation predicates and can include
// scope-based filtering for technologies, regions, and sensitivity levels.
//
// # Example Transformation
//
// Input (Gemara Layer-3 YAML):
//
//	title: "Secure Build Policy"
//	adherence:
//	  assessment-plans:
//	    - id: "slsa-check-01"
//	      requirement-id: "BUILD-01.01"
//	      evaluation-methods:
//	        - type: "automated"
//	          description: "Verify SLSA provenance"
//	      evidence-requirements: "SLSA provenance with trusted builder"
//
// Output (Ampel JSON):
//
//	{
//	  "name": "Secure Build Policy",
//	  "tenets": [
//	    {
//	      "id": "BUILD-01.01-slsa-check-01-0",
//	      "name": "Verify SLSA provenance",
//	      "code": "attestation.predicateType == \"https://slsa.dev/provenance/v1\" && ...",
//	      "attestationTypes": ["https://slsa.dev/provenance/v1"]
//	    }
//	  ],
//	  "rule": "all(tenets)"
//	}
//
// # References
//
//   - Ampel: https://github.com/carabiner-dev/ampel
//   - SLSA: https://slsa.dev
//   - in-toto Attestations: https://github.com/in-toto/attestation
//   - CEL: https://github.com/google/cel-spec
package ampel
