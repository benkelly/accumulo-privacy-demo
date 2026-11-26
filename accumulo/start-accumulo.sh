#!/usr/bin/env bash
# Runs a one-node Accumulo: manager, tablet server and garbage collector in one container.
set -euo pipefail

if ! hdfs dfs -test -d /accumulo; then
  accumulo init --instance-name demo --password demo-root
fi

accumulo manager & pids=($!)
accumulo tserver & pids+=($!)
accumulo gc & pids+=($!)
init.sh &

# Stop the container if any server dies.
wait -n "${pids[@]}"
exit 1
