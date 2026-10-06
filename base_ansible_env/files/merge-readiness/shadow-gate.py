#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Shadow gate for merge readiness enforcement.

Reads a Gemara EvaluationLog (YAML) produced by complyctl and emits
a Gemara EnforcementLog with a three-outcome disposition:

  Clear        - all requirements passed
  Enforced     - one or more requirements failed (remediation needed)
  Undetermined - missing evidence, needs review, or structural issue

Shadow mode only: no repository mutation (executed-action: NONE).

Exit codes:
  0 - Clear
  1 - Runtime error
  2 - Enforced
  3 - Undetermined

Usage:
  python3 shadow-gate.py <evaluation-log.yaml> [output-dir]

Requires: python3, PyYAML (python3-pyyaml)
"""

from __future__ import annotations

import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import yaml

# Result values that count as "passing" for gate purposes.
PASSING_RESULTS = frozenset({"Passed", "Not Applicable"})

# Result values that indicate a clear failure (remediation path exists).
FAILING_RESULTS = frozenset({"Failed"})

# Result values that indicate uncertainty (escalation required).
UNCERTAIN_RESULTS = frozenset(
    {
        "Needs Review",
        "Unknown",
        "Not Run",
    }
)

# Exit codes aligned with gemara-demo conventions.
EXIT_CLEAR = 0
EXIT_ERROR = 1
EXIT_ENFORCED = 2
EXIT_UNDETERMINED = 3


# -------------------------------------------------------------------
# EvaluationLog parsing
# -------------------------------------------------------------------


def load_evaluation_log(path: Path) -> dict[str, Any]:
    """Load and validate the basic structure of a YAML EvaluationLog."""
    with open(path, encoding="utf-8") as fh:
        data = yaml.safe_load(fh)

    if not isinstance(data, dict):
        raise ValueError(f"EvaluationLog must be a YAML mapping, got {type(data).__name__}")

    if "result" not in data:
        raise ValueError("EvaluationLog is missing the top-level 'result' field")

    return data


def extract_requirement_results(
    data: dict[str, Any],
) -> list[dict[str, str]]:
    """Extract per-requirement results from the EvaluationLog.

    Handles the Gemara native format (evaluations -> assessment-logs)
    and falls back to a flat requirement-results list if the native
    structure is not present.

    Returns a list of dicts with keys: requirement_id, result, control,
    and description.
    """
    results: list[dict[str, str]] = []

    evaluations = data.get("evaluations")
    if isinstance(evaluations, list):
        for evaluation in evaluations:
            if not isinstance(evaluation, dict):
                continue
            control_name = evaluation.get("name", "unknown")
            control_result = str(evaluation.get("result", "Unknown"))
            control_entry = ""
            control_ref = evaluation.get("control", {})
            if isinstance(control_ref, dict):
                control_entry = control_ref.get("entry-id", "")

            assessment_logs = evaluation.get("assessment-logs")
            if isinstance(assessment_logs, list) and assessment_logs:
                for log_entry in assessment_logs:
                    if not isinstance(log_entry, dict):
                        continue
                    req_ref = log_entry.get("requirement", {})
                    req_id = (
                        req_ref.get("entry-id", control_name)
                        if isinstance(req_ref, dict)
                        else control_name
                    )
                    results.append(
                        {
                            "requirement_id": req_id,
                            "result": str(log_entry.get("result", control_result)),
                            "control": control_entry or control_name,
                            "description": str(log_entry.get("description", "")),
                        }
                    )
            else:
                # No assessment-logs; use the control-level result.
                results.append(
                    {
                        "requirement_id": control_name,
                        "result": control_result,
                        "control": control_entry or control_name,
                        "description": "",
                    }
                )
        return results

    # Fallback: flat list with requirement-id fields (complyctl may
    # use this structure in some output modes).
    req_list = data.get("requirement-results")
    if isinstance(req_list, list):
        for entry in req_list:
            if not isinstance(entry, dict):
                continue
            results.append(
                {
                    "requirement_id": str(entry.get("requirement-id", "unknown")),
                    "result": str(entry.get("result", "Unknown")),
                    "control": str(entry.get("control-id", "")),
                    "description": str(entry.get("description", "")),
                }
            )
        return results

    return results


# -------------------------------------------------------------------
# Disposition logic
# -------------------------------------------------------------------


def determine_disposition(
    aggregate_result: str,
    requirement_results: list[dict[str, str]],
) -> tuple[str, str, int]:
    """Determine the gate disposition from the evaluation results.

    Returns (disposition, rationale, exit_code).
    """
    if not requirement_results:
        return (
            "Undetermined",
            "No requirement results found in the EvaluationLog. Cannot determine merge readiness.",
            EXIT_UNDETERMINED,
        )

    total = len(requirement_results)
    passed = sum(1 for r in requirement_results if r["result"] in PASSING_RESULTS)
    failed = sum(1 for r in requirement_results if r["result"] in FAILING_RESULTS)
    uncertain = sum(1 for r in requirement_results if r["result"] in UNCERTAIN_RESULTS)

    if uncertain > 0:
        uncertain_ids = [
            r["requirement_id"] for r in requirement_results if r["result"] in UNCERTAIN_RESULTS
        ]
        return (
            "Undetermined",
            f"{uncertain} of {total} requirements have an uncertain "
            f"result ({', '.join(uncertain_ids)}). Manual review or "
            f"re-evaluation is required before a merge decision.",
            EXIT_UNDETERMINED,
        )

    if failed > 0:
        failed_ids = [
            r["requirement_id"] for r in requirement_results if r["result"] in FAILING_RESULTS
        ]
        return (
            "Enforced",
            f"{failed} of {total} merge readiness requirements "
            f"failed ({', '.join(failed_ids)}). "
            f"Remediation required.",
            EXIT_ENFORCED,
        )

    if passed == total:
        return (
            "Clear",
            f"All {total} merge readiness requirements passed.",
            EXIT_CLEAR,
        )

    # Defensive: results with unexpected values.
    unexpected = [
        r
        for r in requirement_results
        if r["result"] not in PASSING_RESULTS
        and r["result"] not in FAILING_RESULTS
        and r["result"] not in UNCERTAIN_RESULTS
    ]
    unexpected_detail = ", ".join(f"{r['requirement_id']}={r['result']}" for r in unexpected)
    return (
        "Undetermined",
        f"Unexpected result values encountered: {unexpected_detail}. "
        f"Cannot determine merge readiness.",
        EXIT_UNDETERMINED,
    )


# -------------------------------------------------------------------
# EnforcementLog generation
# -------------------------------------------------------------------


def build_enforcement_log(
    disposition: str,
    rationale: str,
    requirement_results: list[dict[str, str]],
    eval_log_name: str,
    target_data: dict[str, Any],
    timestamp: str,
) -> dict[str, Any]:
    """Build a Gemara EnforcementLog dict."""
    iso_now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    findings = []
    for req in requirement_results:
        finding: dict[str, Any] = {
            "requirement-id": req["requirement_id"],
            "result": req["result"],
        }
        if req.get("control"):
            finding["control-id"] = req["control"]
        if req.get("description"):
            finding["description"] = req["description"]
        findings.append(finding)

    target_id = "unknown"
    target_name = "unknown"
    target_env = "unknown"
    if isinstance(target_data, dict):
        target_id = target_data.get("id", target_data.get("uri", "unknown"))
        target_name = target_data.get("name", target_id)
        target_env = target_data.get("environment", "unknown")

    enforcement_log: dict[str, Any] = {
        "metadata": {
            "id": f"enforcement-merge-readiness-{timestamp}",
            "type": "EnforcementLog",
            "gemara-version": "1.2.0",
            "description": (
                "Shadow gate enforcement decision for merge "
                "readiness assessment. No repository action was "
                "performed."
            ),
            "date": iso_now,
            "author": {
                "id": "shadow-gate",
                "name": "Shadow Gate",
                "type": "Software",
            },
        },
        "disposition": disposition,
        "actions": [
            {
                "disposition": disposition,
                "message": rationale,
                "start": iso_now,
                "steps": [
                    {
                        "description": (
                            "Shadow gate - evaluated EvaluationLog "
                            "aggregate and per-requirement results"
                        ),
                        "executed-action": "NONE",
                    },
                ],
                "justification": {
                    "rationale": rationale,
                    "evidence": [
                        {
                            "source": eval_log_name,
                            "type": "EvaluationLog",
                        },
                    ],
                    "findings": findings,
                },
            },
        ],
        "target": {
            "id": target_id,
            "name": target_name,
            "environment": target_env,
        },
    }

    return enforcement_log


# -------------------------------------------------------------------
# Output
# -------------------------------------------------------------------


def write_enforcement_log(
    enforcement_log: dict[str, Any],
    output_dir: Path,
    timestamp: str,
) -> Path:
    """Write the EnforcementLog to a YAML file."""
    output_file = output_dir / f"enforcement-log-{timestamp}.yaml"

    class BlockDumper(yaml.SafeDumper):
        """YAML dumper that uses block style for all collections."""

    def represent_str(dumper: yaml.SafeDumper, data: str) -> Any:
        if "\n" in data:
            return dumper.represent_scalar("tag:yaml.org,2002:str", data, style="|")
        return dumper.represent_scalar("tag:yaml.org,2002:str", data)

    BlockDumper.add_representer(str, represent_str)

    source_name = enforcement_log["actions"][0]["justification"]["evidence"][0]["source"]
    header = (
        "# Gemara EnforcementLog - Shadow Gate Output\n"
        f"# Source: {source_name}\n"
        "# Mode: Shadow (no repository mutation)\n"
    )

    yaml_content = yaml.dump(
        enforcement_log,
        Dumper=BlockDumper,
        default_flow_style=False,
        sort_keys=False,
        allow_unicode=True,
        width=99,
    )

    output_file.write_text(header + yaml_content, encoding="utf-8")
    return output_file


def print_summary(
    disposition: str,
    rationale: str,
    eval_log_name: str,
    output_file: Path,
    requirement_results: list[dict[str, str]],
) -> None:
    """Print a human-readable summary to stdout."""
    print("---")
    print("Shadow Gate Result")
    print("---")
    print(f"  Disposition : {disposition}")
    print(f"  Rationale   : {rationale}")
    print(f"  Source       : {eval_log_name}")
    print(f"  Output      : {output_file}")
    print("  Mode        : Shadow (executed_action: NONE)")
    print()

    if requirement_results:
        print("  Requirement Results:")
        for req in requirement_results:
            status_marker = (
                "PASS"
                if req["result"] in PASSING_RESULTS
                else "FAIL"
                if req["result"] in FAILING_RESULTS
                else "????"
            )
            print(f"    [{status_marker}] {req['requirement_id']} = {req['result']}")
        print()

    print("---")


# -------------------------------------------------------------------
# Main
# -------------------------------------------------------------------


def main() -> int:
    """Run the shadow gate and return the exit code."""
    if len(sys.argv) < 2:
        print(
            f"Usage: {sys.argv[0]} <evaluation-log.yaml> [output-dir]",
            file=sys.stderr,
        )
        return EXIT_ERROR

    eval_log_path = Path(sys.argv[1])
    output_dir = Path(sys.argv[2]) if len(sys.argv) > 2 else Path(".")

    if not eval_log_path.is_file():
        print(
            f"Error: evaluation log not found: {eval_log_path}",
            file=sys.stderr,
        )
        return EXIT_ERROR

    output_dir.mkdir(parents=True, exist_ok=True)

    try:
        data = load_evaluation_log(eval_log_path)
    except (yaml.YAMLError, ValueError, OSError) as exc:
        print(
            f"Error: failed to load evaluation log: {exc}",
            file=sys.stderr,
        )
        return EXIT_ERROR

    aggregate_result = str(data.get("result", "Unknown"))
    requirement_results = extract_requirement_results(data)
    target_data = data.get("target", {})

    disposition, rationale, exit_code = determine_disposition(
        aggregate_result,
        requirement_results,
    )

    timestamp = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")
    eval_log_name = eval_log_path.name

    enforcement_log = build_enforcement_log(
        disposition=disposition,
        rationale=rationale,
        requirement_results=requirement_results,
        eval_log_name=eval_log_name,
        target_data=target_data,
        timestamp=timestamp,
    )

    output_file = write_enforcement_log(
        enforcement_log,
        output_dir,
        timestamp,
    )

    print_summary(
        disposition,
        rationale,
        eval_log_name,
        output_file,
        requirement_results,
    )

    return exit_code


if __name__ == "__main__":
    sys.exit(main())
