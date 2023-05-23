# protoc-gen-apigw

`protoc-gen-apigw` is a plugin for the Google protocol buffer compiler to generate a Go server which implements gRPC service interfaces and a REST gateway.

It is inspired by [utrack/clay](https://github.com/utrack/clay) and [grpc-gateway)](https://github.com/grpc-ecosystem/grpc-gateway), but uses a different approach to bridging HTTP Requests to gRPC calls.

Features include:
 - Can use existing gRPC service definitions and generate a REST gateway for them.
 - Can use existing gRPC Middlewares.
 - Router-shim layer: Can be easily used with other HTTP Routers or Muxers in the HTTP Ecorystem.  [Gin](https://github.com/gin-gonic/gin) based integration is standlone of the core project and about.

Many TODOs are remaining, but the basic bridging works.

## License

`protc-gen-apigw` is licensed under the Apache License 2.0.  See [LICENSE](LICENSE) for the full license text.

