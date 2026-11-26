#!/usr/bin/env bash
# Usage: fetch.sh <path under apache.org/dist> <install dir>
# Downloads an Apache tarball, checks its SHA-512 against archive.apache.org, and unpacks it.
set -euo pipefail
path="$1" dest="$2" tgz=/tmp/download.tar.gz

curl -fsSL "https://dlcdn.apache.org/$path" -o "$tgz" \
  || curl -fsSL "https://archive.apache.org/dist/$path" -o "$tgz"

want=$(curl -fsSL "https://archive.apache.org/dist/$path.sha512" | grep -oiE '[0-9a-f]{128}' | head -1)
echo "$want  $tgz" | sha512sum -c -

mkdir -p "$dest"
tar -xzf "$tgz" -C "$dest" --strip-components=1
rm "$tgz"
