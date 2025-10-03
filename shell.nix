{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
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
}