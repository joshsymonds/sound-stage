{
  lib,
  buildNpmPackage,
  importNpmLock,
  nodejs,
}:
buildNpmPackage {
  pname = "sound-stage-web";
  version = "0.1.0";

  src = lib.fileset.toSource {
    root = ../web;
    fileset = lib.fileset.unions [
      ../web/package.json
      ../web/package-lock.json
      ../web/svelte.config.js
      ../web/vite.config.ts
      ../web/tsconfig.json
      ../web/eslint.config.ts
      ../web/vitest-setup.ts
      ../web/src
      ../web/static
    ];
  };

  npmDeps = importNpmLock {npmRoot = ../web;};
  npmConfigHook = importNpmLock.npmConfigHook;
  inherit nodejs;

  # Playwright postinstall otherwise tries to download Chromium from the
  # internet, which the Nix build sandbox blocks. We only need vite build,
  # not browser tests, in this derivation.
  env.PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";

  npmBuildScript = "build";

  installPhase = ''
    runHook preInstall
    mkdir -p $out
    cp -r build/. $out/
    runHook postInstall
  '';

  meta = {
    description = "Compiled SvelteKit SPA for sound-stage";
    license = lib.licenses.mit;
  };
}
