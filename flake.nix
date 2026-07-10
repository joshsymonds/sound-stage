{
  description = "sound-stage: karaoke download + delyric vocal separation worker";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

  outputs = {
    self,
    nixpkgs,
  }: let
    systems = ["x86_64-linux"];
    forAllSystems = f:
      nixpkgs.lib.genAttrs systems (system:
        f (import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        }));
  in {
    packages = forAllSystems (pkgs: rec {
      delyric-worker = pkgs.callPackage ./nix/delyric-worker.nix {};
      sound-stage-web = pkgs.callPackage ./nix/sound-stage-web.nix {};
      sound-stage-server = pkgs.callPackage ./nix/sound-stage-server.nix {
        inherit sound-stage-web;
      };
      default = sound-stage-server;
    });

    nixosModules.default = {
      pkgs,
      lib,
      ...
    }: {
      imports = [./nix/module.nix ./nix/delyric-module.nix];
      services.sound-stage.package = lib.mkDefault self.packages.${pkgs.stdenv.hostPlatform.system}.sound-stage-server;
      services.delyric-worker.package = lib.mkDefault self.packages.${pkgs.stdenv.hostPlatform.system}.delyric-worker;
    };

    # Evaluation-only check: instantiates a minimal NixOS system with
    # services.delyric-worker.enable = true (via the real exported
    # nixosModules.default, so it also exercises the package default wiring
    # above) and forces evaluation of config.system.build.toplevel.drvPath.
    # Referencing .drvPath only requires that derivation to be *instantiated*,
    # not built, so this catches module eval errors (bad option types, typos,
    # assertion failures) cheaply without pulling a full NixOS system build
    # into `nix flake check`.
    checks = forAllSystems (pkgs: let
      system = pkgs.stdenv.hostPlatform.system;
      eval = nixpkgs.lib.nixosSystem {
        inherit system;
        modules = [
          self.nixosModules.default
          {
            services.delyric-worker.enable = true;
            boot.isContainer = true;
            system.stateVersion = "24.05";
          }
        ];
      };
    in {
      delyric-worker-module-eval = pkgs.runCommand "delyric-worker-module-eval" {} ''
        : ${eval.config.system.build.toplevel.drvPath}
        touch $out
      '';
    });
  };
}
