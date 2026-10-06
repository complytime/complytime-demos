# Merge Readiness Demo Guide

## What this demo does

This demo answers a simple question: **is a repository ready for merges?**

It automatically checks a real GitHub repository against a set of security
and quality rules, then produces a structured recommendation: allow the
merge, block it, or flag it for human review. No manual checklist, no
tribal knowledge, no inconsistency across repositories.

### The problem it solves

Teams protect their main branches with rules like "require pull requests",
"require approvals", "block force pushes", and "CI checks must pass".
Today these rules are configured in GitHub settings and checked
individually. There is no unified assessment that says "this repository
meets all merge readiness requirements" or "these specific requirements
are failing". At scale, with dozens or hundreds of repositories, manual
verification becomes impractical.

### How it works (in plain terms)

1. A **policy** declares what rules must be satisfied before code can merge
2. **Evidence collectors** query the GitHub API for the actual state of
   branch protection and CI check runs
3. **Policy evaluators** compare the evidence against the declared rules
4. An **EvaluationLog** records the pass/fail result for each requirement
5. A **shadow gate** reads the EvaluationLog and issues a recommendation:
   - **Clear** (green): all requirements pass, safe to merge
   - **Enforced** (red): one or more requirements failed, fix them first
   - **Undetermined** (yellow): evidence is missing or ambiguous, needs
     human review

The entire chain runs automatically inside a virtual machine. No manual
steps, no judgement calls.

---

## Prerequisites

| Requirement | Purpose |
|---|---|
| libvirt + QEMU + Vagrant | Creates the isolated test VM |
| `vagrant-libvirt` plugin | Vagrant provider for libvirt |
| Ansible (on your workstation) | Runs the playbook against the VM |
| GitHub token | Reads repository settings and CI status |

### GitHub token scopes

The token needs two read-only scopes:

- `checks:read` — to query CI check run status
- `administration:read` — to query branch protection rules

Generate one with:
```bash
# If you use the GitHub CLI:
export GITHUB_TOKEN=$(gh auth token)

# Or create a fine-grained token at:
# https://github.com/settings/tokens
```

---

## Running the demo

### 1. Start the VM

```bash
cd base_vms/fedora
vagrant up
```

First run downloads a Fedora 44 cloud image (~500 MB) and bootstraps the
VM with Go, Git, and other tools. Subsequent runs reuse the cached image.

After the VM is ready, verify connectivity:

```bash
cd ../../base_ansible_env
ansible demo_vm -m ping
```

You should see `"pong"`.

### 2. Export your GitHub token

```bash
export GITHUB_TOKEN=$(gh auth token)
```

The playbook validates this before doing anything on the VM and will fail
immediately with a clear message if it is missing.

### 3. Run the playbook

```bash
cd base_ansible_env
ansible-playbook demo_complyctl_merge_readiness.yml
```

A full run takes roughly 2-5 minutes depending on network speed (package
downloads) and GitHub API response time.

---

## What happens at each phase

### Phase 1: Validate prerequisites

Checks that `GITHUB_TOKEN` is set on your workstation. Fails fast if not.

### Phase 2: Install tools

Installs four tools on the VM:

| Tool | Role |
|---|---|
| **complyctl** | Orchestrates the assessment: loads policy, runs evidence collectors, produces the EvaluationLog |
| **ampel** | Evaluates evidence against CEL policy expressions (the actual pass/fail logic) |
| **snappy** | Collects raw evidence from GitHub REST APIs (branch rules, CI check runs) |
| **oras** | Pushes and pulls OCI artifacts (used for the policy bundle) |

**Why these tools?** Each does one thing. `snappy` collects facts.
`ampel` checks facts against rules. `complyctl` ties them together under
a declared policy. This separation means you can swap any component
without rewriting the others.

### Phase 3: Start a local policy registry

Starts a lightweight OCI registry (zot) on the VM. The merge readiness
policy is published here so `complyctl` can fetch it the same way it
would fetch a policy from a production registry like `quay.io`.

**Why a local registry?** The demo uses custom policy content not yet
published to a public registry. The local registry mirrors the production
workflow without requiring external infrastructure.

### Phase 4: Deploy artifacts

Copies all policy content to the VM:

- **Snappy spec** (`check-runs.yaml`): tells snappy which GitHub API
  endpoint to query for CI check run status
- **Ampel granular policies** (6 JSON files deployed to
  `.complytime/ampel/granular-policies/`): CEL expressions that define
  what "passing" means for each requirement (e.g., "all check runs must
  complete with `success`, `skipped`, or `neutral`"). These are placed
  directly in the provider's expected directory so `complyctl generate`
  merges them into the evaluation bundle.
- **Gemara catalog** (`merge-readiness-catalog.yaml`): declares 6
  controls organized in 2 groups: CI pipeline health and source code
  protection
- **Gemara risk catalog** (`merge-readiness-risks.yaml`): maps each
  control group to an adverse outcome it prevents (e.g., "defective
  change integrated without CI verification")
- **Gemara policy** (`merge-readiness-policy.yaml`): binds the controls,
  risks, evaluation method (ampel), and enforcement method (shadow gate)
  into a single policy document
- **Shadow gate** (`shadow-gate.py`): reads the evaluation results and
  issues the final recommendation

**How these files connect**: The catalog and risk catalog are
independent -- they don't reference each other. The **policy** is the
glue that joins them:

1. It **imports** both by their `metadata.id` values (declared in
   `imports.catalogs` and `imports.risks`).
2. It **maps risks to controls** via `adherence.mitigated-risks`
   (e.g., risk `R-CI` is mitigated by control `ci-pipeline-health`).
3. It **maps controls to evaluation methods** via
   `adherence.assessment-plans`, where each plan's `requirement-id`
   matches an assessment-requirement ID defined inside a catalog
   control (e.g., plan `ci-checks-pass` matches the requirement inside
   control `ci-pipeline-health`).
4. Each **ampel granular policy** file has an `id` that matches the
   corresponding `requirement-id` in the policy's assessment plans.
   During `complyctl generate`, these are matched and merged into a
   single evaluation bundle.

### Phase 5: Publish the policy bundle

Pushes the catalog, risk catalog, and policy as an OCI artifact to the
local registry. This is the same mechanism used to distribute policies
across an organization: publish once, consume from any repository.

### Phase 6: Configure the workspace

Writes the `complytime.yaml` configuration that tells complyctl:
- Where to find the policy (the local registry)
- Which repository to scan (`complytime/complytime-demos`)
- Which branch to check (`main`)
- Which snappy specs to use (the built-in `branch-rules.yaml` and the
  custom `check-runs.yaml`)

No complypack is configured. Instead, the ampel granular policy files
were placed directly in `.complytime/ampel/granular-policies/` during
Phase 4. The provider discovers them there automatically.

Then runs `complyctl get` (fetches the Gemara policy from the local
registry), `complyctl generate` (merges the granular policies into an
evaluation bundle and writes the scan configuration), and
`complyctl doctor` (verifies the workspace is valid).

### Phase 7: Run the scan

This is the main event. `complyctl scan` queries the GitHub API for
branch protection rules and CI check run status, evaluates each
requirement against the ampel policy expressions, and produces a
**Gemara EvaluationLog** — a structured YAML document recording the
pass/fail result for every requirement.

### Phase 8: Shadow gate decision

The shadow gate reads the EvaluationLog and issues a disposition:

| Disposition | Meaning | Exit code |
|---|---|---|
| **Clear** | All requirements passed. Safe to merge. | 0 |
| **Enforced** | One or more requirements failed. Fix before merging. | 2 |
| **Undetermined** | Evidence is missing or needs human review. | 3 |

The gate operates in **shadow mode**: it records its recommendation in a
Gemara EnforcementLog but does **not** block or allow anything. No
repository is mutated. This is the first step toward a fully automated
gate — prove the logic works before enabling enforcement.

### Phase 9: Fetch results

Downloads all artifacts to `./downloads_merge_readiness/` on your
workstation for inspection.

---

## Inspecting the results

After the playbook completes, you have five types of artifacts:

```bash
ls downloads_merge_readiness/
```

| File | What it contains |
|---|---|
| `*-snappy.intoto.json` | Raw GitHub API response wrapped in an in-toto attestation (the facts) |
| `*-ampel.intoto.json` | Ampel policy evaluation results in an in-toto attestation (the judgement) |
| `evaluation-log-*.yaml` | Gemara EvaluationLog with per-requirement pass/fail results |
| `enforcement-log-*.yaml` | Gemara EnforcementLog with the gate disposition and justification |
| `report-*.md` | Human-readable Markdown summary |

### Reading the EvaluationLog

```bash
cat downloads_merge_readiness/evaluation-log-*.yaml
```

The top-level `result:` field shows the aggregate. Under `evaluations:`,
each entry shows one control with its assessment result, the requirement
that was checked, and the evidence step that was executed.

### Reading the EnforcementLog

```bash
cat downloads_merge_readiness/enforcement-log-*.yaml
```

The `disposition:` field is the gate recommendation. Under
`justification.findings:`, each requirement is listed with its individual
result and control mapping.

---

## Demonstrating different outcomes

The live scan against `complytime/complytime-demos` typically produces a
**Clear** disposition (the repository has good branch protection). To
show the other two outcomes, SSH into the VM and run the shadow gate
against a modified EvaluationLog:

```bash
# Get the VM IP from the inventory
VM_IP=$(awk -F'=' '/ansible_host/ {print $2}' ansible_inventory | awk '{print $1}')
ssh ansible@$VM_IP

# On the VM — find the EvaluationLog
EVAL_LOG=$(ls ~/complyctl-demo-merge-readiness/.complytime/scan/evaluation-log-*.yaml | tail -1)

# --- Demonstrate "Enforced" (failed requirement) ---
cp "$EVAL_LOG" /tmp/test-failed.yaml
sed -i '0,/result: Passed/{s/result: Passed/result: Failed/}' /tmp/test-failed.yaml
python3 ~/complyctl-demo-merge-readiness/gate/shadow-gate.py /tmp/test-failed.yaml /tmp/
echo "Exit code: $?"
# Expected: Disposition = Enforced, exit code = 2

# --- Demonstrate "Undetermined" (needs review) ---
cp "$EVAL_LOG" /tmp/test-review.yaml
sed -i '0,/result: Passed/{s/result: Passed/result: Needs Review/}' /tmp/test-review.yaml
python3 ~/complyctl-demo-merge-readiness/gate/shadow-gate.py /tmp/test-review.yaml /tmp/
echo "Exit code: $?"
# Expected: Disposition = Undetermined, exit code = 3
```

---

## The value at scale

This demo runs against one repository. The same mechanism scales to an
entire organization:

- **Publish the policy once** to a shared OCI registry. Every repository
  evaluates against the same standard.
- **Add or modify requirements** by updating the policy YAML and
  re-publishing. No per-repository configuration changes needed.
- **Different risk tiers** can use different policies. A
  higher-consequence repository can require stricter controls (e.g.,
  integration tests, no unresolved high-severity findings) while a
  lower-risk repository uses a lighter policy.
- **The evidence chain is auditable**. Every assessment produces signed
  in-toto attestations, a structured EvaluationLog, and an
  EnforcementLog. You can trace any merge decision back to the exact
  evidence and policy version that produced it.
- **Shadow mode first, enforcement later**. The gate starts as a
  recommendation. Once the team trusts the logic, it can be wired into
  CI to block merges that fail the policy. The same EvaluationLog and
  EnforcementLog formats are used in both modes.

---

## Cleanup

```bash
# Destroy the VM
cd base_vms/fedora
vagrant destroy -f

# Remove downloaded artifacts
rm -rf base_ansible_env/downloads_merge_readiness/
```

---

## Troubleshooting

### `complyctl get` fails with "http: server gave HTTP response to HTTPS client"

The policy URL in `complytime.yaml` must start with `http://` for the
local zot registry. The playbook writes this correctly. If you see this
error after manual edits, verify the URL has the `http://` prefix.

### Phase 3 fails (zot registry)

Ensure `podman` is installed on the VM and port 5000 is not already in
use. Run `sudo dnf install -y podman` on the VM if needed.

### Scan shows fewer than 6 requirements

All 6 granular policy files must be present in
`.complytime/ampel/granular-policies/` before `complyctl generate` runs.
If the `ci-checks-pass` requirement is missing, verify that the
playbook copied all 6 JSON files from `files/merge-readiness/ampel-policies/`
to the granular-policies directory. Also confirm that `complytime.yaml`
does **not** include a `complypacks:` section — the complypack takes
exclusive priority over the granular-policies directory, and the
upstream complypack does not include the `ci-checks-pass` policy.

### Token permission errors

Ensure the GitHub token has `checks:read` and `administration:read`
scopes. Fine-grained tokens must grant read access to "Administration"
and "Checks" for the target repository.
