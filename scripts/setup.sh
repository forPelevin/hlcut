#!/usr/bin/env bash
set -euo pipefail

CACHE_DIR=".cache"
BIN_DIR="$CACHE_DIR/bin"
MODEL_DIR="$CACHE_DIR/models"

mkdir -p "$BIN_DIR" "$MODEL_DIR"

WHISPER_REPO_DIR="$CACHE_DIR/whisper.cpp"
WHISPER_BIN="$BIN_DIR/whisper.cpp"

if [ ! -d "$WHISPER_REPO_DIR" ]; then
  echo "[setup] cloning whisper.cpp..."
  git clone --depth=1 https://github.com/ggerganov/whisper.cpp.git "$WHISPER_REPO_DIR"
fi

echo "[setup] building whisper.cpp..."
pushd "$WHISPER_REPO_DIR" >/dev/null
mkdir -p build
cmake -S . -B build -DGGML_OPENMP=ON
cmake --build build -j
# binary name can vary; prefer 'whisper-cli'
if [ -f build/bin/whisper-cli ]; then
  cp -f build/bin/whisper-cli "/work/$WHISPER_BIN"
elif [ -f build/main ]; then
  cp -f build/main "/work/$WHISPER_BIN"
else
  echo "Could not find whisper binary in build output" >&2
  find build -maxdepth 3 -type f -name 'whisper*' -print >&2 || true
  exit 1
fi
popd >/dev/null

MODEL_PATH="$MODEL_DIR/ggml-base.bin"
if [ ! -f "$MODEL_PATH" ]; then
  echo "[setup] downloading whisper base model..."
  curl -L --fail \
    -o "$MODEL_PATH" \
    https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin
fi

echo "[setup] done"
echo "  whisper: $WHISPER_BIN"
echo "  model  : $MODEL_PATH"
