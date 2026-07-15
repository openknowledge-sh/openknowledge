FROM golang:1.22-bookworm AS build

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates nodejs npm \
  && npm install -g pnpm@10 \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages/npm/package.json packages/npm/package.json
COPY packages/web/package.json packages/web/package.json
RUN pnpm install --frozen-lockfile --ignore-scripts

COPY . .
RUN pnpm build:web

FROM node:22-bookworm-slim

WORKDIR /app

ENV NODE_ENV=production
ENV OPENKNOWLEDGE_WEB_ROOT=dist
ENV OPENKNOWLEDGE_WEB_EXPORT_WIKI=0
ENV HOST=0.0.0.0

COPY --from=build /app/packages/web/dist packages/web/dist
COPY --from=build /app/packages/web/scripts packages/web/scripts

USER node

CMD ["node", "packages/web/scripts/serve.mjs"]
