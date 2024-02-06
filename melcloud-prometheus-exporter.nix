{ lib, pkgs, options, config, ... }:
with lib;                      
let
  melcloudPrometheusExporter = pkgs.callPackage ./default.nix {};
  cfg = config.services.melcloud-prometheus-exporter;
  configPath = pkgs.writeText "melcloud-prometheus-exporter.json" (builtins.toJSON cfg.config);
in {
  options.services.melcloud-prometheus-exporter = {
    enable = mkEnableOption "melcloud-prometheus-exporter service";
    config = mkOption { };
  };

  config = mkIf cfg.enable {
    environment.systemPackages = [ melcloudPrometheusExporter ];

    systemd.services.melcloud-prometheus-exporter = {
      wantedBy = [ "multi-user.target" ];
      after = ["networking.target"];
      serviceConfig = {
        User = "melcloud-prometheus-exporter";
        ExecStart = "${melcloudPrometheusExporter}/bin/melcloud-prometheus-exporter '${configPath}'";
        DynamicUser = true;
        Restart = "on-failure";
        RestartSec = 180;
      };
    };
  };
}
