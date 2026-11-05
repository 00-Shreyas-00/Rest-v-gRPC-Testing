# TypeScript REST server. Compiled with tsc, run as plain JavaScript on Node.js
# with production dependencies only and NODE_ENV=production.

FROM node:24.22.0-bookworm-slim AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

FROM node:24.22.0-bookworm-slim AS runtime
ENV NODE_ENV=production
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --no-audit --no-fund && npm cache clean --force
COPY --from=build /app/dist ./dist
USER node
EXPOSE 8080
# Every second until healthy, then once a minute (negligible during benchmarks).
HEALTHCHECK --interval=60s --timeout=5s --start-period=60s --start-interval=1s --retries=3 \
    CMD ["node", "-e", "fetch('http://127.0.0.1:' + (process.env.HTTP_PORT || 8080) + '/health').then(r => process.exit(r.ok ? 0 : 1), () => process.exit(1))"]
# node is PID 1 and handles SIGTERM itself (graceful shutdown in src/main.ts).
CMD ["node", "dist/main.js"]
