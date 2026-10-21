# protoc-gen-apigw

`protoc-gen-apigw` is a plugin for the Google protocol buffer compiler to generate a Go server which implements gRPC service interfaces and a REST gateway.

It is inspired by [utrack/clay](https://github.com/utrack/clay) and [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway)), but uses a different approach to bridging HTTP Requests to gRPC calls.

Features include:
 - Can use existing gRPC service definitions and generate a REST gateway for them.
 - Can use existing gRPC Middlewares.
 - Router-shim layer: Can be easily used with other HTTP Routers or Muxers in the HTTP Ecosystem.  [Gin](https://github.com/gin-gonic/gin) based integration is standalone of the core project and about.

Many TODOs are remaining, but the basic bridging works.

## Development checks

With Go 1.22 and the usual build tools installed:

```sh
make test
make lint       # Protobuf lint (requires buf).
make lint-go    # The same Go checks used by both CI lint jobs.
```

`lint-go` requires Bash, curl, tar (or unzip on macOS), and sha256sum or
shasum. It supports Linux/macOS on amd64/arm64. On a cold cache it downloads
[plaid-lint v1.0.0-rc.17](https://github.com/ConductorOne/plaid-lint/releases/tag/v1.0.0-rc.17)
and checks the platform archive against its pinned release SHA256.
Two standalone companions preserve checks not executed by that release:
goimports (`golang.org/x/tools` v0.31.0) and tenv v1.12.1, with
`go vet -tenv.all=true` covering test helpers too. The vet invocation also
keeps compiler/type-check failures fatal independently of Plaid. Formatting
differences, reported analyzer findings, and command failures fail the target.

Companions match the versions used by golangci-lint v1.64.8. They are built
with Go 1.24.1 because goimports requires Go 1.23+; Go's public module proxy
and checksum database verify the pinned modules. This does not change the
project's Go version or analysis toolchain.

### Lint cache

Tools, build-SDK modules and the tool build cache persist under
`${XDG_CACHE_HOME:-$HOME/.cache}/protoc-gen-apigw/lint`. Set
`APIGW_LINT_CACHE_HOME` to an absolute cache-home directory to override that
base (the project namespace is always appended). Keep the cache outside the
checkout. Only cold tool preparation needs network access to GitHub and the
Go module proxy/checksum database; valid warm tools are not downloaded,
installed or rebuilt.

Bundles are keyed by format, pinned tool/SDK versions and host platform.
Every reuse verifies the pinned archive, the Plaid binary against that archive,
companion binary hashes and their Go build metadata/module checksums. No lint
analyzer binary is selected from PATH. Treat writable caches as private trusted
data, not as a security boundary against someone able to replace binaries and receipts.

A portable per-bundle lock serializes preparation and checks, including
explicit cleaning. Complete verified installs are published by atomic rename;
partial/corrupt/wrong-version entries fail closed, never execute, and require
`make clean-lint-tools` before retrying. That target removes only the current
project/version/platform tool bundle, retaining SDK modules, build/analysis
caches, other versions and other projects. Only invocation-owned staging and
locks are automatically removed. Lock waits are bounded to 300 seconds
(`APIGW_LINT_LOCK_TIMEOUT=1..300`); a lock left by SIGKILL is never stolen and
should be removed manually only after confirming its owner is no longer active.

All three checks and fresh source enumeration run on every invocation; no
lint verdict is cached by the wrapper. Plaid's normal incremental analysis
cache is separated by Plaid version, physical workspace, analysis Go version
and host platform. CI uses an explicit cache-home override under `runner.temp`
and restores/saves only this project namespace with an exact platform/tool-identity
key, without broad fallback keys. The cache tests use
real verified bundle fixtures and replace installation transport only.

The lint configuration retains the previous lax generated-header policy and
SA/S staticcheck families. The additional ST/QF checks in this plaid release
are explicitly excluded, and TODO comments retain their punctuation exception.
Security checks remain enabled. Only the existing intentional debug-file read
and compiled-generator fixture messages retain narrow path/message exceptions
from the old implicit filters; no whole security rule or legacy preset is
disabled. The fixture's subprocess calls also retain the old HTTP-only noctx
scope; HTTP checks remain enabled there and exec checks remain enabled elsewhere.
Plaid uses newer analyzer libraries, not identical historical implementations.
Its scanner-error reporting limitations also existed in the old adapter; a
successful lint run is not a comprehensive security audit.

## License

`protc-gen-apigw` is licensed under the Apache License 2.0.  See [LICENSE](LICENSE) for the full license text.

