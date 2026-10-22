#!/usr/bin/env bash
# Prepare host-side caches for Terminal-Bench runs (qh tbench):
#  - static uv/uvx 0.9.5 from PyPI (astral.sh, the official installer host, is
#    blocked by this environment's egress policy). The runner copies them into
#    /root/.local/bin of each task container before the official verifier runs.
set -euo pipefail
d=/tmp/qh-tbcache/uv
mkdir -p "$d" && cd "$d"
if [ ! -x uv ]; then
  pip download -q uv==0.9.5 --no-deps --only-binary=:all: --platform musllinux_1_1_x86_64 -d .
  python3 - <<'PY'
import glob, zipfile
z = zipfile.ZipFile(glob.glob("uv-*.whl")[0])
for n in z.namelist():
    if n.endswith("/uv") or n.endswith("/uvx"):
        open(n.split("/")[-1], "wb").write(z.read(n))
PY
  chmod +x uv uvx
fi
./uv --version
