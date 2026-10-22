#!/usr/bin/env bash
# Start the Docker daemon in this cloud container (it is installed but not running).
# Docker Hub answers 429 to anonymous pulls from the shared egress IP, so pulls go
# through Google's public mirror (mirror.gcr.io). Containers that need the network
# must use: --network host, the proxy env vars, and the proxy CA mounted as the
# system bundle (see docker_net_args in eval/runner/tbench.sh).
set -euo pipefail
if docker info >/dev/null 2>&1; then echo "docker already running"; exit 0; fi
mkdir -p /etc/docker
if [ ! -f /etc/docker/daemon.json ]; then
  echo '{ "registry-mirrors": ["https://mirror.gcr.io"] }' > /etc/docker/daemon.json
fi
nohup dockerd > /var/log/dockerd.log 2>&1 &
for _ in $(seq 1 60); do docker info >/dev/null 2>&1 && break; sleep 1; done
docker info >/dev/null 2>&1 || { echo "dockerd failed to start; see /var/log/dockerd.log" >&2; exit 1; }
echo "docker running: $(docker info --format '{{.ServerVersion}} {{.Driver}}') mirrors=$(docker info --format '{{.RegistryConfig.Mirrors}}')"
