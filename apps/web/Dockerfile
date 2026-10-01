FROM node:24-bookworm-slim

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/package.json
RUN npm install --global corepack@0.34.0 \
  && corepack pnpm@10.18.2 install --frozen-lockfile --filter @floodwatch/web

COPY apps/web apps/web

WORKDIR /app/apps/web

EXPOSE 5173

CMD ["corepack", "pnpm@10.18.2", "dev", "--host", "0.0.0.0"]
