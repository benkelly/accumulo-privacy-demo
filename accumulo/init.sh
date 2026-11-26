#!/usr/bin/env bash
# Creates the gateway user and the patients table. Runs on every start, does nothing if done.
set -euo pipefail

until accumulo shell -e "tables" >/dev/null 2>&1; do sleep 2; done

if ! accumulo shell -e "tables" | grep -qx patients; then
  # One PATIENT_ label per seeded patient (p0001 to p0020). Keep in step with the seed data.
  auths="PII,CLINICIAN,LEGAL$(printf ',PATIENT_p%04d' $(seq 1 20))"

  printf 'demo-gateway\ndemo-gateway\n' | accumulo shell -e "createuser gateway"
  accumulo shell -e "setauths -u gateway -s $auths"
  # -evc: refuse a write with a label the writer does not hold, so a typo cannot hide a cell.
  accumulo shell -e "createtable -evc patients"
  for perm in READ WRITE ALTER_TABLE; do
    accumulo shell -e "grant Table.$perm -t patients -u gateway"
  done
fi

# The health check waits for this file.
touch /tmp/ready
