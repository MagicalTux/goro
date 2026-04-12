#!/bin/bash
# Extract failing test names and diffs from the latest CI run
# Usage: ./tools/ci-failures.sh [run_id] [limit]
#
# If the run is still in progress, waits for the test job to complete.

set -e

RUN_ID="${1:-$(gh run list --limit 1 --json databaseId -q '.[0].databaseId')}"
LIMIT="${2:-50}"

echo "=== CI Run: $RUN_ID ==="

# Check run status
STATUS=$(gh api "repos/MagicalTux/goro/actions/runs/$RUN_ID" -q '.status')
CONCLUSION=$(gh api "repos/MagicalTux/goro/actions/runs/$RUN_ID" -q '.conclusion // empty')

# Find the test job (coverage or test-*)
find_job() {
    local jid
    jid=$(gh api "repos/MagicalTux/goro/actions/runs/$RUN_ID/jobs" -q '.jobs[] | select(.name == "coverage") | .id' 2>/dev/null)
    if [ -z "$jid" ]; then
        jid=$(gh api "repos/MagicalTux/goro/actions/runs/$RUN_ID/jobs" -q '.jobs[] | select(.name | startswith("test")) | .id' 2>/dev/null | head -1)
    fi
    echo "$jid"
}

JOB_ID=$(find_job)

if [ -z "$JOB_ID" ] && [ "$STATUS" != "completed" ]; then
    echo "Run is $STATUS, waiting for test job to appear..."
    while [ -z "$JOB_ID" ]; do
        sleep 10
        JOB_ID=$(find_job)
        STATUS=$(gh api "repos/MagicalTux/goro/actions/runs/$RUN_ID" -q '.status')
        if [ "$STATUS" = "completed" ] && [ -z "$JOB_ID" ]; then
            echo "Run completed but no test job found"
            exit 1
        fi
    done
fi

if [ -z "$JOB_ID" ]; then
    echo "No test job found in run $RUN_ID"
    exit 1
fi

# Wait for the job to finish if still running
JOB_STATUS=$(gh api "repos/MagicalTux/goro/actions/jobs/$JOB_ID" -q '.status')
if [ "$JOB_STATUS" != "completed" ]; then
    echo "Job $JOB_ID is $JOB_STATUS, waiting..."
    while [ "$JOB_STATUS" != "completed" ]; do
        sleep 15
        JOB_STATUS=$(gh api "repos/MagicalTux/goro/actions/jobs/$JOB_ID" -q '.status')
        echo -n "."
    done
    echo " done"
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
