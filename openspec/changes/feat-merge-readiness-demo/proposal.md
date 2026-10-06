# Proposal

## Why

The Gemara gated-merge design proposal proposes using Gemara as the portable
schema for requirements-to-evidence contracts in an agent auto-merge system. However, all existing ComplyTime demos evaluate compliance posture at
the repository level (branch protection rules, OS-level CIS controls) — none
demonstrate merge readiness assessment that combines CI pipeline health with
governance controls into a single, policy-driven gate decision.

Repositories already run vulnerability scans, unit tests, and linters as CI
checks, but these produce siloed pass/fail signals with no normalized
assessment. A merge readiness gate needs to aggregate these signals against a
declared policy and emit a structured evaluation that can be consumed by an
enforcement system — even if that enforcement system initially operates in
shadow mode (no repository mutation).

## What Changes

A new Ansible playbook and supporting artifacts are added to the
`complytime-demos` repository (`feat/merge-readiness-demo` branch) that
demonstrate a unified merge readiness assessment using the existing ComplyTime
toolchain.

### New artifacts in `complytime-demos`

- **`demo_complyctl_merge_readiness.yml`**: Self-contained Ansible playbook
  (9 phases) that provisions a Fedora VM with all dependencies, deploys a
  custom Gemara policy via a local OCI registry, runs a multi-evidence
  complyctl scan, and executes a shadow gate.

- **New snappy spec** (`snappy-specs/github/check-runs.yaml`): Collects CI
  check run results from the GitHub REST API
  (`repos/{ORG}/{REPO}/commits/{BRANCH}/check-runs?per_page=100`). Uses the
  same `${ORG}/${REPO}/${BRANCH}` variable pattern as the existing
  branch-rules spec.

- **New ampel policy** (`ampel-policies/ci-checks-pass.json`): CEL tenet
  that evaluates the check-runs API response and passes only when all check
  runs have `status: completed` with `conclusion: success|skipped|neutral`.

- **Gemara merge readiness catalog, risk catalog, and policy**: A
  `ControlCatalog` with 6 controls across 2 groups (CI pipeline + source
  code protection), a `RiskCatalog` with 3 adverse outcomes mapped to
  controls, and a `Policy` with 6 assessment plans, mitigated risk
  mappings, and a shadow gate enforcement method. Schema version
  `gemara-version: "1.2.0"`.

- **Shadow gate script** (`shadow-gate.py`): Python script that parses
  the Gemara EvaluationLog produced by complyctl and emits a Gemara
  EnforcementLog YAML with a three-outcome disposition model:
  - `Clear` (exit 0) — all requirements passed
  - `Enforced` (exit 2) — one or more requirements failed
  - `Undetermined` (exit 3) — missing evidence or needs review
  Shadow mode only (`executed-action: NONE`). Includes per-requirement
  findings in the EnforcementLog justification.

- **5 branch protection ampel policies**: Copied from `org-infra` with
  semantic IDs (`require-pull-request`, `minimum-approvals`,
  `block-force-push`, `prevent-admin-bypass`, `require-code-owner-review`)
  matching the current Gemara policy published on quay.io.

### Architecture

```
GITHUB_TOKEN
    |
    v
complyctl scan complytime-demos --policy-id merge-readiness
    |
    +-> snappy snap builtin:github/branch-rules.yaml
    |       -> ampel verify -> branch protection results (5 controls)
    +-> snappy snap github/check-runs.yaml
    |       -> ampel verify -> CI status results (1 control)
    |
    v
Gemara EvaluationLog (6 controls unified)
    |
    v
shadow-gate.py -> Gemara EnforcementLog (Clear / Enforced / Undetermined)
```

### Demo target

The playbook scans `complytime/complytime-demos` on GitHub, which has active
CI (MegaLinter, OSV-Scanner, Trivy, OpenSSF Scorecards) and branch
protection configured.

### Token scopes

`checks:read` + `administration:read` — same as the existing GitHub demo.

## Capabilities

### New Capabilities

- `demo-merge-readiness`: Defines a merge readiness assessment demo that
  evaluates CI pipeline health and branch protection rules as a unified
  Gemara EvaluationLog, then runs a shadow gate emitting a Gemara
  EnforcementLog with a three-outcome disposition model. Demonstrates:
  - Custom snappy specs collecting evidence from new GitHub API endpoints
  - Custom ampel CEL policies evaluating CI check run results
  - A Gemara policy bundle (catalog + risk catalog + policy) distributed
    via a local OCI registry
  - Risk-to-control mappings and enforcement method declarations
  - The evaluator-to-gate chain: EvaluationLog -> EnforcementLog
  - Three-outcome gate decisions: Clear / Enforced / Undetermined

### Modified Capabilities

_None._

### Removed Capabilities

_None._

## Impact

- **Repository**: `complytime-demos` (`feat/merge-readiness-demo` branch)
- **Files created**:
  - `base_ansible_env/demo_complyctl_merge_readiness.yml`
  - `base_ansible_env/files/merge-readiness/snappy-specs/github/check-runs.yaml`
  - `base_ansible_env/files/merge-readiness/ampel-policies/ci-checks-pass.json`
  - `base_ansible_env/files/merge-readiness/ampel-policies/require-pull-request.json`
  - `base_ansible_env/files/merge-readiness/ampel-policies/minimum-approvals.json`
  - `base_ansible_env/files/merge-readiness/ampel-policies/block-force-push.json`
  - `base_ansible_env/files/merge-readiness/ampel-policies/prevent-admin-bypass.json`
  - `base_ansible_env/files/merge-readiness/ampel-policies/require-code-owner-review.json`
  - `base_ansible_env/files/merge-readiness/gemara/merge-readiness-catalog.yaml`
  - `base_ansible_env/files/merge-readiness/gemara/merge-readiness-risks.yaml`
  - `base_ansible_env/files/merge-readiness/gemara/merge-readiness-policy.yaml`
  - `base_ansible_env/files/merge-readiness/shadow-gate.py`
- **Dependencies**: Existing toolchain (complyctl, complytime-providers-ampel,
  snappy, ampel, oras, python3-pyyaml). New: local zot OCI registry via
  podman. The upstream complypack (`complypack-ampel-branch-protection`) is
  **not used**; all 6 ampel granular policies are deployed directly to
  `.complytime/ampel/granular-policies/` so the provider discovers them
  without complypack exclusive-priority interference.
- **Not affected**: Existing demo playbooks, development playbooks, templates,
  Vagrantfile.

## Known Risks

- **Ampel cross-predicate-type filtering** _(resolved)_: The ampel provider
  now filters the merged policy bundle per attestation type before calling
  `ampel verify`. Each invocation receives only the policies whose
  `predicates.types` match the attestation's `predicateType`. Filtered bundles
  are written to a temporary file under `$XDG_CACHE_HOME/complytime/ampel/`
  and cleaned up after use. This prevents ampel from reporting "required
  attestations missing" for policies that expect a different spec type.

- **Snappy spec requires `mask` for object responses** _(resolved)_: Custom
  snappy specs that query GitHub API endpoints returning JSON objects (not
  arrays) must include a `mask` field listing the response fields to extract.
  The `check-runs.yaml` spec was updated with `mask: [total_count,
  check_runs]` to match the format used by the GitLab specs
  (`branch-protection.yaml`, `project-approvals.yaml`).

- **Snappy does not paginate**: GitHub API responses are limited to 100 results
  per page (`?per_page=100` in the spec). Repositories with more than 100
  check runs per commit will see truncated results. Acceptable for the PoC;
  upstream snappy pagination support is the long-term fix.

- **`complyctl get` with `plain_http` and `skip_verify`**: The local zot
  registry runs without TLS. The `plain_http: true` and `skip_verify: true`
  config keys may not be supported by all complyctl versions. Mitigation:
  verify against complyctl v1.0.0; fall back to manual cache population if
  needed.

- **OCI media type compatibility**: The `oras push` with Gemara-specific media
  types (`application/vnd.gemara.catalog.v1+yaml`,
  `application/vnd.gemara.policy.v1+yaml`) must produce an OCI manifest that
  complyctl's cache layer recognizes. The production publish uses
  `gemaraproj/gemara-publish-action`; the oras push is a simplified
  equivalent. Mitigation: verify the manifest after push and compare against
  a known-good manifest from quay.io.

## Relationship to the Gated-Merge Design Proposal

This PoC validates the first two layers of the gated-merge architecture:

| Spec component             | PoC coverage                                    |
|----------------------------|------------------------------------------------|
| ComplyTime evaluates       | Yes — complyctl + ampel provider                |
| Gemara EvaluationLog       | Yes — produced by complyctl scan                |
| Shadow gate                | Yes — shadow-gate.py with three-outcome model   |
| Signed in-toto SVR         | Partial — ampel produces in-toto attestations   |
| SLSA provenance            | Not in scope — narrate from org-infra workflows |
| Post-merge AuditLog        | Not in scope                                    |

The PoC focuses on demonstrating the evaluator-to-gate chain with real CI
evidence. The SVR signing/verification boundary and post-merge audit are
future work that builds on the patterns established here.
