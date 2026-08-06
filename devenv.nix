{ pkgs, ... }:

let
  # Pinned Music-Source-Separation-Training (MSST) checkout — same rev/
  # sha256 as nix/delyric-worker.nix's DELYRIC_MSST_DIR. This is the
  # dev-shell equivalent of what nix/wrapper.sh exports for the systemd
  # deployment: a build-time-immutable checkout, no runtime bootstrap needed.
  msstSrc = pkgs.fetchFromGitHub {
    owner = "ZFTurbo";
    repo = "Music-Source-Separation-Training";
    rev = "ccf86c105f55a03e4df3b294e8d27613fef80c1f";
    sha256 = "1f600gfhkfyhw313ianq4r99slpq7i6m98a32sbrq5mzng385rv9";
  };
in
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

    # Native libs needed by pip-installed numpy/torch (MSST inference deps)
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

    # Immutable, pinned MSST checkout (see nix/delyric-worker.nix for the
    # same pin) — delyric.py's resolve_msst_dir() requires this and fails
    # loudly at startup if it's unset or doesn't look like a real checkout.
    export DELYRIC_MSST_DIR="${msstSrc}"

    # Persistent location for MSST's sha256-verified checkpoint downloads
    # (~3.3GB) so repeated `just delyric` runs in this dev shell don't
    # re-download them every time.
    export DELYRIC_MODEL_DIR="$DEVENV_STATE/delyric-models"

    # Python venv for MSST inference deps (GPU — pip wheels, no Nix CUDA rebuild)
    export DELYRIC_VENV="$DEVENV_STATE/delyric-venv"
    DELYRIC_REQUIREMENTS="$DEVENV_ROOT/requirements.txt"
    DELYRIC_REQ_HASH_FILE="$DELYRIC_VENV/.requirements-hash"
    # Health check: `-f` follows the symlink, so if the Nix store target of
    # .../bin/python3 was GC'd the check fails and we rebuild. Without this,
    # a partially-GC'd venv survives and silently breaks long runs.
    if [ ! -x "$DELYRIC_VENV/bin/python3" ] \
       || ! "$DELYRIC_VENV/bin/python3" -c 'import sys' >/dev/null 2>&1; then
      echo "Setting up delyric venv from requirements.txt..."
      rm -rf "$DELYRIC_VENV"
      python3 -m venv "$DELYRIC_VENV" --system-site-packages
      "$DELYRIC_VENV/bin/pip" install --quiet -r "$DELYRIC_REQUIREMENTS"
      sha256sum "$DELYRIC_REQUIREMENTS" | cut -d' ' -f1 > "$DELYRIC_REQ_HASH_FILE"
    elif [ ! -f "$DELYRIC_REQ_HASH_FILE" ] \
         || [ "$(cat "$DELYRIC_REQ_HASH_FILE")" != "$(sha256sum "$DELYRIC_REQUIREMENTS" | cut -d' ' -f1)" ]; then
      echo "delyric requirements.txt changed, updating venv..."
      "$DELYRIC_VENV/bin/pip" install --quiet -r "$DELYRIC_REQUIREMENTS"
      sha256sum "$DELYRIC_REQUIREMENTS" | cut -d' ' -f1 > "$DELYRIC_REQ_HASH_FILE"
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
