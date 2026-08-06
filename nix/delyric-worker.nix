{
  lib,
  stdenv,
  bash,
  python3,
  ffmpeg,
  zlib,
  binutils,
  gnumake,
  fetchFromGitHub,
}: let
  nativeLibs = lib.makeLibraryPath [stdenv.cc.cc.lib zlib];
  # Build tools for pip's C-extension compilation (some transitive deps ship
  # only sdists — e.g. uvloop's bitpack.c). Without these, pip install loops
  # forever failing on "gcc: command not found" under the hardened unit PATH.
  buildToolsBin = lib.makeBinPath [stdenv.cc binutils gnumake];

  # Pinned Music-Source-Separation-Training (MSST) checkout — provides the
  # inference.py/ensemble.py CLIs the karaoke ensemble runs as subprocesses.
  # Fetched at build time so the store path is immutable and needs no
  # runtime bootstrap, unlike the pip venv below (which still installs
  # lazily on first start). Bump the rev deliberately, not casually — the
  # ensemble's model configs/checkpoints are validated against this exact
  # commit.
  msstSrc = fetchFromGitHub {
    owner = "ZFTurbo";
    repo = "Music-Source-Separation-Training";
    rev = "ccf86c105f55a03e4df3b294e8d27613fef80c1f";
    sha256 = "1f600gfhkfyhw313ianq4r99slpq7i6m98a32sbrq5mzng385rv9";
  };
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
        --subst-var-by bash          "${bash}" \
        --subst-var-by srcDir        "$out/share/delyric-worker" \
        --subst-var-by python        "${python3}/bin/python" \
        --subst-var-by ffmpegBin     "${ffmpeg}/bin" \
        --subst-var-by nativeLibs    "${nativeLibs}" \
        --subst-var-by buildToolsBin "${buildToolsBin}" \
        --subst-var-by msstDir       "${msstSrc}"
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
