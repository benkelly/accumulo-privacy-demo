#!/usr/bin/env bash
# Runs a one-node HDFS: a namenode and a datanode in the same container.
set -euo pipefail

if [ ! -d /data/name/current ]; then
  hdfs namenode -format -nonInteractive
fi

hdfs datanode &
hdfs namenode &
wait -n
exit 1
