{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.calwatch;
  calwatch = pkgs.callPackage ./default.nix { };
  
  # Default configuration
  defaultConfig = {
    directories = [];
    notification = {
      backend = "dbus";
      duration = 5000;
    };
    logging = {
      level = "info";
    };
  };

  configFile = pkgs.writeText "calwatch-config.yaml" (builtins.toJSON cfg.settings);
in
{
  options.services.calwatch = {
    enable = mkEnableOption "CalWatch CalDAV directory watcher daemon";

    package = mkOption {
      type = types.package;
      default = calwatch;
      defaultText = literalExpression "pkgs.calwatch";
      description = "The CalWatch package to use.";
    };

    user = mkOption {
      type = types.str;
      default = "calwatch";
      description = "User under which CalWatch runs.";
    };

    group = mkOption {
      type = types.str;
      default = "calwatch";
      description = "Group under which CalWatch runs.";
    };

    settings = mkOption {
      type = types.attrs;
      default = defaultConfig;
      description = ''
        CalWatch configuration. See config.example.yaml for available options.
        
        Example:
        {
          directories = [
            {
              directory = "/home/user/.calendars/personal";
              template = "detailed.tpl";
              automatic_alerts = [
                { value = 15; unit = "minutes"; }
                { value = 1; unit = "hours"; }
              ];
            }
          ];
          notification = {
            backend = "dbus";
            duration = 5000;
          };
          logging = {
            level = "info";
          };
        }
      '';
    };

    createUser = mkOption {
      type = types.bool;
      default = true;
      description = "Whether to create the CalWatch user and group.";
    };

    dataDir = mkOption {
      type = types.path;
      default = "/var/lib/calwatch";
      description = "Directory where CalWatch stores its data.";
    };
  };

  config = mkIf cfg.enable {
    users.users = optionalAttrs cfg.createUser {
      ${cfg.user} = {
        description = "CalWatch daemon user";
        group = cfg.group;
        home = cfg.dataDir;
        createHome = true;
        isSystemUser = true;
      };
    };

    users.groups = optionalAttrs cfg.createUser {
      ${cfg.group} = {};
    };

    systemd.services.calwatch = {
      description = "CalWatch - CalDAV Directory Watcher Daemon";
      documentation = [ "https://github.com/yourusername/calwatch" ];
      after = [ "network.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        Type = "simple";
        User = cfg.user;
        Group = cfg.group;
        ExecStart = "${cfg.package}/bin/calwatch";
        Restart = "on-failure";
        RestartSec = 5;

        # Security settings
        NoNewPrivileges = true;
        PrivateTmp = true;
        PrivateDevices = true;
        ProtectSystem = "strict";
        ProtectHome = "read-only";
        ReadWritePaths = [ cfg.dataDir ];

        # Logging
        StandardOutput = "journal";
        StandardError = "journal";
        SyslogIdentifier = "calwatch";

        # Environment for D-Bus notifications
        Environment = [
          "XDG_CONFIG_HOME=${cfg.dataDir}/.config"
          "XDG_DATA_HOME=${cfg.dataDir}/.local/share"
          "XDG_CACHE_HOME=${cfg.dataDir}/.cache"
        ];
      };

      preStart = ''
        # Create config directory
        mkdir -p ${cfg.dataDir}/.config/calwatch
        
        # Copy configuration
        cp ${configFile} ${cfg.dataDir}/.config/calwatch/config.yaml
        
        # Copy default templates if they don't exist
        if [ ! -d ${cfg.dataDir}/.config/calwatch/templates ]; then
          cp -r ${cfg.package}/share/calwatch/templates ${cfg.dataDir}/.config/calwatch/
        fi
        
        # Set proper permissions
        chown -R ${cfg.user}:${cfg.group} ${cfg.dataDir}
      '';
    };

    # Add calwatch package to system packages
    environment.systemPackages = [ cfg.package ];
  };
}