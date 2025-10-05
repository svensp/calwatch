{ lib
, buildGoModule
, fetchFromGitHub
}:

buildGoModule rec {
  pname = "calwatch";
  version = "0.3.0";

  src = ./.;

  vendorHash = "sha256-VrmRmdmc5xg0+sXsEonQwe72Xgr8YyAXZVkzeVUjNy8=";

  subPackages = [ "cmd/calwatch" ];

  postInstall = ''
    # Install templates
    mkdir -p $out/share/calwatch
    cp -r templates $out/share/calwatch/

    # Install example configuration
    cp config.example.yaml $out/share/calwatch/

    # Install documentation
    mkdir -p $out/share/doc/calwatch
    cp README.md $out/share/doc/calwatch/
    cp -r docs $out/share/doc/calwatch/
  '';

  meta = with lib; {
    description = "A lightweight CalDAV directory watcher daemon for Linux desktop environments";
    longDescription = ''
      CalWatch monitors your local CalDAV directories (synced via vdirsyncer) and 
      sends desktop notifications for upcoming calendar events. It uses pure Go 
      D-Bus integration for notifications, eliminating runtime dependencies.
    '';
    homepage = "https://github.com/yourusername/calwatch";
    license = licenses.mit;
    maintainers = with maintainers; [ ];
    platforms = platforms.linux;
    mainProgram = "calwatch";
  };
}