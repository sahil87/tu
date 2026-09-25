#!/usr/bin/env bash
set -euo pipefail

# Package the four tu-go release archives into dist/ (binary + vendored ccusage
# + tu.default.conf per platform, flat — no top-level directory), plus
# dist/tu-go-SHA256SUMS and a host smoke test.
# Called by: just go-package
# Requires: just go-build-all (dist/bin/tu-<os>-<arch> for each platform)

PLATFORMS=("darwin/arm64" "darwin/amd64" "linux/arm64" "linux/amd64")
ASSET_PREFIX="tu-go"
TARBALL_DIR="dist/ccusage-tarballs"
SUMS_FILE="dist/${ASSET_PREFIX}-SHA256SUMS"

# CCUSAGE_VERSION is the sole ccusage pin — read it first, before any download.
CCUSAGE_VERSION=$(tr -d '[:space:]' < CCUSAGE_VERSION)

echo "Packaging ${ASSET_PREFIX} archives (ccusage $CCUSAGE_VERSION)..."

for platform in "${PLATFORMS[@]}"; do
  os="${platform%%/*}"
  arch="${platform##*/}"

  bin="dist/bin/tu-${os}-${arch}"
  if [ ! -f "$bin" ]; then
    echo "ERROR: Missing $bin — run 'just go-build-all' first." >&2
    exit 1
  fi

  # npm arch spelling: x64 for amd64, arm64 unchanged; os unchanged. The
  # tarball carries the target platform's ccusage at package/bin/ccusage.
  npm_arch="$arch"
  if [ "$arch" = "amd64" ]; then
    npm_arch="x64"
  fi
  npm_pkg="ccusage-${os}-${npm_arch}"
  url="https://registry.npmjs.org/@ccusage/${npm_pkg}/-/${npm_pkg}-${CCUSAGE_VERSION}.tgz"
  tarball="${TARBALL_DIR}/${npm_pkg}-${CCUSAGE_VERSION}.tgz"

  mkdir -p "$TARBALL_DIR"
  if [ ! -f "$tarball" ]; then
    if ! curl -fsSL "$url" -o "$tarball"; then
      rm -f "$tarball"
      echo "error: failed to fetch $url" >&2
      exit 1
    fi
  fi

  archive="dist/${ASSET_PREFIX}-${os}-${arch}.tar.gz"
  staging="dist/staging-${ASSET_PREFIX}-${os}-${arch}"

  rm -rf "$staging"
  mkdir -p "$staging/vendor/ccusage/bin"
  cp "$bin" "$staging/tu"
  chmod 0755 "$staging/tu"
  tar xzf "$tarball" -C "$staging" package/bin/ccusage
  mv "$staging/package/bin/ccusage" "$staging/vendor/ccusage/bin/ccusage"
  chmod 0755 "$staging/vendor/ccusage/bin/ccusage"
  rm -rf "$staging/package"
  cp tu.default.conf "$staging/tu.default.conf"

  COPYFILE_DISABLE=1 tar czf "$archive" -C "$staging" tu vendor tu.default.conf
  echo "  ${ASSET_PREFIX}-${os}-${arch}.tar.gz ($(wc -c < "$archive") bytes)"
  rm -rf "$staging"
done

# sha256sum format (two spaces, bare filenames) — the asset dogfood-install
# verifies against. shasum exists on both macOS and Linux.
(cd dist && shasum -a 256 "${ASSET_PREFIX}"-darwin-arm64.tar.gz "${ASSET_PREFIX}"-darwin-amd64.tar.gz "${ASSET_PREFIX}"-linux-arm64.tar.gz "${ASSET_PREFIX}"-linux-amd64.tar.gz) > "$SUMS_FILE"
echo "  $(basename "$SUMS_FILE") written"

# Member check over all four archives: exactly tu, vendor/ccusage/bin/ccusage,
# tu.default.conf (directory entries allowed, no top-level directory).
expected_members=$(printf 'tu\ntu.default.conf\nvendor/ccusage/bin/ccusage')
for platform in "${PLATFORMS[@]}"; do
  archive="dist/${ASSET_PREFIX}-${platform%%/*}-${platform##*/}.tar.gz"
  members=$(tar tzf "$archive" | grep -v '/$' | sort)
  if [ "$members" != "$expected_members" ]; then
    echo "error: $archive members differ from the contract (tu, tu.default.conf, vendor/ccusage/bin/ccusage):" >&2
    echo "$members" >&2
    exit 1
  fi
done

# Host smoke test — the one pre-release execution of the tarball layout against
# ccusage.ResolveBinary (vendor beside the real binary). Cross-platform
# archives cannot run on this host and were checked structurally above.
case "$(uname -s)" in
  Linux) host_os=linux ;;
  Darwin) host_os=darwin ;;
  *) echo "error: unsupported host OS: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64) host_arch=amd64 ;;
  arm64 | aarch64) host_arch=arm64 ;;
  *) echo "error: unsupported host arch: $(uname -m)" >&2; exit 1 ;;
esac

host_archive="dist/${ASSET_PREFIX}-${host_os}-${host_arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

tar xzf "$host_archive" -C "$tmp"
# The same expression the justfile stamps (-X main.version={{go_version}}), so
# the archive's --version agrees by construction; at release the checkout is
# the tag, so this is exactly "tu version vX.Y.Z".
want="tu version $(git describe --tags --always)"
got=$("$tmp/tu" --version)
if [ "$got" != "$want" ]; then
  echo "error: host smoke test failed: $host_archive ./tu --version printed '$got', want '$want'" >&2
  exit 1
fi
if ! "$tmp/vendor/ccusage/bin/ccusage" --version > /dev/null; then
  echo "error: host smoke test failed: $host_archive vendored ccusage --version did not exit 0" >&2
  exit 1
fi
echo "Host smoke test passed (${host_os}/${host_arch}): $got; vendored ccusage ok"

echo "Packaging complete: ${#PLATFORMS[@]} archives"
