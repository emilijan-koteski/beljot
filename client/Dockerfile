# syntax=docker/dockerfile:1.7

# Build context = repo root (so this image can also COPY Caddyfile.prod).

FROM node:22-alpine AS vite-build
WORKDIR /src
# VITE_* vars are baked into the static bundle at build time, so the Google
# client ID must be present here — the runtime .env on the VPS is too late.
# Passed as a --build-arg by the deploy workflow (public OAuth client ID; empty
# by default disables the Google button).
ARG VITE_GOOGLE_CLIENT_ID=""
ENV VITE_GOOGLE_CLIENT_ID=$VITE_GOOGLE_CLIENT_ID
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
