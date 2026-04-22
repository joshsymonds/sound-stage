{ pkgs, ... }:

{
  dotenv.enable = true;

  packages = [
    # Go
    pkgs.go_1_26
    pkgs.gopls
    pkgs.go-tools       # staticcheck
    pkgs.golangci-lint
    pkgs.delve

    # Python (for delyric.py vocal separation pipeline + FastAPI worker)
    (pkgs.python3.withPackages (ps: [
      ps.click
      ps.tqdm
      ps.pytest
      ps.fastapi
      ps.uvicorn
      ps.httpx
    ]))

    # Web frontend (Svelte 5 + SvelteKit + Storybook)
    pkgs.nodejs_22

    # Runtime dependencies
    pkgs.yt-dlp
    pkgs.ffmpeg

    # Native libs needed by pip-installed numpy/torch (audio-separator[gpu])
    pkgs.stdenv.cc.cc.lib
    pkgs.zlib

    # Build tooling
    pkgs.just
  ];

  enterShell = ''
    export GOEXPERIMENT=jsonv2
    export GOPATH="$DEVENV_STATE/go"
    export GOMODCACHE="$GOPATH/pkg/mod"
    export PATH="$GOPATH/bin:$PATH"

    # Native libs for pip-installed wheels (numpy, torch, etc.)
    export LD_LIBRARY_PATH="${pkgs.stdenv.cc.cc.lib}/lib:${pkgs.zlib}/lib:/run/opengl-driver/lib''${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

    if ! command -v goimports &>/dev/null; then
      go install golang.org/x/tools/cmd/goimports@latest
    fi

    # Python venv for audio-separator (GPU — pip wheels, no Nix CUDA rebuild)
    export DELYRIC_VENV="$DEVENV_STATE/delyric-venv"
    # Health check: `-f` follows the symlink, so if the Nix store target of
    # .../bin/python3 was GC'd the check fails and we rebuild. Without this,
    # a partially-GC'd venv survives and silently breaks long runs.
    if [ ! -x "$DELYRIC_VENV/bin/audio-separator" ] \
       || ! "$DELYRIC_VENV/bin/python3" -c 'import sys' >/dev/null 2>&1; then
      echo "Setting up delyric venv with audio-separator[gpu]..."
      rm -rf "$DELYRIC_VENV"
      python3 -m venv "$DELYRIC_VENV" --system-site-packages
      "$DELYRIC_VENV/bin/pip" install --quiet "audio-separator[gpu]"
    fi
    export PATH="$DELYRIC_VENV/bin:$PATH"

    # Pin a GC root on the venv's Python so determinate-nixd's auto-GC
    # can't delete it mid-run. Without this, exiting the devenv shell drops
    # the last reference and the next GC sweep breaks any in-flight pipeline.
    mkdir -p "$DEVENV_ROOT/.devenv/gc"
    PY_TARGET="$(readlink -f "$DELYRIC_VENV/bin/python3" 2>/dev/null || true)"
    if [ -n "$PY_TARGET" ] && [ -e "$PY_TARGET" ]; then
      nix-store --add-root "$DEVENV_ROOT/.devenv/gc/delyric-python" \
        --indirect --realise "$PY_TARGET" >/dev/null
    fi
  '';

  processes.web.exec = "cd web && npm run dev";
  processes.storybook.exec = "cd web && npm run storybook";
}
