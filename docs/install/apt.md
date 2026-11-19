# apt (Debian, Ubuntu)

Signed repository at `https://releases.ambierhq.com/apt`, suite `stable`, component `main`, architectures `amd64` and `arm64`.
Works on Debian 12+ and Ubuntu 22.04+. No `apt-key`.

```sh
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://releases.ambierhq.com/apt/key.gpg | sudo tee /etc/apt/keyrings/ambierhq.asc >/dev/null
echo "deb [signed-by=/etc/apt/keyrings/ambierhq.asc] https://releases.ambierhq.com/apt stable main" | sudo tee /etc/apt/sources.list.d/ambierhq.list
sudo apt update && sudo apt install skigard
```

Check the key before you trust it: `gpg --show-keys /etc/apt/keyrings/ambierhq.asc` must show the fingerprint published at
https://github.com/ambierhq/skigard/blob/main/SECURITY.md (the maintainers publish it there next to the release signature
instructions). `signed-by` limits that key to this one repository.

Without the repository: `sudo apt install ./skigard_<version>_linux_amd64.deb` from the GitHub release (the file is
named `skigard_<version>_linux_<arch>.deb` there).

Remove: `sudo apt remove skigard && sudo rm /etc/apt/sources.list.d/ambierhq.list /etc/apt/keyrings/ambierhq.asc`.

How it is built: `packaging/repos/build-apt.sh` from the release's `.deb` files, signed `Release.gpg` and `InRelease`;
only the last three versions stay in the pool.
