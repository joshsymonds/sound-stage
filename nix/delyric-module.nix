# NixOS module for the delyric vocal-separation worker.
#
# Expected usage from a flake-based nix-config, on a GPU host (NixOS,
# NVIDIA driver, library on an NFS automount):
#
#   inputs.sound-stage.url = "github:joshsymonds/sound-stage";
#   # …
#   imports = [ inputs.sound-stage.nixosModules.default ];
#
#   services.delyric-worker = {
#     enable = true;
#     library = "/mnt/music/sound-stage";
#     openFirewall = true;
#   };
#
# Service binds 0.0.0.0:9001 by default — the sound-stage server calls it
# over the LAN. The underlying package (nix/delyric-worker.nix) bootstraps a
# Python venv (audio-separator[gpu], torch, …) into StateDirectory on first
# start; that first start can take a long time (multi-GB downloads) and
# needs outbound network and a writable state dir.
{
  config,
  lib,
  ...
}: let
  cfg = config.services.delyric-worker;
  inherit (lib) mkEnableOption mkIf mkOption types;
in {
  options.services.delyric-worker = {
    enable = mkEnableOption "the delyric vocal-separation worker";

    package = mkOption {
      type = types.package;
      description = "The delyric-worker package to run.";
    };

    bindHost = mkOption {
      type = types.str;
      default = "0.0.0.0";
      description = "Host the listener binds to. Defaults to all interfaces so the sound-stage server can reach it over the LAN.";
    };

    port = mkOption {
      type = types.port;
      default = 9001;
      description = "TCP port the HTTP server binds.";
    };

    library = mkOption {
      type = types.path;
      default = "/mnt/music/sound-stage";
      description = "Song library root. Must match the sound-stage server's libraryDir so songPath validation in /process accepts its requests.";
    };

    openFirewall = mkEnableOption "opening the firewall for the delyric-worker port";
  };

  config = mkIf cfg.enable {
    users.users.delyric-worker = {
      isSystemUser = true;
      group = "delyric-worker";
      description = "delyric-worker service user";
    };
    users.groups.delyric-worker = {};

    networking.firewall.allowedTCPPorts = mkIf cfg.openFirewall [cfg.port];

    systemd.services.delyric-worker = {
      description = "delyric vocal-separation worker";
      after = ["network-online.target"];
      wants = ["network-online.target"];
      wantedBy = ["multi-user.target"];

      # The library lives on an NFS automount; RequiresMountsFor resolves
      # whichever mount/automount unit currently covers this path instead of
      # hardcoding one, so it works whether the mount is already active or
      # needs to be triggered. There's no dedicated NixOS option for this
      # directive, so it goes through unitConfig (see
      # nixos/lib/systemd-unit-options.nix).
      unitConfig.RequiresMountsFor = cfg.library;

      environment = {
        DELYRIC_BIND_HOST = cfg.bindHost;
        DELYRIC_PORT = toString cfg.port;
        DELYRIC_LIBRARY = cfg.library;
        # The wrapper requires DELYRIC_STATE_DIR (systemd StateDirectory);
        # %S is the state directory root (/var/lib), not the per-unit
        # subdirectory, so the "delyric-worker" name below must match
        # StateDirectory= exactly.
        DELYRIC_STATE_DIR = "%S/delyric-worker";
        # audio-separator downloads ~2.5GB of model checkpoints and defaults
        # to /tmp/audio-separator-models; PrivateTmp=true wipes /tmp on every
        # restart, which would re-download that on every restart without a
        # persistent location. Point it at StateDirectory instead.
        DELYRIC_MODEL_DIR = "%S/delyric-worker/models";
        # The service user has no home directory; the first-run pip
        # bootstrap and torch write to ~/.cache. An unset/unwritable HOME
        # would fail first start after the multi-GB bootstrap already ran.
        HOME = "%S/delyric-worker";
      };

      serviceConfig = {
        ExecStart = "${cfg.package}/bin/delyric-worker";

        User = "delyric-worker";
        Group = "delyric-worker";
        Restart = "on-failure";
        # Longer than a typical service's RestartSec: a crash-loop here would
        # otherwise re-trigger the multi-GB venv bootstrap repeatedly.
        RestartSec = "30s";
        # The wrapper bootstraps a multi-GB pip venv (torch, audio-separator)
        # on first run and only binds the port once that completes — the
        # default TimeoutStartSec would kill it mid-install.
        TimeoutStartSec = "infinity";

        StateDirectory = "delyric-worker";

        # Hardening (see systemd.exec(5)), based on the services.sound-stage
        # module's block. Relaxed only for GPU access and the first-run pip
        # bootstrap; everything else is unchanged.
        NoNewPrivileges = true;
        PrivateTmp = true;
        # PrivateDevices=true would hide the NVIDIA character devices behind
        # systemd's private, minimal /dev — the worker needs the real ones to
        # reach the GPU. DeviceAllow + DevicePolicy=closed below scope cgroup
        # device access back down to just those devices.
        PrivateDevices = false;
        DeviceAllow = [
          # CUDA control/UVM devices — see
          # https://docs.nvidia.com/dgx/pdf/dgx-os-5-user-guide.pdf
          "char-nvidiactl"
          "char-nvidia-caps"
          "char-nvidia-frontend"
          "char-nvidia-uvm"
        ];
        DevicePolicy = "closed";
        # ProtectSystem=strict only makes the hierarchy read-only (per
        # systemd.exec(5)); it does not hide /run/opengl-driver, so the
        # NVIDIA userspace libraries the wrapper loads via LD_LIBRARY_PATH
        # stay reachable without further relaxation.
        ProtectSystem = "strict";
        ProtectHome = true;
        ProtectHostname = true;
        ProtectClock = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectKernelLogs = true;
        ProtectControlGroups = true;
        ProtectProc = "invisible";
        ProcSubset = "pid";
        # The library is where separated audio is written; StateDirectory
        # (the venv) is already made writable automatically.
        ReadWritePaths = [cfg.library];
        # AF_INET/AF_INET6 stay enabled (unlike a fully locked-down service)
        # so the first-run pip install and model downloads can reach the
        # network.
        RestrictAddressFamilies = ["AF_UNIX" "AF_INET" "AF_INET6"];
        RestrictNamespaces = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        LockPersonality = true;
        # Left disabled (unlike sound-stage's baseline true): CUDA's PTX JIT
        # and libffi trampolines can require writable+executable mappings,
        # and GPU behavior can't be validated before this ships to the real
        # host. Re-tighten to true once deployment validation on gnomon
        # confirms the worker runs correctly with it enabled.
        MemoryDenyWriteExecute = false;
        SystemCallArchitectures = "native";
        SystemCallFilter = ["@system-service" "~@privileged" "~@resources"];
        CapabilityBoundingSet = [""];
        AmbientCapabilities = [""];
        UMask = "0077";
      };
    };
  };
}
