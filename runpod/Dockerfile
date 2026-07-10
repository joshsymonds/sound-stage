# RunPod backfill worker image — one community-cloud RTX 4090 pod per shard.
#
# Base family matches backlot's proven runpod/pytorch cu1300-torch*-ubuntu2404
# line (see ~/Personal/backlot/Dockerfile on gnomon). There is no cu1300-torch213
# tag: audio-separator[gpu]==0.44.3 (requirements.txt) pulls its own torch
# 2.13.0+cu130 wheel via pip below, so the base's bundled torch291 is just a
# starting point pip upgrades from — unlike backlot's ai-toolkit/ComfyUI stack,
# nothing here depends on the base image's own torch/torchvision pairing.
FROM runpod/pytorch:1.0.3-cu1300-torch291-ubuntu2404

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=compute,utility

WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends ffmpeg && \
    rm -rf /var/lib/apt/lists/*

# requirements.txt layer is cache-friendly on its own (rebuilds only when
# deps change, not on every delyric.py/runpod/ edit below).
COPY requirements.txt requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --no-cache-dir -r requirements.txt

# Mirrors the repo's relative layout (delyric.py at the root, runpod/ beside
# it) so `import delyric` resolves identically here and in runpod/conftest.py's
# test-session shim — see runpod/supervisor.py's module docstring.
COPY delyric.py delyric.py
COPY runpod/*.py runpod/
COPY runpod/runpod-start.sh runpod/runpod-start.sh
RUN chmod +x runpod/runpod-start.sh

# Bundled ~10s sine-wave clip for the startup GPU probe (runpod/startup_probe.py)
# — synthesized at build time so no binary fixture is committed to git. A pure
# tone is sufficient: the probe only measures whether a full ensemble
# separation pass completes within the time budget, not audio quality.
RUN mkdir -p runpod/testclip && \
    ffmpeg -y -f lavfi -i "sine=frequency=440:duration=10" \
      -c:a libopus -b:a 128k runpod/testclip/audio.webm

EXPOSE 22
CMD ["runpod/runpod-start.sh"]
