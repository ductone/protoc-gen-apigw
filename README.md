# protoc-gen-apigw

`protoc-gen-apigw` is a plugin for the Google protocol buffer compiler to generate a Go server which implements gRPC service interfaces and a REST gateway.

It is inspired by [utrack/clay](https://github.com/utrack/clay) and [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway)), but uses a different approach to bridging HTTP Requests to gRPC calls.

Features include:
 - Can use existing gRPC service definitions and generate a REST gateway for them.
 - Can use existing gRPC Middlewares.
 - Router-shim layer: Can be easily used with other HTTP Routers or Muxers in the HTTP Ecosystem.  [Gin](https://github.com/gin-gonic/gin) based integration is standalone of the core project and about.

Many TODOs are remaining, but the basic bridging works.

## Enum query parameters

Enum query parameters accept the case-sensitive protobuf enum names advertised
in the generated OpenAPI schema, including aliases. Decimal numeric values are
also accepted for backward compatibility, within the protobuf enum's signed
32-bit range. Unknown numeric values are left to application validation.
An omitted or empty value defaults to zero. Invalid names, non-integers and
out-of-range numbers return `InvalidArgument` without reflecting the input.

Only singular, top-level query fields are supported; nested enum declarations
and enum types imported from other protobuf packages are supported. Repeated
query fields and nested query field paths remain unsupported. Route parameters,
body decoding and other scalar conversions are unchanged.

The compiled generator regression can be run with
`go test -mod=vendor ./internal/apigw -run '^TestEnumQueryGenerated$' -v`.
It requires `protoc`, `protoc-gen-go` and `protoc-gen-go-grpc` on `PATH`, as does
example generation, and explicitly skips when these tools are unavailable.
It builds the plugin, generates temporary Go/gRPC/gateway
and OpenAPI output, and executes the actual generated decoder against every
advertised enum name and numeric/error boundary cases.

## License

`protc-gen-apigw` is licensed under the Apache License 2.0.  See [LICENSE](LICENSE) for the full license text.

