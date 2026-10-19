#!/usr/bin/env bash
# Internal helpers for lint-go.sh; this file never caches a lint verdict.

lint_hash() {
  if command -v sha256sum >/dev/null; then sha256sum; else shasum -a 256; fi | awk '{print $1}'
}

lint_cache_cleanup() {
  local status=0
  if [[ -n ${lint_stage:-} ]]; then rm -rf -- "$lint_stage" || status=$?; lint_stage=; fi
  if [[ -n ${lint_work:-} ]]; then rm -rf -- "$lint_work" || status=$?; lint_work=; fi
  if [[ ${lint_lock_owned:-0} == 1 ]]; then
    rmdir "$lint_lock" || status=$?
    lint_lock_owned=0
  fi
  return "$status"
}

lint_cache_begin() {
  umask 077
  lint_lock_owned=0
  lint_stage=
  lint_work=
  local home=${APIGW_LINT_CACHE_HOME:-${XDG_CACHE_HOME:-$HOME/.cache}}
  [[ $home == /* ]] || { echo 'Lint cache home must be absolute' >&2; return 1; }
  lint_cache="$home/protoc-gen-apigw/lint"
  lint_os=$(go env GOHOSTOS) || return 1
  lint_arch=$(go env GOHOSTARCH) || return 1
  lint_platform="$lint_os-$lint_arch"
  case "$lint_platform" in
    linux-amd64) lint_extension=tar.gz; lint_digest=1a9365eef0d89fca6474a18566c0d263aa0f05407e26adc4b2f92ac880dee226 ;;
    linux-arm64) lint_extension=tar.gz; lint_digest=bfcb96d7b2d1f3dee171c55faafcd05f87ea4141f191356f50ba6a536b16758f ;;
    darwin-amd64) lint_extension=zip; lint_digest=73743b952d6db82768265538e4c614b011dedde3c93e605af149d7316ea32f7f ;;
    darwin-arm64) lint_extension=zip; lint_digest=0e1a894ac0924adf3ae71ff3bd00c0c8ad2aad05a8f2b820bcd7301fd1e07287 ;;
    *) echo 'Go lint supports Linux/macOS amd64/arm64' >&2; return 1 ;;
  esac
  lint_key="v1-plaid-v1.0.0-rc.17-imports-v0.31.0-tenv-v1.12.1-go1.24.1-$lint_platform"
  lint_tools="$lint_cache/tools/$lint_key"
  lint_lock="$lint_tools.lock"
  mkdir -p "$lint_cache/tools" "$lint_cache/tmp" || return 1
  local timeout=${APIGW_LINT_LOCK_TIMEOUT:-300} start=$SECONDS
  [[ $timeout =~ ^[1-9][0-9]{0,2}$ ]] && (( timeout <= 300 )) || {
    echo 'APIGW_LINT_LOCK_TIMEOUT must be 1..300 seconds' >&2; return 1;
  }
  # Never steal a lock, including one left by SIGKILL. Only its owner removes it.
  until mkdir "$lint_lock" 2>/dev/null; do
    if [[ ! -d $lint_lock ]] || (( SECONDS - start >= timeout )); then
      echo "Cannot acquire lint cache lock: $lint_lock" >&2
      return 1
    fi
    sleep 1
  done
  lint_lock_owned=1
  lint_work=$(mktemp -d "$lint_cache/tmp/run.XXXXXX") || return 1
}

lint_verify_companion() {
  local directory=$1 tool=$2 package=$3 module=$4 version=$5 sum=$6 info
  [[ -f $directory/bin/$tool && ! -L $directory/bin/$tool && -x $directory/bin/$tool ]] || return 1
  [[ -f $directory/$tool.sha256 && ! -L $directory/$tool.sha256 ]] || return 1
  local expected actual
  expected=$(cat "$directory/$tool.sha256") || return 1
  [[ $expected =~ ^[a-f0-9]{64}$ ]] || return 1
  actual=$(lint_hash < "$directory/bin/$tool") || return 1
  [[ $actual == "$expected" ]] || return 1
  info=$(go version -m "$directory/bin/$tool") || return 1
  [[ $info == "$directory/bin/$tool: go1.24.1"$'\n'* ]] || return 1
  grep -Fxq $'\tpath\t'"$package" <<< "$info" || return 1
  grep -Fxq $'\tmod\t'"$module"$'\t'"$version"$'\t'"$sum" <<< "$info" || return 1
  grep -Fxq $'\tbuild\tGOOS='"$lint_os" <<< "$info" || return 1
  grep -Fxq $'\tbuild\tGOARCH='"$lint_arch" <<< "$info" || return 1
}

lint_verify_tools() {
  local directory=$1 archive actual archived
  [[ -d $directory && ! -L $directory && -f $directory/identity && ! -L $directory/identity ]] || return 1
  [[ -d $directory/bin && ! -L $directory/bin ]] || return 1
  [[ $(cat "$directory/identity") == "$lint_key" ]] || return 1
  archive="$directory/plaid.$lint_extension"
  [[ -f $archive && ! -L $archive && -x $directory/bin/plaid-lint && ! -L $directory/bin/plaid-lint ]] || return 1
  actual=$(lint_hash < "$archive") || return 1
  [[ $actual == "$lint_digest" ]] || return 1
  if [[ $lint_extension == zip ]]; then
    archived=$(unzip -p "$archive" plaid-lint | lint_hash) || return 1
  else
    archived=$(tar -xOf "$archive" plaid-lint | lint_hash) || return 1
  fi
  actual=$(lint_hash < "$directory/bin/plaid-lint") || return 1
  [[ $actual == "$archived" ]] || return 1
  lint_verify_companion "$directory" goimports golang.org/x/tools/cmd/goimports golang.org/x/tools v0.31.0 \
    'h1:0EedkvKDbh+qistFTd0Bcwe/YLh4vHwWEkiI0toFIBU=' || return 1
  lint_verify_companion "$directory" tenv github.com/sivchari/tenv/cmd/tenv github.com/sivchari/tenv v1.12.1 \
    'h1:+E0QzjktdnExv/wwsnnyk4oqZBUfuh89YMQT1cyuvSY=' || return 1
}

lint_install_tools() {
  local directory=$1 archive="$1/plaid.$lint_extension" actual tool
  mkdir "$directory/bin" || return 1
  curl --fail --location --output "$archive" \
    "https://github.com/ConductorOne/plaid-lint/releases/download/v1.0.0-rc.17/plaid-lint-v1.0.0-rc.17-$lint_platform.$lint_extension" || return 1
  actual=$(lint_hash < "$archive") || return 1
  [[ $actual == "$lint_digest" ]] || { echo 'Plaid archive checksum mismatch' >&2; return 1; }
  if [[ $lint_extension == zip ]]; then
    unzip -q "$archive" -d "$directory/bin" || return 1
  else
    tar -xzf "$archive" -C "$directory/bin" || return 1
  fi
  for tool in golang.org/x/tools/cmd/goimports@v0.31.0 github.com/sivchari/tenv/cmd/tenv@v1.12.1; do
    env GOTOOLCHAIN=go1.24.1 GOMODCACHE="$lint_cache/go-modules" GOCACHE="$lint_cache/go-build" \
      GOBIN="$directory/bin" GOOS="$lint_os" GOARCH="$lint_arch" CGO_ENABLED=0 GOFLAGS= GOEXPERIMENT= \
      GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org GONOSUMDB= GONOPROXY= GOPRIVATE= \
      go install "$tool" || return 1
  done
  for tool in goimports tenv; do
    lint_hash < "$directory/bin/$tool" > "$directory/$tool.sha256" || return 1
  done
  printf '%s\n' "$lint_key" > "$directory/identity" || return 1
}

lint_prepare_tools() {
  if [[ -e $lint_tools || -L $lint_tools ]]; then
    if ! lint_verify_tools "$lint_tools"; then
      echo "Invalid lint tool cache: $lint_tools; run make clean-lint-tools and retry" >&2
      return 1
    fi
    echo "Reusing verified lint tools: $lint_key" >&2
  else
    echo "Installing pinned lint tools: $lint_key" >&2
    lint_stage=$(mktemp -d "$lint_cache/tools/.install.XXXXXX") || return 1
    lint_install_tools "$lint_stage" || return 1
    lint_verify_tools "$lint_stage" || return 1
    # Publish the complete verified bundle atomically on the same filesystem.
    mv "$lint_stage" "$lint_tools" || return 1
    lint_stage=
  fi
}
