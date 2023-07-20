{
  description = "polydactyl nix package";

  # Nixpkgs / NixOS version to use.
  inputs.nixpkgs.url = "nixpkgs/nixos-22.11"; # We could use nixos-unstable but.. why ?
  inputs.nixpkgs-unstable.url = "nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs, nixpkgs-unstable }:
    let

      # Generate a user-friendly version number.
      version = builtins.substring 0 8 self.lastModifiedDate;

      # System types to support.
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];

      # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      # Nixpkgs instantiated for supported system types.
      nixpkgsFor = forAllSystems (system: import nixpkgs {
        inherit system;
        # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
        config = {
          allowBroken = true;
        };
        overlays = [
          # self.overlay
        ];
      });
      # Nixpkgs-unstable instantiated for supported system types.
      nixpkgsUnstableFor = forAllSystems (system: import nixpkgs-unstable {
        inherit system;
        # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
        config = {
          allowBroken = true;
        };
        overlays = [
          # self.overlay
        ];
      });

    in
    {

      # Provide some binary packages for selected system types.
      packages = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          polydactyl = pkgs.buildGo120Module {
            pname = "polydactyl";
            inherit version;
            # In 'nix develop', we don't need a copy of the source tree
            # in the Nix store.
            src = ./.;
            subPackages = [ "cmd/polydactyl" ];

            # We use vendor, no need for vendorSha256
            vendorSha256 = null;
          };
          docker =
            let
              polydactyl = self.defaultPackage.${system};
            in
            pkgs.dockerTools.buildLayeredImage {
              name = polydactyl.pname;
              tag = polydactyl.version;
              contents = [ polydactyl ];

              config = {
                Cmd = [ "/bin/polydactyl" ];
                WorkingDir = "/";
              };
            };
        });

      # The default package for 'nix build'. This makes sense if the
      # flake provides only one package or there is a clear "main"
      # package.
      defaultPackage = forAllSystems (system: self.packages.${system}.polydactyl);

      checks = forAllSystems (system: { });

      devShells = forAllSystems
        (system:
          let
            pkgs = nixpkgsFor.${system};
            pkgs-unstable = nixpkgsUnstableFor.${system};
          in
          {
            default =
              pkgs.mkShell
                {
                  inherit (self.checks.${system}.pre-commit-check) shellHook;
                  buildInputs = with pkgs; [
                    go_1_18
                    gotools
                    golangci-lint
                    gopls
                    go-outline
                    gopkgs
                    pre-commit
                    # FIXME remove when 22.11 is released
                    pkgs-unstable.revive
                  ];
                };
          });
    };
}
