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
      imports = [./nix/module.nix];
      services.sound-stage.package = lib.mkDefault self.packages.${pkgs.stdenv.hostPlatform.system}.sound-stage-server;
    };
  };
}
