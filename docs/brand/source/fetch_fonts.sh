#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
mkdir -p fonts
base=https://raw.githubusercontent.com/google/fonts/main/ofl
curl -fsSL -o fonts/Cinzel.ttf "$base/cinzel/Cinzel%5Bwght%5D.ttf"
curl -fsSL -o fonts/BodoniModa.ttf "$base/bodonimoda/BodoniModa%5Bopsz,wght%5D.ttf"
curl -fsSL -o fonts/PlayfairDisplay.ttf "$base/playfairdisplay/PlayfairDisplay%5Bwght%5D.ttf"
