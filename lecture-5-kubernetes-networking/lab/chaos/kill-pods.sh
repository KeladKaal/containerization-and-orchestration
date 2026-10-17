#!/usr/bin/env bash
set -euo pipefail

NS=${NS:-shop}
INTERVAL=${INTERVAL:-5}
LOG=${LOG:-$(dirname "$0")/chaos.log}

echo "chaos: killing random api/worker pod in ns=$NS every ${INTERVAL}s, log=$LOG"
while true; do
  pod=$(kubectl -n "$NS" get pods \
    -l 'app.kubernetes.io/component in (api,worker)' \
    --field-selector=status.phase=Running -o name |
    awk 'BEGIN{srand()} {a[NR]=$0} END{if (NR) print a[int(rand()*NR)+1]}')
  if [ -n "$pod" ]; then
    kubectl -n "$NS" delete "$pod" --wait=false
    echo "$(date +%s) $pod" >> "$LOG"
  fi
  sleep "$INTERVAL"
done
