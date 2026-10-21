#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
helper="$PWD/tools/lint-tools.sh"

# Worker processes use real verified bundle fixtures, never fake tool binaries.
# Only the installation transport is replaced, so these tests need no downloads.
if [[ ${1:-} == --worker ]]; then
  mode=$2
  source "$helper"
  trap lint_cache_cleanup EXIT
  lint_cache_begin
  lint_install_tools() {
    printf 'install\n' >> "$CACHE_TEST_LOG"
    if [[ $mode == interrupted ]]; then printf partial > "$1/partial"; return 23; fi
    if [[ $mode != install && $mode != slow ]]; then echo 'Unexpected installation' >&2; return 24; fi
    if [[ $mode == slow ]]; then
      printf partial > "$1/partial"
      touch "$CACHE_TEST_READY"
      start=$SECONDS
      until [[ -e $CACHE_TEST_RELEASE ]]; do
        (( SECONDS - start < 10 )) || return 25
        sleep 0.1
      done
    fi
    cp -R "$CACHE_TEST_FIXTURE/." "$1/"
  }
  if [[ $mode == wrong-platform ]]; then
    if [[ $lint_arch == arm64 ]]; then lint_arch=amd64; else lint_arch=arm64; fi
    lint_verify_companion "$CACHE_TEST_FIXTURE" goimports golang.org/x/tools/cmd/goimports golang.org/x/tools v0.31.0 \
      'h1:0EedkvKDbh+qistFTd0Bcwe/YLh4vHwWEkiI0toFIBU='
  else
    lint_prepare_tools
  fi
  exit 0
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/lint-cache-test.XXXXXX")
first=
second=
cleanup() {
  [[ -z $first ]] || kill "$first" 2>/dev/null || true
  [[ -z $second ]] || kill "$second" 2>/dev/null || true
  rm -rf -- "$work"
}
trap cleanup EXIT
# Require a prior real cold/warm run. Do not install tools as part of unit setup.
source "$helper"
trap 'lint_cache_cleanup; cleanup' EXIT
lint_cache_begin
lint_verify_tools "$lint_tools"
cp -R "$lint_tools" "$work/fixture"
key=$lint_key
lint_cache_cleanup
export CACHE_TEST_FIXTURE="$work/fixture" CACHE_TEST_LOG="$work/installs"
# Standard XDG and HOME fallbacks, without touching the caller's real home.
env -u APIGW_LINT_CACHE_HOME XDG_CACHE_HOME="$work/xdg" bash -c '
  set -euo pipefail; source "$1"; trap lint_cache_cleanup EXIT
  lint_cache_begin; [[ $lint_cache == "$XDG_CACHE_HOME/protoc-gen-apigw/lint" ]]
' bash "$helper"
env -u APIGW_LINT_CACHE_HOME -u XDG_CACHE_HOME HOME="$work/home" bash -c '
  set -euo pipefail; source "$1"; trap lint_cache_cleanup EXIT
  lint_cache_begin; [[ $lint_cache == "$HOME/.cache/protoc-gen-apigw/lint" ]]
' bash "$helper"
export APIGW_LINT_LOCK_TIMEOUT=3
: > "$CACHE_TEST_LOG"

seed() {
  home="$work/$1"
  entry="$home/protoc-gen-apigw/lint/tools/$key"
  mkdir -p "$(dirname "$entry")"
  cp -R "$CACHE_TEST_FIXTURE" "$entry"
}
worker() { APIGW_LINT_CACHE_HOME="$home" bash "$0" --worker "$1"; }
reject() {
  if worker "$1" > "$work/output" 2>&1; then
    echo "Unexpected acceptance: $1" >&2; cat "$work/output"; exit 1
  fi
}

seed warm
worker warm
[[ ! -s $CACHE_TEST_LOG ]]

seed wrong-version
printf 'obsolete-version\n' > "$entry/identity"
reject warm
seed partial
rm "$entry/identity"
reject warm
seed archive
printf corruption >> "$entry/plaid.$lint_extension"
reject warm
seed plaid
printf corruption >> "$entry/bin/plaid-lint"
reject warm
seed companion
printf corruption >> "$entry/bin/goimports"
reject warm
seed wrong-module
cp "$entry/bin/tenv" "$entry/bin/goimports"
lint_hash < "$entry/bin/goimports" > "$entry/goimports.sha256"
reject warm
seed symlink
rm "$entry/bin/goimports"
ln -s "$CACHE_TEST_FIXTURE/bin/goimports" "$entry/bin/goimports"
reject warm
seed platform
reject wrong-platform
[[ ! -s $CACHE_TEST_LOG ]]

# Different/obsolete keys must not become current merely because they exist.
home="$work/key-invalidation"
mkdir -p "$home/protoc-gen-apigw/lint/tools"
cp -R "$CACHE_TEST_FIXTURE" "$home/protoc-gen-apigw/lint/tools/obsolete-key"
worker install
[[ -d $home/protoc-gen-apigw/lint/tools/$key && -d $home/protoc-gen-apigw/lint/tools/obsolete-key ]]
[[ $(wc -l < "$CACHE_TEST_LOG") -eq 1 ]]

# A failed installation must publish nothing and clean only its own staging/lock.
home="$work/interrupted"
reject interrupted
[[ ! -e $home/protoc-gen-apigw/lint/tools/$key ]]
[[ -z $(ls -A "$home/protoc-gen-apigw/lint/tools") ]]

# A stale lock is bounded and is never stolen/removed by a non-owner.
home="$work/locked"
lock="$home/protoc-gen-apigw/lint/tools/$key.lock"
mkdir -p "$lock"
APIGW_LINT_LOCK_TIMEOUT=1 reject warm
[[ -d $lock ]]

# Lock/atomic-publication proof with two processes and a deliberately partial stage.
home="$work/concurrent"
export CACHE_TEST_READY="$work/ready" CACHE_TEST_RELEASE="$work/release"
: > "$CACHE_TEST_LOG"
worker slow > "$work/first.log" 2>&1 &
first=$!
start=$SECONDS
until [[ -e $CACHE_TEST_READY ]]; do
  (( SECONDS - start < 10 )) || { cat "$work/first.log"; exit 1; }
  sleep 0.1
done
[[ ! -e $home/protoc-gen-apigw/lint/tools/$key ]]
worker warm > "$work/second.log" 2>&1 &
second=$!
sleep 0.2
kill -0 "$second"
[[ ! -e $home/protoc-gen-apigw/lint/tools/$key ]]
touch "$CACHE_TEST_RELEASE"
wait "$first"
first=
wait "$second"
second=
[[ $(wc -l < "$CACHE_TEST_LOG") -eq 1 ]]
[[ ! -d $home/protoc-gen-apigw/lint/tools/$key.lock ]]

# Explicit cleaning removes only this project's current bundle, not other data.
mkdir -p "$home/protoc-gen-apigw/lint/go-modules" "$home/unrelated-project"
touch "$home/protoc-gen-apigw/lint/go-modules/keep" "$home/unrelated-project/keep"
APIGW_LINT_CACHE_HOME="$home" bash tools/lint-go.sh --clean-tools
[[ ! -e $home/protoc-gen-apigw/lint/tools/$key ]]
[[ -f $home/protoc-gen-apigw/lint/go-modules/keep && -f $home/unrelated-project/keep ]]
echo 'Lint cache identity, integrity, key, partial, concurrency, timeout and scoped-clean tests passed'
