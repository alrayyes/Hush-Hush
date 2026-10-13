#!/usr/bin/env bash
# Renders the installable-app icons from their SVG sources. Run it after
# editing static/icons/*.svg and commit the PNGs: the build doesn't render
# them, so a fresh clone needs no image tooling. Needs rsvg-convert.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../static/icons"

rsvg-convert -w 192 -h 192 icon.svg -o icon-192.png
rsvg-convert -w 512 -h 512 icon.svg -o icon-512.png
rsvg-convert -w 512 -h 512 icon-maskable.svg -o icon-maskable-512.png
rsvg-convert -w 180 -h 180 icon-maskable.svg -o apple-touch-icon.png
