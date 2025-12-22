{
  description = "Wails planner application";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        # Darwin-specific dependencies
        darwinDeps = pkgs.lib.optionals pkgs.stdenv.isDarwin [
          pkgs.libiconv
        ];
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Wails CLI
            wails

            # Go toolchain
            go

            # Node.js for frontend
            nodejs_20

            # Build tools
            pkg-config

            # macOS dependencies
          ] ++ darwinDeps;

          shellHook = ''
            # Set CGO flags to use the Nix-provided SDK
            export CGO_ENABLED=1

            # For macOS, set up the SDK path
            ${pkgs.lib.optionalString pkgs.stdenv.isDarwin ''
              export CGO_CFLAGS="-Wno-nullability-completeness"
            ''}
          '';
        };
      }
    );
}
