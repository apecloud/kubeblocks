#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${1:-}"

if [[ -z "${IMAGE}" ]]; then
  case "$(docker info --format '{{.Architecture}}')" in
    amd64|x86_64) PLATFORM="linux/amd64" ;;
    arm64|aarch64) PLATFORM="linux/arm64" ;;
    *)
      echo "unsupported Docker architecture; set KBAGENT_TEST_PLATFORM explicitly" >&2
      exit 1
      ;;
  esac
  PLATFORM="${KBAGENT_TEST_PLATFORM:-${PLATFORM}}"
  IMAGE="test.local/kubeblocks/kubeblocks-tools:kbagent-init-test"
  docker buildx build "${ROOT_DIR}" \
    --file "${ROOT_DIR}/docker/Dockerfile-tools" \
    --platform "${PLATFORM}" \
    --build-arg "GOPROXY=${GOPROXY:-https://proxy.golang.org}" \
    --tag "${IMAGE}" \
    --load
fi

CONTAINER_NAME="kbagent-init-test-$$"
cleanup() {
  docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

ACTIONS='[{"name":"orphan","exec":{"command":["/bin/sh","-c","timeout 1 sleep 0.2 </dev/null >/dev/null 2>&1 &"]}},{"name":"timeout-tree","exec":{"command":["/bin/sh","-c","(sleep 2; touch /tmp/descendant-survived) & wait"]},"timeoutSeconds":1}]'

docker run -d \
  --name "${CONTAINER_NAME}" \
  -e "KB_AGENT_ACTION=${ACTIONS}" \
  "${IMAGE}" \
  /bin/kbagent >/dev/null

ready=false
for _ in $(seq 1 60); do
  if docker exec "${CONTAINER_NAME}" curl -fsS \
    -H 'Content-Type: application/json' \
    -d '{"action":"orphan","rerun":true}' \
    http://127.0.0.1:3501/v1.0/action >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.1
done
if [[ "${ready}" != "true" ]]; then
  echo "kbagent HTTP endpoint did not become ready" >&2
  docker logs "${CONTAINER_NAME}" >&2
  exit 1
fi

docker exec "${CONTAINER_NAME}" sh -ec '
  test "$(cat /proc/1/comm)" = "tini-static"
  found_agent=false
  for status in /proc/[0-9]*/status; do
    name=$(awk "\$1 == \"Name:\" { print \$2 }" "$status")
    ppid=$(awk "\$1 == \"PPid:\" { print \$2 }" "$status")
    if [ "$name" = "kbagent" ] && [ "$ppid" = "1" ]; then
      found_agent=true
      break
    fi
  done
  test "$found_agent" = true

  i=0
  while [ "$i" -lt 2000 ]; do
    curl -fsS -H "Content-Type: application/json" \
      -d "{\"action\":\"orphan\",\"rerun\":true}" \
      http://127.0.0.1:3501/v1.0/action >/dev/null
    i=$((i + 1))
  done
'

sleep 1
docker exec "${CONTAINER_NAME}" sh -ec '
  zombies=0
  for status in /proc/[0-9]*/status; do
    state=$(awk "\$1 == \"State:\" { print \$2 }" "$status")
    ppid=$(awk "\$1 == \"PPid:\" { print \$2 }" "$status")
    if [ "$state" = "Z" ] && [ "$ppid" = "1" ]; then
      zombies=$((zombies + 1))
    fi
  done
  test "$zombies" -eq 0
'

docker exec "${CONTAINER_NAME}" curl -sS \
  -H 'Content-Type: application/json' \
  -d '{"action":"timeout-tree","rerun":true}' \
  http://127.0.0.1:3501/v1.0/action >/dev/null
sleep 2.5
if docker exec "${CONTAINER_NAME}" test -e /tmp/descendant-survived; then
  echo "action timeout did not kill the descendant process" >&2
  exit 1
fi

docker stop --time 10 "${CONTAINER_NAME}" >/dev/null
if [[ "$(docker inspect --format '{{.State.ExitCode}}' "${CONTAINER_NAME}")" != "0" ]]; then
  echo "kbagent did not exit cleanly after SIGTERM" >&2
  exit 1
fi

echo "kbagent PID 1 regression test passed"
