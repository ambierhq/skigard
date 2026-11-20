{
  lib,
  buildGoModule,
  fetchFromGitHub,
  installShellFiles,
  nix-update-script,
  stdenv,
  versionCheckHook,
}:

buildGoModule (finalAttrs: {
  pname = "skigard";
  version = "0.1.1";

  src = fetchFromGitHub {
    owner = "ambierhq";
    repo = "skigard";
    tag = "v${finalAttrs.version}";
    hash = "sha256-pfan8utVwoFkuC7JWzyALHbJmfdZaOEV1spixMrEiYk=";
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

  nativeInstallCheckInputs = [ versionCheckHook ];
  versionCheckProgramArg = "version";
  doInstallCheck = true;

  passthru.updateScript = nix-update-script { };

  meta = {
    description = "Lockfile and review gate for agent skills";
    homepage = "https://github.com/ambierhq/skigard";
    changelog = "https://github.com/ambierhq/skigard/releases/tag/v${finalAttrs.version}";
    license = lib.licenses.mit;
    mainProgram = "skigard";
    maintainers = with lib.maintainers; [ pexea12 ];
  };
})
