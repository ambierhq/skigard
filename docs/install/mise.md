# mise

```sh
mise use -g github:ambierhq/skigard      # works with the current release (v0.1.1)
mise use -g ubi:ambierhq/skigard         # same archives; mise has deprecated the ubi backend in favour of github:
mise use -g skigard                      # needs a registry entry (not submitted)
```

Release archives are named `skigard_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) with the binary at the archive root,
which is what these backends rely on. Also works with `eget` and `aqua` once the registry entry is merged.
