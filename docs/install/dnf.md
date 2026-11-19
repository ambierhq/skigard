# dnf / yum (Fedora, RHEL and clones)

Signed repository at `https://releases.ambierhq.com/rpm/$basearch` (`x86_64`, `aarch64`). Packages and `repomd.xml` are both signed.

```sh
sudo tee /etc/yum.repos.d/ambierhq.repo >/dev/null <<'REPO'
[ambierhq]
name=Ambier
baseurl=https://releases.ambierhq.com/rpm/$basearch
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://releases.ambierhq.com/rpm/key.gpg
REPO
sudo dnf install skigard
```

dnf asks you to confirm the key fingerprint on first use; compare it with the one in
https://github.com/ambierhq/skigard/blob/main/SECURITY.md.

Without the repository: `sudo dnf install ./skigard_<version>_linux_amd64.rpm` (from the GitHub release).
Remove: `sudo dnf remove skigard && sudo rm /etc/yum.repos.d/ambierhq.repo`.
