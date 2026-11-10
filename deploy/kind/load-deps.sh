#!/usr/bin/env bash
# 把 kind 集群需要的公网镜像拉到本机并加载进节点，补 registry-1.docker.io 别名，
# 让 kubelet 的 IfNotPresent 查找命中本地镜像（绕过 kind 节点继承的沙箱代理 env）。
# 用法：deploy/kind/load-deps.sh [集群名，默认 model-go]
set -euo pipefail
CLUSTER="${1:-model-go}"

IMAGES=(
  bitnami/postgresql:latest
  bitnami/redis:latest
)

docker pull "${IMAGES[@]}"
kind load docker-image "${IMAGES[@]}" --name "$CLUSTER"

NODE="${CLUSTER}-control-plane"
for img in "${IMAGES[@]}"; do
  docker exec "$NODE" ctr -n k8s.io images tag "docker.io/${img}" "registry-1.docker.io/${img}" >/dev/null
  echo "aliased registry-1.docker.io/${img}"
done
