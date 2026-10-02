#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# shadow-gate.sh — Merge readiness shadow gate
#
# Reads a Gemara EvaluationLog produced by complyctl and emits a Gemara
# EnforcementLog with the gate recommendation. Shadow mode only — no
# repository mutation is performed (executed_action: NONE).
#
# Usage: ./shadow-gate.sh <evaluation-log.yaml> [output-dir]
#
# Requires: grep, date

set -euo pipefail

EVAL_LOG="${1:?Usage: $0 <evaluation-log.yaml> [output-dir]}"
OUTPUT_DIR="${2:-.}"

if [ ! -f "$EVAL_LOG" ]; then
    echo "Error: evaluation log not found: $EVAL_LOG" >&2
    exit 1
fi

TIMESTAMP="$(date -u +%Y%m%d-%H%M%S)"
EVAL_BASENAME="$(basename "$EVAL_LOG")"

# Extract the aggregate result from the EvaluationLog.
# complyctl writes "result: Passed" or "result: Failed" at the top level.
EVAL_RESULT="$(grep -m1 '^result:' "$EVAL_LOG" | sed 's/^result:[[:space:]]*//')"

if [ -z "$EVAL_RESULT" ]; then
    echo "Error: could not extract result from $EVAL_LOG" >&2
    exit 1
fi

# Count per-requirement results for the summary.
TOTAL="$(grep -c '^  - requirement-id:' "$EVAL_LOG" 2>/dev/null || echo 0)"
PASSED="$(grep -c 'result: Passed' "$EVAL_LOG" 2>/dev/null || echo 0)"
FAILED="$(grep -c 'result: Failed' "$EVAL_LOG" 2>/dev/null || echo 0)"

# Map evaluation result to Gemara Disposition enum.
if [ "$EVAL_RESULT" = "Passed" ]; then
    DISPOSITION="Clear"
    RATIONALE="All ${TOTAL} merge readiness requirements passed."
else
    DISPOSITION="Enforced"
    RATIONALE="${FAILED} of ${TOTAL} merge readiness requirements failed. Remediation required."
fi

OUTPUT_FILE="${OUTPUT_DIR}/enforcement-log-${TIMESTAMP}.yaml"

cat > "$OUTPUT_FILE" <<EOF
# Gemara EnforcementLog — Shadow Gate Output
# Source: ${EVAL_BASENAME}
# Mode: Shadow (no repository mutation)
metadata:
    id: enforcement-merge-readiness-${TIMESTAMP}
    type: EnforcementLog
    gemara-version: "1.2.0"
    description: >-
        Shadow gate enforcement decision for merge readiness assessment.
        No repository action was performed.
disposition: ${DISPOSITION}
actions:
    - disposition: ${DISPOSITION}
      message: "${RATIONALE}"
      start: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      steps:
          - description: "Shadow gate — evaluated EvaluationLog aggregate result"
            executed-action: NONE
      justification:
          rationale: "${RATIONALE}"
          evidence:
              - source: "${EVAL_BASENAME}"
                type: EvaluationLog
target:
    id: complytime/complytime-demos
    name: complytime-demos
    environment: github
EOF

echo "---"
echo "Shadow Gate Result"
echo "---"
echo "  Disposition : ${DISPOSITION}"
echo "  Rationale   : ${RATIONALE}"
echo "  Source       : ${EVAL_BASENAME}"
echo "  Output      : ${OUTPUT_FILE}"
echo "  Mode        : Shadow (executed_action: NONE)"
echo "---"

cat "$OUTPUT_FILE"
