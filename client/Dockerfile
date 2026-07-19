# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS vite-build
WORKDIR /src
ARG VITE_GOOGLE_CLIENT_ID=""
ENV VITE_GOOGLE_CLIENT_ID=$VITE_GOOGLE_CLIENT_ID
ARG APP_VERSION=""
ENV APP_VERSION=$APP_VERSION
COPY client/package.json client/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci
COPY client/ ./
RUN npm run build

FROM caddy:2.10-builder-alpine AS caddy-build
RUN xcaddy build \
    --with github.com/caddy-dns/cloudflare

FROM caddy:2.10-alpine
COPY --from=caddy-build /usr/bin/caddy /usr/bin/caddy
COPY --from=vite-build /src/dist /srv
COPY Caddyfile.prod /etc/caddy/Caddyfile
EXPOSE 80 443
