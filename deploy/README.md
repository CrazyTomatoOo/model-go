# Deploy

Helm charts, split by cluster role. See docs/adr/0002 for the hub/member split.

## hub/ — 管理集群 (management plane)

| Chart | Binary (cmd/) | Role |
|---|---|---|
| hub/apiserver | apiserver | REST surface (gin), Postgres only |
| hub/agent-gateway | agent-gateway | gRPC entrypoint for member agents |
| hub/scheduler | scheduler | queueing / scoring / binding |
| hub/controller-manager | controller-manager | orchestration, no workload CRDs |

## member/ — 业务集群 (compute plane)

| Chart | Binary (cmd/) | Role |
|---|---|---|
| member/cluster-agent | cluster-agent | pull tasks, heartbeat, capacity report |
| member/cluster-controller | cluster-controller | reconcile TrainingJob/InferenceService CRDs |
| member/model-gateway | model-gateway | model traffic entrypoint |
| member/addons/ | — | override values for Volcano and accelerator device plugins |

Templates for `hub/apiserver` and `hub/console` are written; the other six
charts still ship Chart.yaml + values.yaml only. Dockerfiles live in
`docker/`; the kind dev environment lives in `deploy/kind/`.

## 本地 kind 联调环境

已验证的全套流程（约 15 分钟，命令均在仓库根目录执行）：

```bash
# 1. 构建镜像（本机 arm64；国内网络走镜像源参数，海外/CI 去掉两个 --build-arg）
docker build -f docker/apiserver/Dockerfile -t model-go-apiserver:dev \
  --build-arg GOPROXY=https://goproxy.cn,direct .
docker build -f docker/console/Dockerfile -t model-go-console:dev \
  --build-arg NPM_CONFIG_REGISTRY=https://registry.npmmirror.com .

# 2. 建集群（沙箱/带 *_proxy 环境变量时 kind 节点会继承代理 env，
#    导致节点拉公网镜像连 127.0.0.1:7890 失败——公网镜像必须本机 pull 后 kind load，见第 3 步）
kind create cluster --config deploy/kind/config.yaml

# 3. 公网镜像本地拉取 + 加载（bitnami tag 为 latest，pullPolicy 必须 IfNotPresent，
#    且 kind load 存的是 docker.io 引用，kubelet 查的是 registry-1.docker.io 引用，
#    需要在节点上给镜像补 registry-1 别名——脚本化见 deploy/kind/load-deps.sh）
docker pull bitnami/postgresql:latest bitnami/redis:latest
kind load docker-image model-go-apiserver:dev model-go-console:dev \
  bitnami/postgresql:latest bitnami/redis:latest --name model-go

# 4. ingress-nginx（manifest 钉了 digest，节点上只有 tag 引用时要先去掉 @sha256 段）
sed -E 's|@sha256:[a-f0-9]+||' /tmp/ingress-kind.yaml | kubectl apply -f -

# 5. PG / Redis（Redis 关认证：apiserver 目前只支持无密码 Redis，见后续工作）
helm upgrade -i modelgo-pg bitnami/postgresql -n model-go --create-namespace \
  --set auth.username=modelgo --set auth.password=modelgo --set auth.database=modelgo \
  --set image.pullPolicy=IfNotPresent --set persistence.size=1Gi
helm upgrade -i modelgo-redis bitnami/redis -n model-go \
  --set architecture=standalone --set auth.enabled=false \
  --set master.image.pullPolicy=IfNotPresent

# 6. apiserver + bootstrap（dev 密码仅联调用；密码策略：不得包含用户名，≥12 字符两类）
helm upgrade -i model-go-apiserver deploy/hub/apiserver -n model-go \
  --set image.repository=model-go-apiserver --set image.tag=dev \
  --set 'postgres.dsn=postgres://modelgo:modelgo@modelgo-pg-postgresql:5432/modelgo?sslmode=disable' \
  --set redis.addr=modelgo-redis-master:6379 \
  --set bootstrap.enabled=true \
  --set 'bootstrap.passwords.sysadmin=Hub-dev#2026-a' \
  --set 'bootstrap.passwords.secadmin=Hub-dev#2026-b' \
  --set 'bootstrap.passwords.auditadmin=Hub-dev#2026-c'

# 7. console
helm upgrade -i model-go-console deploy/hub/console -n model-go \
  --set image.repository=model-go-console --set image.tag=dev

# 8. 全链路 smoke（登录→强制改密→me→登出→新密码重登）
deploy/kind/smoke.sh
```

重置初始管理员（不动库表结构）：

```bash
kubectl -n model-go exec modelgo-pg-postgresql-0 -- sh -c \
  'PGPASSWORD=modelgo psql -U modelgo -d modelgo -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"'
helm upgrade model-go-apiserver deploy/hub/apiserver -n model-go --reuse-values   # 重跑 bootstrap hook
```

已知的坑（都踩过）：

1. **kind 节点代理 env**：在带 `http(s)_proxy` 的 shell 里创建的 kind 节点会把代理
   env 传给 kubelet，公网镜像拉取必失败。两条路：干净 env 重建集群，或一律本地
   pull + `kind load` + 节点内 `ctr images tag` 补 registry-1 别名。
2. **bootstrap Job 的 TTY**：`term.ReadPassword` 需要 TTY，容器里用
   `printf '%s\n%s\n%s\n' $PWS | script -qec "stty -echo; apiserver admin bootstrap"`
   伪 TTY 喂三行；`stty -echo` 防止密码回显进 Job 日志。
3. **改密 API 字段是 `old_password`/`new_password`**（不是 current_password）。
4. **MustChangeGate 不拦 `/auth/me`**，它返回 `must_change_password` 标志，前端据此跳改密页。

