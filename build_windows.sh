#!/usr/bin/env bash

set -euo pipefail

EDGEGATE_TAGS="${EDGEGATE_TAGS:-with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api}"

export CC="${CC:-x86_64-w64-mingw32-gcc}"
export CXX="${CXX:-x86_64-w64-mingw32-g++}"
export CGO_ENABLED=1

mkdir -p bin
rm -f bin/edgegate-core.dll bin/edgegate-core.h bin/libcore.dll bin/libcore.h bin/EdgegateCli.exe edgegate-core.dll

GOOS= GOARCH= go run ./cmd/main tunnel exit

export GOOS=windows
export GOARCH=amd64

CGO_LDFLAGS= go build -trimpath -tags "${EDGEGATE_TAGS}" -ldflags="-w -s" -buildmode=c-shared -o bin/edgegate-core.dll ./platform/desktop
cp -f bin/edgegate-core.dll bin/libcore.dll
cp -f bin/edgegate-core.h bin/libcore.h

if ! go install -mod=readonly github.com/akavel/rsrc@latest; then
  echo "[warn] failed to install rsrc, skipping EdgegateCli.exe build"
  exit 0
fi

rsrc_dir="$(go env GOBIN)"
if [ -n "${rsrc_dir}" ]; then
  :
else
  gopath_value="$(go env GOPATH)"
  case "${gopath_value}" in
    [A-Za-z]:\\*)
      ;;
    *:*)
      gopath_value="${gopath_value%%:*}"
      ;;
  esac
  case "${gopath_value}" in
    *';'*)
      gopath_value="${gopath_value%%;*}"
      ;;
  esac
  rsrc_dir="${gopath_value}/bin"
fi

if command -v cygpath >/dev/null 2>&1; then
  rsrc_dir="$(cygpath -u "${rsrc_dir}" 2>/dev/null || printf '%s' "${rsrc_dir}")"
fi

export PATH="${rsrc_dir}:${PATH}"
rsrc_bin="$(command -v rsrc || true)"

if [ -z "${rsrc_bin}" ]; then
  echo "[warn] rsrc binary not found at ${rsrc_bin}, skipping EdgegateCli.exe build"
  exit 0
fi

"${rsrc_bin}" -ico ./assets/edgegate-cli.ico -o ./cmd/bydll/cli.syso

cleanup() {
  rm -f edgegate-core.dll
}
trap cleanup EXIT

cp -f bin/edgegate-core.dll ./edgegate-core.dll
CGO_LDFLAGS=edgegate-core.dll go build -trimpath -tags "${EDGEGATE_TAGS}" -ldflags="-s -w" -o bin/EdgegateCli.exe ./cmd/bydll/
