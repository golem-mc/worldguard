#!/bin/sh
set -e
cd "$(dirname "$0")/.."

# RAM-backed scratch avoids large linker writes; cap concurrency so the test
# binaries fit in the runner's memory.
if [ -n "${CI_RAM_TMPDIR:-}" ]; then
  export TMPDIR="$CI_RAM_TMPDIR"
  export GOFLAGS="${GOFLAGS:+$GOFLAGS }-p=6"
fi
# CI bypasses the cache: most packages change each run, and cached file-access
# logs mostly describe temporary worlds that will never exist again.
count=
if [ -n "${CI:-}" ]; then
  count=-count=1
fi
report=$(mktemp)
json=$(mktemp)
stress=$(mktemp)
trap 'rm -f "$report" "$json" "$stress"' EXIT
status=0
# On a PR, the tests it adds or changes are repeated alongside the suite
# (tools/test-changed.sh), whose load is what a flaky test fails under. The
# checkout is the PR merged into main, so its first parent is main.
stress_pid=
if [ -n "${CI:-}" ] && [ "${GITHUB_EVENT_NAME:-}" = pull_request ]; then
  parent=$(git cat-file -p HEAD | sed -n 's/^parent //p' | head -n 1)
  if git fetch -q --depth=1 origin "$parent"; then
    STRESS_BASE=$parent tools/test-changed.sh >"$stress" 2>&1 &
    stress_pid=$!
  fi
fi
# Server tests set their own -parallel because they mostly wait on game ticks.
gotestsum --format pkgname-and-test-fails --format-hide-empty-pkg --jsonfile="$json" \
  --rerun-fails=2 --rerun-fails-max-failures=10 --rerun-fails-abort-on-data-race --rerun-fails-report="$report" \
  --packages=./... -- -timeout 20m $count || status=$?
# Flaky tests are still bugs, so report reruns even if they passed.
if [ -s "$report" ]; then
  printf '\nRerun after a failure:\n'
  cat "$report"
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '### Tests rerun after a failure\n\n```\n%s\n```\n' "$(cat "$report")" >> "$GITHUB_STEP_SUMMARY"
  fi
fi
slowest=$(gotestsum tool slowest --jsonfile "$json" --threshold 15s) || true
if [ -n "$slowest" ]; then
  printf '\nSlowest tests (15s or more):\n%s\n' "$slowest"
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '### Slowest tests\n\n```\n%s\n```\n' "$slowest" >> "$GITHUB_STEP_SUMMARY"
  fi
fi
if [ -n "$stress_pid" ]; then
  printf '\nNew or changed tests, repeated:\n'
  wait "$stress_pid" || status=1
  cat "$stress"
fi
exit "$status"
