# Installation

The server (`hush-hush`) is container-only - see
[Docker](README.md#docker) in the README. `hush-hush-cli`, the client every
writer and consumer uses, installs through whichever of these fits:

- **Arch Linux (AUR)**: `paru -S hush-hush-cli-bin` (or any other AUR
  helper) - a prebuilt binary, not built from source.
- **Nix (flakes)**: `nix run github:alrayyes/hush-hush` to try it, or
  `nix profile install github:alrayyes/hush-hush` to keep it. No hosted
  binary cache - a first run/install builds from source, same tradeoff as
  every other from-source path here.
- **Debian/Ubuntu**: download the `.deb` from the
  [latest release](https://github.com/alrayyes/hush-hush/releases/latest)
  and `sudo dpkg -i hush-hush-cli_*.deb`.
- **Fedora/RHEL**: download the `.rpm` from the same release page and
  `sudo rpm -i hush-hush-cli_*.rpm`.
- **Anywhere else, or from source**:

  ```sh
  git clone https://github.com/alrayyes/hush-hush.git
  cd hush-hush
  go build ./cmd/hush-hush-cli
  ```

Every path installs a man page too - `man hush-hush-cli` once it's on
`PATH`. None of these are a hosted apt/dnf repository - each is a
downloadable file attached to the GitHub release, not something `apt
update` discovers on its own.
