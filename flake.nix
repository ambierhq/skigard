{
  description = "skigard: lockfile and review gate for agent skills";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAll (pkgs: rec {
        skigard = pkgs.buildGoModule {
          pname = "skigard";
          # Flake builds are of whatever commit you point at; tagged releases are packaged in nixpkgs.
          version = "0.0.0-${self.shortRev or self.dirtyShortRev or "dev"}";
          src = self;
          subPackages = [ "cmd/skigard" ];
          # go.mod requires the newest Go patch release and nixpkgs can lag by a patch: build with the Go nixpkgs has
          # (the code does not need the newer patch). Drop this once nixpkgs catches up.
          postPatch = ''
            sed -i -E 's/^go [0-9]+\.[0-9]+(\.[0-9]+)?$/go ${pkgs.go.version}/' go.mod
          '';
          # Hash of `go mod vendor`. Refresh with: mise x -- python3 scripts/nix-vendor-hash.py
          # (CI builds with nix and fails if it is stale).
          vendorHash = "sha256-pbA/AlBz3cQYRTMnQ/qBPcinYOKokrBLNhkbRTq54gE=";
          env.CGO_ENABLED = 0;
          flags = [ "-trimpath" ];
          ldflags = [
            "-s" "-w"
            "-X github.com/ambierhq/skigard/internal/cli.Version=${self.shortRev or self.dirtyShortRev or "dev"}"
          ];
          nativeBuildInputs = [ pkgs.installShellFiles ];
          postInstall = pkgs.lib.optionalString (pkgs.stdenv.buildPlatform.canExecute pkgs.stdenv.hostPlatform) ''
            installShellCompletion --cmd skigard \
              --bash <($out/bin/skigard completion bash) \
              --zsh <($out/bin/skigard completion zsh) \
              --fish <($out/bin/skigard completion fish)
          '';
          meta = {
            description = "Lockfile and review gate for agent skills";
            homepage = "https://github.com/ambierhq/skigard";
            license = pkgs.lib.licenses.mit;
            mainProgram = "skigard";
          };
        };
        default = skigard;
      });

      apps = forAll (pkgs: rec {
        skigard = {
          type = "app";
          program = "${self.packages.${pkgs.system}.skigard}/bin/skigard";
        };
        default = skigard;
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go golangci-lint editorconfig-checker git ];
        };
      });

      formatter = forAll (pkgs: pkgs.nixfmt-rfc-style);
    };
}
