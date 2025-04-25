{
  description = "Devshell flake";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    rust-overlay = {
      inputs = {
        nixpkgs.follows = "nixpkgs";
      };
      url = "github:oxalica/rust-overlay";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      rust-overlay,
    }@inputs:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        overlays = [ (import rust-overlay) ];
        pkgs = import nixpkgs {
          inherit overlays system;
        };

        rustToolchain = pkgs.rust-bin.stable.latest.default.override {
          extensions = [ "rust-src" ];
        };
      in
      {
        formatter = pkgs.nixfmt-rfc-style;

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gofumpt
            golangci-lint
            rustToolchain
          ];
        };
      }
    );
}
