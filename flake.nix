{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    utils = { url = "github:numtide/flake-utils"; };
  };
  outputs = { self, nixpkgs, utils }:
    utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        with pkgs;
        {
          devShells.default = pkgs.mkShell {
            buildInputs = [
              crd2pulumi
              git
              jq
              kubernetes-helm
              k9s
              nodejs_23
            ];
          };
        }
      );
}

