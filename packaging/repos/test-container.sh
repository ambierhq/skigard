#!/bin/sh
# Install skigard from a built repository tree, inside a clean distro container, and run it.
#   test-container.sh apt|rpm|arch TREE      (TREE is the OUTDIR passed to build-all.sh, mounted into the container)
# Uses file:// so no web server is needed; apt, dnf and pacman verify the signatures exactly as they do over HTTPS.
set -eu
kind="$1"; tree="$2"
case "$kind" in
apt)
  export DEBIAN_FRONTEND=noninteractive
  install -d -m 0755 /etc/apt/keyrings
  cp "$tree/apt/key.gpg" /etc/apt/keyrings/ambierhq.asc
  echo "deb [signed-by=/etc/apt/keyrings/ambierhq.asc] file:$tree/apt stable main" > /etc/apt/sources.list.d/ambierhq.list
  apt-get update
  apt-get install -y skigard
  ;;
rpm)
  cat > /etc/yum.repos.d/ambierhq.repo <<REPO
[ambierhq]
name=Ambier
baseurl=file://$tree/rpm/\$basearch
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=file://$tree/rpm/key.gpg
REPO
  dnf install -y skigard
  ;;
arch)
  pacman -Sy --noconfirm --needed gnupg >/dev/null
  pacman-key --init
  fpr="$(gpg --show-keys --with-colons "$tree/arch/key.gpg" | awk -F: '$1=="fpr"{print $10; exit}')"
  pacman-key --add "$tree/arch/key.gpg"
  pacman-key --lsign-key "$fpr"
  # shellcheck disable=SC2016 # $arch is pacman's variable, not ours
  printf '\n[ambierhq]\nSigLevel = Required DatabaseRequired\nServer = file://%s/arch/$arch\n' "$tree" >> /etc/pacman.conf
  pacman -Sy --noconfirm skigard
  ;;
*) echo "usage: $0 apt|rpm|arch TREE" >&2; exit 2 ;;
esac
skigard version
skigard --help >/dev/null
echo "$kind: install ok"
