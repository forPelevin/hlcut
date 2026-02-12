#!/usr/bin/env bash
set -euo pipefail

CACHE_DIR=".cache"
BIN_DIR="$CACHE_DIR/bin"
MODEL_DIR="$CACHE_DIR/models"
ROOT_DIR="$(pwd -P)"
WHISPER_REF="${WHISPER_REF:-764482c3175d9c3bc6089c1ec84df7d1b9537d83}"
MODEL_URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin"
MODEL_SHA256="${MODEL_SHA256:-60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe}"
SETUP_STATE_FILE="$CACHE_DIR/setup.fingerprint"
MODEL_PATH="$MODEL_DIR/ggml-base.bin"

mkdir -p "$BIN_DIR" "$MODEL_DIR"

WHISPER_REPO_DIR="$CACHE_DIR/whisper.cpp"
WHISPER_BIN="$BIN_DIR/whisper.cpp"

sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  echo "No SHA256 tool found (need sha256sum or shasum)" >&2
  return 1
}

sha256_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
    return
  fi
  echo "No SHA256 tool found (need sha256sum or shasum)" >&2
  return 1
}

verify_sha256() {
  local file="$1"
  local expected="$2"
  local actual
  actual="$(sha256_file "$file")"
  if [ "$actual" != "$expected" ]; then
    echo "SHA256 mismatch for $file" >&2
    echo "  expected: $expected" >&2
    echo "  actual  : $actual" >&2
    return 1
  fi
}

compiler_version_line() {
  if command -v cc >/dev/null 2>&1; then
    cc --version | head -n1
    return
  fi
  if command -v c++ >/dev/null 2>&1; then
    c++ --version | head -n1
    return
  fi
  echo "unknown"
}

cmake_version_line() {
  if command -v cmake >/dev/null 2>&1; then
    cmake --version | head -n1
    return
  fi
  echo "unknown"
}

compute_setup_fingerprint() {
  local setup_script_sha
  setup_script_sha="$(sha256_file "$ROOT_DIR/scripts/setup.sh")"
  {
    printf 'setup_script_sha=%s\n' "$setup_script_sha"
    printf 'whisper_ref=%s\n' "$WHISPER_REF"
    printf 'model_url=%s\n' "$MODEL_URL"
    printf 'model_sha256=%s\n' "$MODEL_SHA256"
    printf 'os=%s\n' "$(uname -s)"
    printf 'arch=%s\n' "$(uname -m)"
    printf 'cmake=%s\n' "$(cmake_version_line)"
    printf 'compiler=%s\n' "$(compiler_version_line)"
  } | sha256_stdin
}

should_skip_setup() {
  local stored_fingerprint current_head
  [ -f "$SETUP_STATE_FILE" ] || return 1
  [ -x "$WHISPER_BIN" ] || return 1
  [ -f "$MODEL_PATH" ] || return 1
  [ -d "$WHISPER_REPO_DIR/.git" ] || return 1
  stored_fingerprint="$(head -n1 "$SETUP_STATE_FILE" || true)"
  [ -n "$stored_fingerprint" ] || return 1
  [ "$stored_fingerprint" = "$SETUP_FINGERPRINT" ] || return 1
  current_head="$(git -C "$WHISPER_REPO_DIR" rev-parse HEAD 2>/dev/null || true)"
  [ "$current_head" = "$WHISPER_REF" ] || return 1
  verify_sha256 "$MODEL_PATH" "$MODEL_SHA256" >/dev/null 2>&1 || return 1
  return 0
}

SETUP_FINGERPRINT="$(compute_setup_fingerprint)"
if should_skip_setup; then
  echo "[setup] unchanged cache/env fingerprint; skipping"
  echo "  whisper: $WHISPER_BIN"
  echo "  model  : $MODEL_PATH"
  exit 0
fi

if [ ! -d "$WHISPER_REPO_DIR/.git" ]; then
  echo "[setup] cloning whisper.cpp..."
  git clone --filter=blob:none https://github.com/ggerganov/whisper.cpp.git "$WHISPER_REPO_DIR"
fi

echo "[setup] syncing whisper.cpp to pinned ref: $WHISPER_REF"
pushd "$WHISPER_REPO_DIR" >/dev/null
git fetch --depth=1 origin "$WHISPER_REF"
git -c advice.detachedHead=false checkout --force FETCH_HEAD
echo "[setup] building whisper.cpp..."
BUILD_DIR="$(pwd -P)/build"

if [ -f build/CMakeCache.txt ]; then
  CACHED_SRC_DIR="$(sed -n 's/^CMAKE_HOME_DIRECTORY:INTERNAL=//p' build/CMakeCache.txt | head -n1)"
  CACHED_BUILD_DIR="$(sed -n 's/^CMAKE_CACHEFILE_DIR:INTERNAL=//p' build/CMakeCache.txt | head -n1)"

  if [ "$CACHED_SRC_DIR" != "$(pwd -P)" ] || [ "$CACHED_BUILD_DIR" != "$BUILD_DIR" ]; then
    echo "[setup] detected stale CMake cache path, cleaning build dir..."
    rm -rf build
  fi
fi

mkdir -p build
cmake -S . -B build -DGGML_OPENMP=ON
cmake --build build -j
# binary name can vary; prefer 'whisper-cli'
if [ -f build/bin/whisper-cli ]; then
  cp -f build/bin/whisper-cli "$ROOT_DIR/$WHISPER_BIN"
elif [ -f build/main ]; then
  cp -f build/main "$ROOT_DIR/$WHISPER_BIN"
else
  echo "Could not find whisper binary in build output" >&2
  find build -maxdepth 3 -type f -name 'whisper*' -print >&2 || true
  exit 1
fi
popd >/dev/null

if [ ! -f "$MODEL_PATH" ]; then
  echo "[setup] downloading whisper base model..."
  TMP_MODEL_PATH="${MODEL_PATH}.tmp"
  curl -L --fail \
    -o "$TMP_MODEL_PATH" \
    "$MODEL_URL"
  verify_sha256 "$TMP_MODEL_PATH" "$MODEL_SHA256"
  mv -f "$TMP_MODEL_PATH" "$MODEL_PATH"
fi
verify_sha256 "$MODEL_PATH" "$MODEL_SHA256"
printf '%s\n' "$SETUP_FINGERPRINT" > "$SETUP_STATE_FILE"

echo "[setup] done"
echo "  whisper: $WHISPER_BIN"
echo "  model  : $MODEL_PATH"
