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

    # Python (for delyric.py vocal separation pipeline)
    (pkgs.python3.withPackages (ps: [
      ps.click
      ps.tqdm
      ps.pytest
    ]))

    # Runtime dependencies
    pkgs.yt-dlp
    pkgs.ffmpeg

    # Build tooling
    pkgs.just
  ];

  enterShell = ''
    export GOEXPERIMENT=jsonv2
    export GOPATH="$DEVENV_STATE/go"
    export GOMODCACHE="$GOPATH/pkg/mod"
    export PATH="$GOPATH/bin:$PATH"

    if ! command -v goimports &>/dev/null; then
      go install golang.org/x/tools/cmd/goimports@latest
    fi

    # Python venv for audio-separator (GPU — pip wheels, no Nix CUDA rebuild)
    export DELYRIC_VENV="$DEVENV_STATE/delyric-venv"
    if [ ! -f "$DELYRIC_VENV/bin/audio-separator" ]; then
      echo "Setting up delyric venv with audio-separator[gpu]..."
      python3 -m venv "$DELYRIC_VENV" --system-site-packages
      "$DELYRIC_VENV/bin/pip" install --quiet "audio-separator[gpu]"
    fi
    export PATH="$DELYRIC_VENV/bin:$PATH"
  '';
}
