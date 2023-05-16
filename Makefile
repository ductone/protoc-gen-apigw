MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables

.PHONY: build
build:
	mkdir -p build
	go build -mod=readonly -v -o build/ .

.PHONY: generate
generate:
	buf generate protos

.PHONY: test
test:
	go test -v ./... ./example/...

.PHONY: example
example: build
	buf --debug generate --template buf.example.gen.yaml --path example/bookstore

.PHONY: fmt
fmt:
	buf format -w 

.PHONY: lint
lint:
	buf lint ./protos

.PHONY: adddep
adddep:
	go mod tidy -v
	go mod vendor

.PHONY: updatedeps
updatedeps:
	go get -d -u ./...
	go mod tidy -v
	go mod vendor

