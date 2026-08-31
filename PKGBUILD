# Maintainer: Ryan Kes <ryan@ryankes.eu>
#
# Binary package, not built from source: goreleaser already cross-compiles
# this on every release (.goreleaser.yml), and pulling in a Go toolchain
# just to rebuild what's already sitting on the release page has nothing
# to offer over downloading it. hush-hush (the server) has no AUR package
# of its own - it's container-only, see .goreleaser.yml's build config for
# why.
#
# sha256sums below are placeholders. Run `updpkgsums` (pacman-contrib)
# once a real tagged release with this packaging actually exists - there's
# nothing to check them against before then.
pkgname=hush-hush-cli-bin
pkgver=0.14.0
pkgrel=1
pkgdesc="Client for the hush-hush secrets object store"
arch=('x86_64' 'aarch64')
url="https://github.com/alrayyes/hush-hush"
license=('GPL-3.0-only')
provides=('hush-hush-cli')
conflicts=('hush-hush-cli')
source_x86_64=("$pkgname-$pkgver.tar.gz::$url/releases/download/v$pkgver/hush-hush-cli_${pkgver}_linux_amd64.tar.gz")
source_aarch64=("$pkgname-$pkgver.tar.gz::$url/releases/download/v$pkgver/hush-hush-cli_${pkgver}_linux_arm64.tar.gz")
sha256sums_x86_64=('0000000000000000000000000000000000000000000000000000000000000000')
sha256sums_aarch64=('0000000000000000000000000000000000000000000000000000000000000000')

package() {
  install -Dm755 hush-hush-cli "$pkgdir/usr/bin/hush-hush-cli"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"

  local page
  for page in man1/*.1; do
    install -Dm644 "$page" "$pkgdir/usr/share/man/$page"
  done
}
