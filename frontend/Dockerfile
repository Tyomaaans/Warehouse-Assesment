ARG ASSET_SOURCE=build
FROM node:22.22.2-alpine@sha256:8ea2348b068a9544dae7317b4f3aafcdc032df1647bb7d768a05a5cad1a7683f AS build
WORKDIR /app
RUN npm install --prefix /opt/warehouse-npm --no-audit --no-fund npm@11.21.0
ENV PATH=/opt/warehouse-npm/node_modules/.bin:$PATH
COPY package.json package-lock.json .npmrc ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY . .
RUN npm run typecheck && npm run lint && npm run build

FROM scratch AS prebuilt
COPY compiled/ /app/dist/

FROM ${ASSET_SOURCE} AS assets
FROM node:22.22.2-alpine@sha256:8ea2348b068a9544dae7317b4f3aafcdc032df1647bb7d768a05a5cad1a7683f
WORKDIR /app
COPY --from=assets /app/dist/ ./dist/
COPY server.mjs ./
USER 1000:1000
ENV NODE_ENV=production PORT=8080 NODE_OPTIONS=--max-old-space-size=32
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=4s --retries=5 CMD ["node", "-e", "fetch('http://127.0.0.1:8080/health').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"]
CMD ["node", "server.mjs"]
