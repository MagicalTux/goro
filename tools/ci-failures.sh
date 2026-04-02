#!/bin/bash
# Extract failing test names and diffs from the latest CI run
# Usage: ./tools/ci-failures.sh [run_id] [limit]

set -e

RUN_ID="${1:-$(gh run list --limit 1 --json databaseId -q '.[0].databaseId')}"
LIMIT="${2:-50}"

echo "=== CI Run: $RUN_ID ==="

# Find the coverage job (it runs all 2000 tests)
JOB_ID=$(gh api "repos/MagicalTux/goro/actions/runs/$RUN_ID/jobs" -q '.jobs[] | select(.name == "coverage") | .id')

if [ -z "$JOB_ID" ]; then
    # Try test job instead
    JOB_ID=$(gh api "repos/MagicalTux/goro/actions/runs/$RUN_ID/jobs" -q '.jobs[] | select(.name | startswith("test")) | .id' | head -1)
fi

if [ -z "$JOB_ID" ]; then
    echo "No test job found in run $RUN_ID"
    exit 1
fi

echo "Job ID: $JOB_ID"
echo ""

# Download the log
LOG=$(gh api "repos/MagicalTux/goro/actions/jobs/$JOB_ID/logs" 2>/dev/null || echo "")

if [ -z "$LOG" ]; then
    echo "Could not download logs"
    exit 1
fi

# Extract test total
echo "$LOG" | strings | grep -oP '\d+ passed \(\d+\.\d+% success\).*' | tail -1
echo ""

# Extract failing test names (random sample)
ALL_FAILS=$(echo "$LOG" | strings | grep -oP 'Error in test/[^\s:]+\.phpt' | sed 's/Error in //' | sort -u)
TOTAL_FAILS=$(echo "$ALL_FAILS" | wc -l)
echo "=== Failing tests ($LIMIT random of $TOTAL_FAILS) ==="
echo "$ALL_FAILS" | shuf | head -"$LIMIT"

echo ""
echo "=== Sample diffs ==="
# Extract first few diffs (lines starting with + or -)
echo "$LOG" | strings | grep -B1 -A3 "output not as expected" | head -100
