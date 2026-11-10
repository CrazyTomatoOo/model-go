# syntax=docker/dockerfile:1
# apiserver：多阶段构建。
# builder 钉 golang:1.24.3（与 go.mod toolchain 行一致；钉版纪律禁止 @latest）。
# 运行时 debian:bookworm-slim：自带 util-linux script，admin bootstrap 在无 TTY
# 容器里靠 script 伪 TTY 喂密码（GNU script -qec）。
FROM golang:1.24.3-bookworm AS build
WORKDIR /src
ENV CGO_ENABLED=0
ARG GOPROXY=""
ENV GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/apiserver ./cmd/apiserver

FROM debian:bookworm-slim
RUN useradd --system --uid 65532 --home-dir /nonexistent --shell /usr/sbin/nologin modelgo
COPY --from=build /out/apiserver /usr/local/bin/apiserver
USER modelgo
ENTRYPOINT ["apiserver"]
