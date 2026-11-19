# Nix

```sh
nix run github:ambierhq/skigard -- version
nix profile install github:ambierhq/skigard
nix develop github:ambierhq/skigard     # Go, golangci-lint, editorconfig-checker
```

The flake builds with `buildGoModule` from the commit you reference (pin a tag: `github:ambierhq/skigard/v0.1.0`). A nixpkgs
package (`pkgs.skigard`) is planned; its draft is [`packaging/nixpkgs/package.nix`](../../packaging/nixpkgs/package.nix).
