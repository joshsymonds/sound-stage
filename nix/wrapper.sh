#!@bash@/bin/bash
# Wrapper that bootstraps a Python venv for audio-separator[gpu] on first run
# and execs the delyric FastAPI worker.
set -euo pipefail

SRC_DIR="@srcDir@"
PYTHON="@python@"
FFMPEG_BIN="@ffmpegBin@"
NATIVE_LIBS="@nativeLibs@"
BUILD_TOOLS="@buildToolsBin@"

: "${DELYRIC_STATE_DIR:?DELYRIC_STATE_DIR must be set (systemd StateDirectory)}"

VENV="${DELYRIC_STATE_DIR}/venv"
REQ_HASH_FILE="${VENV}/.requirements-hash"
REQ_FILE="${SRC_DIR}/requirements.txt"

venv_healthy() {
  [ -x "${VENV}/bin/python" ] && "${VENV}/bin/python" -c 'import sys' >/dev/null 2>&1
}

NEW_HASH="$(sha256sum "${REQ_FILE}" | cut -d' ' -f1)"
needs_install=0

if ! venv_healthy; then
  echo "delyric-worker: (re)creating venv at ${VENV}" >&2
  rm -rf "${VENV}"
  "${PYTHON}" -m venv --system-site-packages "${VENV}"
  needs_install=1
elif [ ! -f "${REQ_HASH_FILE}" ] || [ "$(cat "${REQ_HASH_FILE}")" != "${NEW_HASH}" ]; then
  echo "delyric-worker: requirements.txt changed, updating venv" >&2
  needs_install=1
fi

if [ "${needs_install}" = "1" ]; then
  # Prepend build tools so transitive sdists (uvloop, etc.) can compile C.
  # --prefer-binary picks wheels when available to minimize compilation surface.
  PATH="${BUILD_TOOLS}:${PATH:-}" "${VENV}/bin/pip" install --quiet --upgrade pip
  PATH="${BUILD_TOOLS}:${PATH:-}" "${VENV}/bin/pip" install --quiet --prefer-binary -r "${REQ_FILE}"
  echo "${NEW_HASH}" > "${REQ_HASH_FILE}"
fi

export LD_LIBRARY_PATH="${NATIVE_LIBS}:/run/opengl-driver/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"
export PATH="${FFMPEG_BIN}:${PATH:-}"

cd "${SRC_DIR}"
exec "${VENV}/bin/python" "${SRC_DIR}/delyric_worker.py"
