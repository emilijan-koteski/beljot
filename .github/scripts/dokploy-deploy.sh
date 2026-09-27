#!/usr/bin/env bash
# Point a Dokploy Docker-provider application at an image reference and deploy it.
# Env: DOKPLOY_URL, DOKPLOY_API_KEY, APP_ID, IMAGE
set -euo pipefail

: "${DOKPLOY_URL:?}" "${DOKPLOY_API_KEY:?}" "${APP_ID:?}" "${IMAGE:?}"

api() {
  curl -fsS -X POST "${DOKPLOY_URL}/api/$1" \
    -H "x-api-key: ${DOKPLOY_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$2" >/dev/null
}

# Pinning the exact tag is what makes Swarm roll the service; a floating :latest is a no-op
# on Dokploy 0.30.x (https://github.com/Dokploy/dokploy/issues/5496).
api application.update "$(jq -cn --arg id "$APP_ID" --arg image "$IMAGE" \
  '{applicationId: $id, sourceType: "docker", dockerImage: $image}')"
api application.deploy "$(jq -cn --arg id "$APP_ID" '{applicationId: $id}')"

echo "Deploy requested for ${IMAGE}"
