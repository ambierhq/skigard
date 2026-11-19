# Draft for nixpkgs: pkgs/by-name/sk/skigard/package.nix (needs a maintainer entry in maintainers/maintainer-list.nix).
# Do not submit before the first release. At that time: set `version`, `hash` (nix-prefetch-url --unpack, or build once with
# lib.fakeHash) and `vendorHash` (scripts/nix-vendor-hash.py gives the same value; confirm with a build).
{
  lib,
  buildGoModule,
  fetchFromGitHub,
  installShellFiles,
  stdenv,
}:

buildGoModule (finalAttrs: {
  pname = "skigard";
  version = "0.1.0";

  src = fetchFromGitHub {
    owner = "ambierhq";
    repo = "skigard";
    tag = "v${finalAttrs.version}";
    hash = lib.fakeHash;
  };

  vendorHash = "sha256-pbA/AlBz3cQYRTMnQ/qBPcinYOKokrBLNhkbRTq54gE=";

  subPackages = [ "cmd/skigard" ];

  env.CGO_ENABLED = 0;

  ldflags = [
    "-s"
    "-w"
    "-X github.com/ambierhq/skigard/internal/cli.Version=${finalAttrs.version}"
  ];

  nativeBuildInputs = [ installShellFiles ];

  postInstall = lib.optionalString (stdenv.buildPlatform.canExecute stdenv.hostPlatform) ''
    installShellCompletion --cmd skigard \
      --bash <($out/bin/skigard completion bash) \
      --zsh <($out/bin/skigard completion zsh) \
      --fish <($out/bin/skigard completion fish)
  '';

  doInstallCheck = true;
  installCheckPhase = ''
    runHook preInstallCheck
    $out/bin/skigard version | grep -F "${finalAttrs.version}"
    runHook postInstallCheck
  '';

  meta = {
    description = "Lockfile and review gate for agent skills";
    homepage = "https://github.com/ambierhq/skigard";
    changelog = "https://github.com/ambierhq/skigard/releases/tag/v${finalAttrs.version}";
    license = lib.licenses.mit;
    mainProgram = "skigard";
    maintainers = [ ]; # add the owner's entry
  };
})
