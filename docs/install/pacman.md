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

## AUR

`skigard` builds the tagged source (runs the tests in `check()`), `skigard-bin` installs the release binary:

```sh
yay -S skigard        # or: paru -S skigard-bin
```

The PKGBUILDs live in [`packaging/arch`](../../packaging/arch/PKGBUILD) and [`packaging/aur`](../../packaging/aur). Read the PKGBUILD
before you build, as with any AUR package. The AUR is a convenience channel; it is not the official Arch repositories.
