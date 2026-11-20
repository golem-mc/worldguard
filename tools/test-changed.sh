#!/usr/bin/env bash
# Repeats the tests a branch adds or changes, to catch a flaky one before it
# merges (mise run test:changed). A test counts when the diff against
# STRESS_BASE (default: the merge base with origin/main) adds its func line or
# changes its body; a change to a helper alone stresses nothing. Each package's
# test binary is built once, timed on one run, then repeated at GOMAXPROCS 1
# and 2, 2 to 20 times, within STRESS_SECONDS (default 240) shared by the
# packages: a single CPU starves the scheduler as a crowded CI runner does.
set -euo pipefail
cd "$(dirname "$0")/.."
base=${STRESS_BASE:-$(git merge-base origin/main HEAD)}
budget=${STRESS_SECONDS:-240}
# "dir Name" for each test func a hunk sits in (git's hunk header names the
# enclosing func) or adds.
tests=$(git diff -U0 "$base" HEAD -- '*_test.go' | sed -nE '
  s#^\+\+\+ b/(.*)/[^/]*$#F \1#p
  s#^\+\+\+ b/[^/]*$#F .#p
  s#^@@.*@@ func (Test[A-Za-z0-9_]*)\(.*#T \1#p
  s#^\+func (Test[A-Za-z0-9_]*)\(.*#T \1#p' |
  awk '$1 == "F" { dir = $2; next } $2 != "TestMain" { print dir, $2 }' | sort -u)
if [ -z "$tests" ]; then
  echo "no new or changed tests"
  exit 0
fi
bin=$(mktemp -d)
trap 'rm -rf "$bin"' EXIT
status=0
dirs=$(cut -d' ' -f1 <<<"$tests" | sort -u)
budget=$((budget / $(wc -l <<<"$dirs")))
for dir in $dirs; do
  names=$(awk -v d="$dir" '$1 == d { print $2 }' <<<"$tests" | paste -sd'|' -)
  [ -d "$dir" ] || continue
  # A nested module is not in this module's package list.
  [ "$dir" != . ] && [ -f "$dir/go.mod" ] && continue
  go test -c -o "$bin/t" "./$dir"
  # A package left with no tests builds no binary.
  [ -x "$bin/t" ] || continue
  start=$(date +%s%N)
  if ! (cd "$dir" && "$bin/t" -test.run "^($names)$" -test.count=1 >"$bin/out" 2>&1); then
    grep -E -- '^(--- FAIL|FAIL|panic:)|_test\.go:[0-9]+:' "$bin/out" | head -n 40 || true
    echo "FAIL $dir: $names failed on its first run"
    status=1
    continue
  fi
  ms=$((($(date +%s%N) - start) / 1000000 + 1))
  n=$((budget * 1000 / ms / 2))
  ((n < 2)) && n=2
  ((n > 20)) && n=20
  echo "$dir: ${names//|/, } x$n at GOMAXPROCS 1 and 2 (one run took ${ms}ms)"
  if ! (cd "$dir" && "$bin/t" -test.run "^($names)$" -test.count="$n" -test.cpu=1,2 -test.timeout=15m >"$bin/out" 2>&1); then
    grep -E -- '^(--- FAIL|FAIL|panic:)|_test\.go:[0-9]+:' "$bin/out" | head -n 40 || true
    echo "FAIL $dir: a new or changed test failed when repeated, so it is flaky"
    status=1
  fi
  rm -f "$bin/t"
done
exit "$status"
