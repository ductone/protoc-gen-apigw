module github.com/ductone/protoc-gen-apigw/example

go 1.20

require (
	github.com/ductone/protoc-gen-apigw/apigw v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.55.0
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
)

replace github.com/ductone/protoc-gen-apigw/apigw => ../apigw
