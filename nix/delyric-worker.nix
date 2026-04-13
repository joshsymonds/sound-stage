{
  lib,
  stdenv,
  bash,
  python3,
  ffmpeg,
  zlib,
}: let
  nativeLibs = lib.makeLibraryPath [stdenv.cc.cc.lib zlib];
in
  stdenv.mkDerivation {
    pname = "delyric-worker";
    version = "0.1.0";

    src = lib.fileset.toSource {
      root = ../.;
      fileset = lib.fileset.unions [
        ../delyric.py
        ../delyric_worker.py
        ../requirements.txt
      ];
    };

    dontBuild = true;
    dontConfigure = true;

    installPhase = ''
      runHook preInstall

      mkdir -p $out/share/delyric-worker $out/bin
      cp delyric.py delyric_worker.py requirements.txt $out/share/delyric-worker/

      substitute ${./wrapper.sh} $out/bin/delyric-worker \
        --subst-var-by bash       "${bash}" \
        --subst-var-by srcDir     "$out/share/delyric-worker" \
        --subst-var-by python     "${python3}/bin/python" \
        --subst-var-by ffmpegBin  "${ffmpeg}/bin" \
        --subst-var-by nativeLibs "${nativeLibs}"
      chmod +x $out/bin/delyric-worker

      runHook postInstall
    '';

    meta = {
      description = "FastAPI HTTP wrapper around the delyric vocal separation pipeline";
      license = lib.licenses.mit;
      platforms = lib.platforms.linux;
      mainProgram = "delyric-worker";
    };
  }
