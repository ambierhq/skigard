# Arch Linux

## Our pacman repository

Signed packages and database at `https://releases.ambierhq.com/arch/$arch` (`x86_64`, `aarch64`).

```sh
curl -fsSL https://releases.ambierhq.com/arch/key.gpg | sudo pacman-key --add -
sudo pacman-key --lsign-key <FINGERPRINT>      # the fingerprint from SECURITY.md; verify it first
printf '\n[ambierhq]\nSigLevel = Required DatabaseRequired\nServer = https://releases.ambierhq.com/arch/$arch\n' | sudo tee -a /etc/pacman.conf
sudo pacman -Sy skigard
```

Without the repository: `sudo pacman -U <url of skigard_<version>_linux_<arch>.pkg.tar.zst from the GitHub release>`.

## Other routes

There is no AUR package. Install through the ambierhq pacman repository above, or with `pacman -U` of the release asset.
The official Arch repositories would need a Package Maintainer to adopt skigard; until then they are not an option.

To build from source, the PKGBUILD lives in [`packaging/arch`](../../packaging/arch/PKGBUILD). Read it before you build.
