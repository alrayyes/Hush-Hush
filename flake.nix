{
  description = "Client for the hush-hush secrets object store";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "hush-hush-cli";
          version = self.shortRev or self.dirtyShortRev or "dev";

          src = ./.;
          subPackages = [ "cmd/hush-hush-cli" ];

          vendorHash = "sha256-YNjMudR+a2b8vt6xEOkE24guVf9opUpibCqx55jQqBU=";

          # Same man-page generation goreleaser's before hook runs - the
          # freshly built binary walks its own command tree, so this stays
          # in sync with the CLI automatically. Building natively per
          # system (no cross-compilation here) is what makes running the
          # binary during the build safe.
          postInstall = ''
            $out/bin/hush-hush-cli man manpages
            installManPage manpages/*.1
          '';

          nativeBuildInputs = [ pkgs.installShellFiles ];

          meta = with pkgs.lib; {
            description = "Client for the hush-hush secrets object store";
            homepage = "https://github.com/alrayyes/hush-hush";
            license = licenses.gpl3Only;
            mainProgram = "hush-hush-cli";
          };
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };
      }
    );
}
