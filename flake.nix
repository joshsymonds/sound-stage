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
    packages = forAllSystems (pkgs: {
      delyric-worker = pkgs.callPackage ./nix/delyric-worker.nix {};
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.delyric-worker;
    });
  };
}
