# syntax=docker/dockerfile:1
# console：pnpm 构建 vite SPA → nginx serve + /api 反代。
# BACKEND_URL 在容器启动时用 sed 替换 nginx.conf 的 __BACKEND__ 占位。
FROM node:22-bookworm-slim AS build
WORKDIR /web
ARG NPM_CONFIG_REGISTRY=""
ENV NPM_CONFIG_REGISTRY=${NPM_CONFIG_REGISTRY}
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
COPY web/apps/console/package.json apps/console/package.json
RUN pnpm install --frozen-lockfile
COPY web/apps/console apps/console
RUN pnpm --filter @model-go/console build

FROM nginx:1.27-alpine
COPY docker/console/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /web/apps/console/dist /usr/share/nginx/html
ENTRYPOINT ["sh", "-c", "sed -i \"s|__BACKEND__|${BACKEND_URL}|g\" /etc/nginx/conf.d/default.conf && exec nginx -g 'daemon off;'"]
