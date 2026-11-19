#!/bin/sh
# Build and lint packaging/arch/PKGBUILD inside an archlinux container, from the CURRENT checkout (no tag needed).
#   docker run --rm -v "$PWD:/src:ro" archlinux:latest sh /src/packaging/arch/test.sh
# Runs: makepkg (prepare, build, check, package) as an unprivileged user, namcap on PKGBUILDs and the package,
# a check that packaging/aur/srcinfo.sh agrees with `makepkg --printsrcinfo`, and an install + run of the package.
set -eu
pacman -Syu --noconfirm --needed base-devel go git namcap bash >/dev/null
useradd -m builder
work=/home/builder/pkg; mkdir -p "$work"
version=0.1.0
git config --global --add safe.directory /src
# The PKGBUILD normally downloads the tag tarball; here the same layout is made from the working tree.
mkdir -p /tmp/t && cp -r /src /tmp/t/skigard-$version && rm -rf /tmp/t/skigard-$version/.git /tmp/t/skigard-$version/dist
tar -czf "$work/skigard-$version.tar.gz" -C /tmp/t "skigard-$version"
sed -e "s|^source=.*|source=(\"\$pkgname-\$pkgver.tar.gz\")|" /src/packaging/arch/PKGBUILD > "$work/PKGBUILD"
chown -R builder:builder "$work"
su builder -c "cd $work && makepkg --noconfirm -f"
pkg="$(ls "$work"/skigard-*.pkg.tar.zst)"

echo "== namcap"
namcap "$work/PKGBUILD"
namcap "$pkg"
namcap /src/packaging/aur/skigard-bin/PKGBUILD

echo "== srcinfo.sh agrees with makepkg --printsrcinfo"
for p in /src/packaging/arch/PKGBUILD /src/packaging/aur/skigard-bin/PKGBUILD; do
  d="$(mktemp -d)"; cp "$p" "$d/PKGBUILD"; chown -R builder:builder "$d"
  su builder -c "cd $d && makepkg --printsrcinfo" | sed 's/^[[:space:]]*//' | sort > "$d/real"
  bash /src/packaging/aur/srcinfo.sh "$p" | sed 's/^[[:space:]]*//' | sort > "$d/ours"
  diff "$d/real" "$d/ours" || { echo "srcinfo.sh differs from makepkg for $p"; exit 1; }
done

echo "== install"
pacman -U --noconfirm "$pkg"
skigard version
bash -c 'source /usr/share/bash-completion/completions/skigard' 2>/dev/null || true
test -s /usr/share/zsh/site-functions/_skigard
test -s /usr/share/licenses/skigard/LICENSE
echo "arch package ok"
