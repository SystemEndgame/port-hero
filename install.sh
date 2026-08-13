#!/usr/bin/env sh
# ⚓ PORT HERO — auditable install script
#   curl -sL https://raw.githubusercontent.com/SystemEndgame/port-hero/main/install.sh | sh
#
# Downloads the prebuilt binary for your OS/arch into ~/.local/bin.
set -eu

REPO="SystemEndgame/port-hero"
VERSION="${PORT_HERO_VERSION:-latest}"
DEST="${PORT_HERO_INSTALL_DIR:-$HOME/.local/bin}"

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux)  echo "linux" ;;
    *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    arm64|aarch64) echo "arm64" ;;
    x86_64|amd64)  echo "amd64" ;;
    *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
  esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | \
    grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/.*"\(.*\)"/\1/')"
fi

URL="https://github.com/$REPO/releases/download/$VERSION/port-hero-${OS}-${ARCH}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "⚓ Installing Port Hero $VERSION ($OS/$ARCH)…"

curl -fSL --progress-bar "$URL" -o "$TMP/port-hero"
chmod +x "$TMP/port-hero"

mkdir -p "$DEST"
mv "$TMP/port-hero" "$DEST/port-hero"

# Create a `port` alias (the docs use `port`), unless a conflicting
# binary already exists (e.g. MacPorts).
if ! command -v port >/dev/null 2>&1 || [ "$(command -v port)" = "$DEST/port-hero" ]; then
  ln -sf "$DEST/port-hero" "$DEST/port"
fi

echo
echo "✔ Installed to $DEST/port-hero"
echo "   available as: port-hero  and  port"
echo
echo "Run:  port 3000     (or: port-hero 3000)"
echo "Path: add '$DEST' to your PATH if not already there."
echo "      (zsh: echo 'export PATH="\$HOME/.local/bin:\$PATH"' >> ~/.zshrc)"
echo "Built with ❤ by GoLive — golive.ly"
