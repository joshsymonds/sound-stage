{
  lib,
  buildGoModule,
  sound-stage-web,
}:
buildGoModule {
  pname = "sound-stage";
  version = "0.1.0";

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../main.go
      ../embed.go
      ../archive
      ../cmd
      ../server
      ../usdb
      ../ytdlp
      # Sentinel keeps //go:embed all:web/build resolvable before preBuild
      # overlays the real SPA assets.
      ../web/build/.keep
    ];
  };

  vendorHash = "sha256-7K17JaXFsjf163g5PXCb5ng2gYdotnZ2IDKk8KFjNj0=";

  # Stage the SPA into web/build/ so //go:embed picks up real assets.
  preBuild = ''
    rm -rf web/build
    mkdir -p web/build
    cp -r ${sound-stage-web}/. web/build/
  '';

  # Skip tests at package time — ytdlp tests spawn the yt-dlp binary, which
  # the build sandbox doesn't carry. `just test` in dev exercises full
  # coverage; this derivation is for shipping the binary.
  doCheck = false;

  meta = {
    description = "SoundStage karaoke server: Go HTTP API + embedded SvelteKit SPA";
    license = lib.licenses.mit;
    mainProgram = "sound-stage";
  };
}
