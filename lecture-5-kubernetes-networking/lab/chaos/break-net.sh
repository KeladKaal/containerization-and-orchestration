#!/usr/bin/env bash
set -euo pipefail

NS=shop
DIR="$(cd "$(dirname "$0")" && pwd)"

FAULTS=(
  "c2hvcC1wb3N0Z3Jlc3xbeyJvcCI6InJlcGxhY2UiLCJwYXRoIjoiL3NwZWMvaW5ncmVzcy8wL2Zyb20vMC9wb2RTZWxlY3Rvci9tYXRjaExhYmVscy9hcHAua3ViZXJuZXRlcy5pb34xY29tcG9uZW50IiwidmFsdWUiOiJhcGlzZXJ2ZXIifV0="
  "c2hvcC1hcGl8W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL2VncmVzcy8wL3BvcnRzLzAvcG9ydCIsInZhbHVlIjo1NDMzfV0="
  "c2hvcC1hcGl8W3sib3AiOiJyZW1vdmUiLCJwYXRoIjoiL3NwZWMvZWdyZXNzLzEifV0="
  "c2hvcC13b3JrZXJ8W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL2VncmVzcy8wL3BvcnRzLzAvcG9ydCIsInZhbHVlIjo1NDMzfV0="
)

kubectl get networkpolicy -n "$NS" -o yaml > "$DIR/.netpol-backup.yaml"

PICK=$((RANDOM % ${#FAULTS[@]}))
DECODED="$(printf '%s' "${FAULTS[$PICK]}" | base64 -d)"
TARGET="${DECODED%%|*}"
PATCH="${DECODED#*|}"

kubectl patch networkpolicy -n "$NS" "$TARGET" --type=json -p "$PATCH" > /dev/null

printf '%s' "$PICK" | base64 > "$DIR/.fault"
echo "Поломка применена. Бэкап политик: chaos/.netpol-backup.yaml"
echo "Дальше — симптом и Hubble UI. Не подглядывать в скрипт и в .fault :)"
