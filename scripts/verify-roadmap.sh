#!/usr/bin/env bash
# verify-roadmap.sh — audit the roadmap dashboard against git reality.
#
# For every dashboard row whose STATUS column is ✅, assert that at least one
# commit references that feature ID (Conventional-Commit scope carries the ID,
# e.g. "feat(context): C1 ..."). This makes "marked done but never committed"
# — the classic hallucinated-progress failure — fail loudly instead of hiding.
#
# Exit 0 = every ✅ row is backed by a commit. Exit 1 = at least one is not.
# Runs locally and in CI (ci.yml). Also warns (does not fail) when a done row
# has no test function naming its ID.
set -uo pipefail

DOC="${1:-docs/WORKORDER_roadmap_beyond_bx.md}"
if [ ! -f "$DOC" ]; then
  echo "verify-roadmap: cannot find $DOC" >&2
  exit 2
fi

fail=0
checked=0
warned=0

# Dashboard rows look like: | F1 | Stable refs | 0 Foundations | ⬜ | ⬜ | ⬜ | ⬜ | ✅ |
# Field 2 (after leading '|') is the feature ID; the last non-empty field is STATUS.
while IFS= read -r line; do
  id="$(printf '%s' "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/,"",$2); print $2}')"
  # Only ID rows like F1, C12, D5 — skip header/separator/other rows.
  printf '%s' "$id" | grep -qE '^[A-Z][0-9]+$' || continue

  status="$(printf '%s' "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/,"",$(NF-1)); print $(NF-1)}')"
  case "$status" in
    *✅*) : ;;        # a done row — audit it
    *)   continue ;;  # not done — nothing to verify
  esac

  checked=$((checked+1))

  if git log --oneline --all 2>/dev/null | grep -qw "$id"; then
    commit_ok="OK"
  else
    commit_ok="FAIL"
    fail=1
  fi

  # Advisory: is there a test function naming this ID?
  if grep -rElq "func Test${id}" --include='*.go' . 2>/dev/null; then
    test_note=""
  else
    test_note="  (warn: no func Test${id}* found)"
    warned=$((warned+1))
  fi

  printf '%-4s %-4s %s%s\n' "$commit_ok" "$id" \
    "$([ "$commit_ok" = OK ] && echo 'commit references it' || echo 'DONE but NO commit references its ID')" \
    "$test_note"
done < "$DOC"

echo "---"
echo "audited ${checked} done-row(s); ${warned} missing an ID-named test"

if [ "$fail" -ne 0 ]; then
  echo "VERIFY FAILED: dashboard claims not backed by commits (hallucinated progress)" >&2
  exit 1
fi
echo "VERIFY OK"
