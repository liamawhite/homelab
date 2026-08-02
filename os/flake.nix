{
  description = "NixOS config for nix-gpu-3070-1, the RTX 3070 desktop k3s control-plane node";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    disko = {
      url = "github:nix-community/disko";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, disko }: {
    nixosConfigurations.nix-gpu-3070-1 = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        disko.nixosModules.disko
        ./hosts/nix-gpu-3070-1
      ];
    };
  };
}
