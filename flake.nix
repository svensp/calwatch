{
  description = "CalWatch - A lightweight CalDAV directory watcher daemon";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = {
          default = self.packages.${system}.calwatch;
          calwatch = pkgs.callPackage ./default.nix { };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            go-tools
            golangci-lint
            delve
          ];

          shellHook = ''
            echo "Go development environment loaded"
            echo "Go version: $(go version)"
            export GOPATH=$PWD/.go
            export PATH=$GOPATH/bin:$PATH
            mkdir -p $GOPATH
          '';
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.calwatch}/bin/calwatch";
        };
      }
    ) // {
      nixosModules.default = import ./nixos-module.nix;
      nixosModules.calwatch = import ./nixos-module.nix;

      # For backwards compatibility
      nixosModule = import ./nixos-module.nix;
    };
}