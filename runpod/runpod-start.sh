#!/usr/bin/env bash
set -euo pipefail

# RunPod backfill worker startup — mirrors backlot's scripts/runpod-start.sh
# shape: sshd first (so rsync can push/pull while the probe/job run), then a
# mandatory GPU probe, then RUNPOD_JOB mode (self-exits when the job ends —
# this is what stops billing on a completed/failed shard).

echo "=== RunPod Backfill Worker Startup ==="

if command -v sshd &>/dev/null; then
    echo "Starting sshd..."
    for keytype in rsa ecdsa ed25519; do
        keyfile="/etc/ssh/ssh_host_${keytype}_key"
        if [[ ! -f "$keyfile" ]]; then
            ssh-keygen -t "$keytype" -f "$keyfile" -N "" -q
            echo "  Generated $keytype host key"
        fi
    done
    if [[ -n "${PUBLIC_KEY:-}" ]]; then
        mkdir -p ~/.ssh
        echo "$PUBLIC_KEY" >> ~/.ssh/authorized_keys
        chmod 700 ~/.ssh
        chmod 600 ~/.ssh/authorized_keys
        echo "  Authorized key installed"
    fi
    mkdir -p /run/sshd
    /usr/sbin/sshd -D &
    echo "  sshd started on port 22"
fi

mkdir -p /workspace/shard

echo ""
echo "--- Startup GPU probe ---"
if ! python3 -m runpod.startup_probe; then
    echo ""
    echo "!!! STARTUP GPU PROBE FAILED — refusing to process any shard work !!!"
    exit 1
fi
echo "GPU probe passed."

# ============================================
# JOB MODE: run the shard runner and exit
# ============================================
if [ -n "${RUNPOD_JOB:-}" ]; then
    TIMEOUT="${RUNPOD_TIMEOUT:-14400}"
    echo ""
    echo "=== Running shard job ==="
    echo "  Command: $RUNPOD_JOB"
    echo "  Timeout: ${TIMEOUT}s ($(( TIMEOUT / 3600 ))h $(( (TIMEOUT % 3600) / 60 ))m)"
    echo ""

    set +e
    timeout "$TIMEOUT" bash -c "$RUNPOD_JOB"
    EXIT_CODE=$?
    set -e

    if [ $EXIT_CODE -eq 124 ]; then
        echo ""
        echo "!!! SHARD JOB TIMED OUT after ${TIMEOUT}s !!!"
    elif [ $EXIT_CODE -eq 0 ]; then
        echo ""
        echo "=== Shard job completed successfully ==="
    else
        echo ""
        echo "=== Shard job failed (exit code: $EXIT_CODE) ==="
    fi

    exit $EXIT_CODE
fi

# ============================================
# INTERACTIVE MODE: keep alive for SSH
# ============================================
echo ""
echo "=== Interactive mode: staying alive for SSH ==="
exec sleep infinity
