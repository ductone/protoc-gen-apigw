FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/protoc-gen-apigw"]
COPY protoc-gen-apigw /