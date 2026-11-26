#!/usr/bin/env bash
# Runs a one-node Accumulo: manager, tablet server and garbage collector in one container.
set -euo pipefail

if ! hdfs dfs -test -d /accumulo; then
  accumulo init --instance-name demo --password demo-root
fi

# A clean stop leaves the goal state at CLEAN_STOP, and then the manager hosts no user tables.
accumulo org.apache.accumulo.manager.state.SetGoalState NORMAL

accumulo manager & pids=($!)
accumulo tserver & pids+=($!)
accumulo gc & pids+=($!)
init.sh &

# On `docker compose down`, stop cleanly so the next start has no write-ahead log to replay.
trap 'accumulo admin stopAll' TERM

# Stop the container if any server dies.
wait -n "${pids[@]}"
exit 1
