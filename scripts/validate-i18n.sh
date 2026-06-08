#!/usr/bin/env bash
set -euo pipefail

I18N_DIR="panel/spa/static/i18n"
REFERENCE="$I18N_DIR/en.json"

if [ ! -f "$REFERENCE" ]; then
  echo "ERROR: reference file $REFERENCE not found"
  exit 1
fi

ref_keys=$(jq -r 'keys[]' "$REFERENCE" | sort)
errors=0

for f in "$I18N_DIR"/*.json; do
  lang=$(basename "$f" .json)
  if [ "$f" = "$REFERENCE" ]; then continue; fi

  if ! jq empty "$f" 2>/dev/null; then
    echo "ERROR: $f is not valid JSON"
    errors=$((errors + 1))
    continue
  fi

  file_keys=$(jq -r 'keys[]' "$f" | sort)

  missing=$(comm -23 <(echo "$ref_keys") <(echo "$file_keys"))
  extra=$(comm -13 <(echo "$ref_keys") <(echo "$file_keys"))

  if [ -n "$missing" ]; then
    echo "MISSING in $lang:"
    echo "$missing" | sed 's/^/  /'
    errors=$((errors + 1))
  fi

  if [ -n "$extra" ]; then
    echo "EXTRA in $lang:"
    echo "$extra" | sed 's/^/  /'
    errors=$((errors + 1))
  fi

  if [ -z "$missing" ] && [ -z "$extra" ]; then
    echo "OK: $lang — all keys present"
  fi
done

if [ "$errors" -gt 0 ]; then
  echo ""
  echo "FAILED: $errors issue(s) found"
  exit 1
fi

echo ""
echo "PASSED: all translations have matching keys"
