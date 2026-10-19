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
shasum. It supports Linux/macOS on amd64/arm64. It downloads
[plaid-lint v1.0.0-rc.17](https://github.com/ConductorOne/plaid-lint/releases/tag/v1.0.0-rc.17)
and checks the platform archive against its pinned release SHA256.
Two standalone companions preserve checks not executed by that release:
goimports (`golang.org/x/tools` v0.31.0) and tenv v1.12.1, with
`go vet -tenv.all=true` covering test helpers too. The vet invocation also
keeps compiler/type-check failures fatal independently of Plaid. Formatting
differences, reported analyzer findings, and command failures fail the target.

Companions match the versions used by golangci-lint v1.64.8. They are built
using an invocation-local Go 1.24.1 toolchain because goimports requires Go 1.23+;
Go's public module proxy and checksum database verify the pinned modules.
This does not change the project's Go version or analysis toolchain. Downloads
and binaries live in a temporary directory removed on exit; network access
to GitHub and the Go module proxy/checksum database is required.

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

