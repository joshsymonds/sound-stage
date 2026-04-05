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
  '';
}
