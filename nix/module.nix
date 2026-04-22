# NixOS module for sound-stage.
#
# Expected usage from a flake-based nix-config:
#
#   inputs.sound-stage.url = "github:joshsymonds/sound-stage";
#   # …
#   imports = [ inputs.sound-stage.nixosModules.default ];
#
#   age.secrets."sound-stage.env".file = ../secrets/sound-stage.env.age;
#
#   services.sound-stage = {
#     enable = true;
#     deckURL = "http://172.31.0.39:9000";
#     delyricURL = "http://172.31.0.98:9001";
#     libraryDir = "/mnt/music/sound-stage";
#     environmentFile = config.age.secrets."sound-stage.env".path;
#   };
#
# The agenix-decrypted file should look like:
#
#   USDB_USERNAME=...
#   USDB_PASSWORD=...
#
# Service binds 127.0.0.1:8080 (loopback only). A reverse proxy (Caddy etc.)
# fronts it on :443 with TLS.
{
  config,
  lib,
  ...
}: let
  cfg = config.services.sound-stage;
  inherit (lib) mkEnableOption mkIf mkOption optionalString types;
in {
  options.services.sound-stage = {
    enable = mkEnableOption "the SoundStage karaoke queue server";

    package = mkOption {
      type = types.package;
      description = "The sound-stage-server package to run.";
    };

    port = mkOption {
      type = types.str;
      default = "8080";
      description = "TCP port the HTTP server binds.";
    };

    bindAddress = mkOption {
      type = types.str;
      default = "127.0.0.1";
      description = ''
        Host the listener binds to. Defaults to loopback so the API is
        only reachable through the fronting reverse proxy. Set to
        "0.0.0.0" to expose on all interfaces (rarely correct).
      '';
    };

    deckURL = mkOption {
      type = types.str;
      example = "http://172.31.0.39:9000";
      description = "USDX Pascal API base URL on the Steam Deck.";
    };

    delyricURL = mkOption {
      type = types.str;
      default = "";
      example = "http://172.31.0.98:9001";
      description = "Optional delyric worker base URL. Empty disables.";
    };

    libraryDir = mkOption {
      type = types.path;
      default = "/mnt/music/sound-stage";
      description = "Directory holding the song library and the .downloaded.txt archive. Must be writable by the service user.";
    };

    user = mkOption {
      type = types.str;
      default = "sound-stage";
      description = "Service user. Stable so NFS mount permissions work across reboots.";
    };

    group = mkOption {
      type = types.str;
      default = "sound-stage";
      description = "Service group.";
    };

    environmentFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      example = "/run/secrets/sound-stage.env";
      description = ''
        Path to a systemd EnvironmentFile holding USDB_USERNAME and
        USDB_PASSWORD. Typically wired to an agenix-decrypted file.
        When null, USDB-dependent features (search, download) are
        disabled at runtime.
      '';
    };
  };

  config = mkIf cfg.enable {
    users.users.${cfg.user} = {
      isSystemUser = true;
      group = cfg.group;
      description = "sound-stage service user";
    };
    users.groups.${cfg.group} = {};

    systemd.services.sound-stage = {
      description = "SoundStage karaoke server";
      after = ["network-online.target"];
      wants = ["network-online.target"];
      wantedBy = ["multi-user.target"];

      serviceConfig = {
        # Persistent flags (--output) come BEFORE the subcommand; serve
        # flags come AFTER. Cobra rejects the reverse.
        ExecStart =
          "${cfg.package}/bin/sound-stage --output ${cfg.libraryDir} serve "
          + "--port ${cfg.port} "
          + "--bind ${cfg.bindAddress} "
          + "--deck-url ${cfg.deckURL}"
          + optionalString (cfg.delyricURL != "") " --delyric-url ${cfg.delyricURL}";

        User = cfg.user;
        Group = cfg.group;
        Restart = "on-failure";
        RestartSec = "5s";

        EnvironmentFile = mkIf (cfg.environmentFile != null) cfg.environmentFile;

        # Hardening (see systemd.exec(5)).
        NoNewPrivileges = true;
        PrivateTmp = true;
        PrivateDevices = true;
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
        ReadWritePaths = [cfg.libraryDir];
        RestrictAddressFamilies = ["AF_UNIX" "AF_INET" "AF_INET6"];
        RestrictNamespaces = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        SystemCallArchitectures = "native";
        SystemCallFilter = ["@system-service" "~@privileged" "~@resources"];
        CapabilityBoundingSet = [""];
        AmbientCapabilities = [""];
        UMask = "0077";
      };
    };
  };
}
