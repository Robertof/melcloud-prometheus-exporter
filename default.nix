{ stdenv }:

stdenv.mkDerivation rec {
  name = "melcloud-prometheus-exporter-${version}";
  version = "1.0.0";

  # Must be produced separately on the host system, see `deployment` folder
  src = [ ./melcloud-prometheus-exporter.bin ];

  unpackPhase = ''
    for srcFile in $src; do
      cp $srcFile $(stripHash $srcFile)
    done
  '';

  installPhase = ''
    install -m755 -D melcloud-prometheus-exporter.bin $out/bin/melcloud-prometheus-exporter
  '';

  meta = with stdenv.lib; {
    description = "Prometheus exporter for MELCloud connected devices.";
    platforms = platforms.linux;
  };
}
