// Code generated by protoc-gen-apigw 0.1.0 from bookstore/v1/bookstore.proto. DO NOT EDIT
package v1

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protopack"
)

const APIGW_BookstoreServiceServer_OpenAPI_3_1_Spec = `---
openapi: 3.1.0
info:
    title: API For .bookstore.v1.BookstoreService
    version: 0.0.1
paths:
    /shelves/{shelf}/books:
        post:
            operationId: .bookstore.v1.BookstoreService.CreateBook
            parameters:
                - name: shelf
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
            requestBody:
                content:
                    application/json:
                        schema:
                            $ref: '#/components/schemas/.bookstore.v1.CreateBookRequestInput'
            responses:
                default:
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/.bookstore.v1.CreateBookResponse'
    /shelves/{shelf}/books/{book}:
        get:
            operationId: .bookstore.v1.BookstoreService.GetBook
            parameters:
                - name: shelf
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
                - name: book
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
                - name: author
                  in: query
                  style: simple
                  schema:
                    type: boolean
            responses:
                default:
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/.bookstore.v1.GetBookResponse'
        delete:
            operationId: .bookstore.v1.BookstoreService.DeleteBook
            parameters:
                - name: shelf
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
                - name: book
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
            requestBody:
                content:
                    application/json:
                        schema:
                            $ref: '#/components/schemas/.bookstore.v1.DeleteBookRequestInput'
            responses:
                default:
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/.bookstore.v1.DeleteBookResponse'
    /shelves/{shelf}/books/{book.id}:
        patch:
            operationId: .bookstore.v1.BookstoreService.UpdateBook
            parameters:
                - name: shelf
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
                - name: book.id
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
            requestBody:
                content:
                    application/json:
                        schema:
                            $ref: '#/components/schemas/.bookstore.v1.UpdateBookRequestInput'
            responses:
                default:
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/.bookstore.v1.UpdateBookResponse'
    /authors/{author}:
        get:
            operationId: .bookstore.v1.BookstoreService.GetAuthor
            parameters:
                - name: author
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
            responses:
                default:
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/.bookstore.v1.GetAuthorResponse'
    /shelves:
        get:
            operationId: .bookstore.v1.BookstoreService.ListShelves
            parameters: []
            responses:
                default:
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/.bookstore.v1.ListShelvesResponse'
    /shelf:
        post:
            operationId: .bookstore.v1.BookstoreService.CreateShelf
            parameters: []
            requestBody:
                content:
                    application/json:
                        schema:
                            $ref: '#/components/schemas/.bookstore.v1.CreateShelfRequest'
            responses:
                default:
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/.bookstore.v1.CreateShelfResponse'
    /shelves/{shelf}:
        delete:
            operationId: .bookstore.v1.BookstoreService.DeleteShelf
            parameters:
                - name: shelf
                  in: path
                  required: true
                  style: simple
                  schema:
                    type: string
                    format: int64
            requestBody:
                content:
                    application/json:
                        schema:
                            $ref: '#/components/schemas/.bookstore.v1.DeleteShelfRequestInput'
            responses:
                default:
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/.bookstore.v1.DeleteShelfResponse'
components:
    schemas:
        .bookstore.v1.UpdateBookResponse:
            type: object
            properties:
                book:
                    $ref: '#/components/schemas/.bookstore.v1.Book'
        .bookstore.v1.UpdateBookRequestInput:
            type: object
            properties:
                book:
                    $ref: '#/components/schemas/.bookstore.v1.Book'
        .bookstore.v1.ListShelvesResponse:
            type: object
            properties:
                shelves:
                    type: array
                    items:
                        $ref: '#/components/schemas/.bookstore.v1.Shelf'
                    nullable: true
        .bookstore.v1.CreateShelfRequest:
            type: object
            properties:
                shelf:
                    $ref: '#/components/schemas/.bookstore.v1.Shelf'
        .bookstore.v1.CreateBookRequestInput:
            type: object
            properties:
                book:
                    $ref: '#/components/schemas/.bookstore.v1.Book'
        .bookstore.v1.GetBookResponse:
            type: object
            properties:
                book:
                    $ref: '#/components/schemas/.bookstore.v1.Book'
        .bookstore.v1.DeleteBookResponse:
            type: object
        .bookstore.v1.DeleteBookRequestInput:
            type: object
        .bookstore.v1.GetAuthorResponse:
            type: object
            properties:
                author:
                    $ref: '#/components/schemas/.bookstore.v1.Author'
        .bookstore.v1.GetAuthorRequestInput:
            type: object
        .bookstore.v1.ListShelvesRequest:
            type: object
        .bookstore.v1.DeleteShelfResponse:
            type: object
        .bookstore.v1.DeleteShelfRequestInput:
            type: object
        .bookstore.v1.Book:
            type: object
            properties:
                id:
                    type: string
                    format: int64
                author:
                    type: string
                title:
                    type: string
                quotes:
                    type: array
                    items:
                        type: string
                    nullable: true
        .bookstore.v1.GetBookRequestInput:
            type: object
        .bookstore.v1.Shelf:
            type: object
            properties:
                id:
                    type: string
                    format: int64
                theme:
                    type: string
                search[decoded]:
                    type: string
                search%5Bencoded%5D:
                    type: string
        .bookstore.v1.CreateShelfResponse:
            type: object
            properties:
                shelf:
                    $ref: '#/components/schemas/.bookstore.v1.Shelf'
        .bookstore.v1.CreateBookResponse:
            type: object
            properties:
                book:
                    $ref: '#/components/schemas/.bookstore.v1.Book'
        .bookstore.v1.Author:
            type: object
            properties:
                lname:
                    type: string
                id:
                    type: string
                    format: int64
                gender:
                    type: string
                    enum:
                        - GENDER_UNSPECIFIED
                        - GENDER_MALE
                        - GENDER_FEMALE
                firstName:
                    type: string

`

func RegisterGatewayBookstoreServiceServer(s apigw_v1.ServiceRegistrar, srv BookstoreServiceServer) {
	s.RegisterService(&apigw_desc_BookstoreServiceServer, srv)
}

var apigw_desc_BookstoreServiceServer = apigw_v1.ServiceDesc{
	Name:        "bookstore.v1.BookstoreService",
	HandlerType: (*BookstoreServiceServer)(nil),
	Methods: []*apigw_v1.MethodDesc{
		{
			Name:    BookstoreService_ListShelves_FullMethodName,
			Method:  http.MethodGet,
			Route:   "/shelves",
			Handler: _BookstoreService_ListShelves_APIGW_Handler,
			Decoder: _BookstoreService_ListShelves_APIGW_Decoder,
		},
		{
			Name:    BookstoreService_CreateShelf_FullMethodName,
			Method:  http.MethodPost,
			Route:   "/shelf",
			Handler: _BookstoreService_CreateShelf_APIGW_Handler,
			Decoder: _BookstoreService_CreateShelf_APIGW_Decoder,
		},
		{
			Name:    BookstoreService_DeleteShelf_FullMethodName,
			Method:  http.MethodDelete,
			Route:   "/shelves/{shelf}",
			Handler: _BookstoreService_DeleteShelf_APIGW_Handler,
			Decoder: _BookstoreService_DeleteShelf_APIGW_Decoder,
		},
		{
			Name:    BookstoreService_CreateBook_FullMethodName,
			Method:  http.MethodPost,
			Route:   "/shelves/{shelf}/books",
			Handler: _BookstoreService_CreateBook_APIGW_Handler,
			Decoder: _BookstoreService_CreateBook_APIGW_Decoder,
		},
		{
			Name:    BookstoreService_GetBook_FullMethodName,
			Method:  http.MethodGet,
			Route:   "/shelves/{shelf}/books/{book}",
			Handler: _BookstoreService_GetBook_APIGW_Handler,
			Decoder: _BookstoreService_GetBook_APIGW_Decoder,
		},
		{
			Name:    BookstoreService_DeleteBook_FullMethodName,
			Method:  http.MethodDelete,
			Route:   "/shelves/{shelf}/books/{book}",
			Handler: _BookstoreService_DeleteBook_APIGW_Handler,
			Decoder: _BookstoreService_DeleteBook_APIGW_Decoder,
		},
		{
			Name:    BookstoreService_UpdateBook_FullMethodName,
			Method:  http.MethodPatch,
			Route:   "/shelves/{shelf}/books/{book.id}",
			Handler: _BookstoreService_UpdateBook_APIGW_Handler,
			Decoder: _BookstoreService_UpdateBook_APIGW_Decoder,
		},
		{
			Name:    BookstoreService_GetAuthor_FullMethodName,
			Method:  http.MethodGet,
			Route:   "/authors/{author}",
			Handler: _BookstoreService_GetAuthor_APIGW_Handler,
			Decoder: _BookstoreService_GetAuthor_APIGW_Decoder,
		},
	},
}

func _BookstoreService_ListShelves_APIGW_Handler(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error) {
	in := new(ListShelvesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BookstoreServiceServer).ListShelves(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BookstoreService_ListShelves_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BookstoreServiceServer).ListShelves(ctx, req.(*ListShelvesRequest))
	}

	rv, err := interceptor(ctx, in, info, handler)
	if err != nil {
		return nil, err
	}
	return rv.(proto.Message), nil
}

func _BookstoreService_ListShelves_APIGW_Decoder(ctx context.Context, input apigw_v1.DecoderInput, out proto.Message) error {
	var err error
	_ = err

	return nil
}

func _BookstoreService_CreateShelf_APIGW_Handler(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error) {
	in := new(CreateShelfRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BookstoreServiceServer).CreateShelf(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BookstoreService_CreateShelf_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BookstoreServiceServer).CreateShelf(ctx, req.(*CreateShelfRequest))
	}

	rv, err := interceptor(ctx, in, info, handler)
	if err != nil {
		return nil, err
	}
	return rv.(proto.Message), nil
}

func _BookstoreService_CreateShelf_APIGW_Decoder(ctx context.Context, input apigw_v1.DecoderInput, out proto.Message) error {
	var err error
	_ = err

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	return nil
}

func _BookstoreService_DeleteShelf_APIGW_Handler(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error) {
	in := new(DeleteShelfRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BookstoreServiceServer).DeleteShelf(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BookstoreService_DeleteShelf_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BookstoreServiceServer).DeleteShelf(ctx, req.(*DeleteShelfRequest))
	}

	rv, err := interceptor(ctx, in, info, handler)
	if err != nil {
		return nil, err
	}
	return rv.(proto.Message), nil
}

func _BookstoreService_DeleteShelf_APIGW_Decoder(ctx context.Context, input apigw_v1.DecoderInput, out proto.Message) error {
	var err error
	_ = err

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	vn0 := input.PathParam("shelf")

	vn1tmp, err := strconv.ParseInt(vn0, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "shelf is not a valid int: %s", err)
	}

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.VarintType},
		protopack.Varint(vn1tmp),
	}

	err = proto.Unmarshal(vn1.Marshal(), out)
	if err != nil {
		return err
	}

	return nil
}

func _BookstoreService_CreateBook_APIGW_Handler(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error) {
	in := new(CreateBookRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BookstoreServiceServer).CreateBook(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BookstoreService_CreateBook_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BookstoreServiceServer).CreateBook(ctx, req.(*CreateBookRequest))
	}

	rv, err := interceptor(ctx, in, info, handler)
	if err != nil {
		return nil, err
	}
	return rv.(proto.Message), nil
}

func _BookstoreService_CreateBook_APIGW_Decoder(ctx context.Context, input apigw_v1.DecoderInput, out proto.Message) error {
	var err error
	_ = err

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	vn0 := input.PathParam("shelf")

	vn1tmp, err := strconv.ParseInt(vn0, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "shelf is not a valid int: %s", err)
	}

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.VarintType},
		protopack.Varint(vn1tmp),
	}

	err = proto.Unmarshal(vn1.Marshal(), out)
	if err != nil {
		return err
	}

	return nil
}

func _BookstoreService_GetBook_APIGW_Handler(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error) {
	in := new(GetBookRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BookstoreServiceServer).GetBook(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BookstoreService_GetBook_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BookstoreServiceServer).GetBook(ctx, req.(*GetBookRequest))
	}

	rv, err := interceptor(ctx, in, info, handler)
	if err != nil {
		return nil, err
	}
	return rv.(proto.Message), nil
}

func _BookstoreService_GetBook_APIGW_Decoder(ctx context.Context, input apigw_v1.DecoderInput, out proto.Message) error {
	var err error
	_ = err

	vn4 := input.Query().Get("author")

	vn5tmp, err := strconv.ParseBool(strings.ToLower(vn4))
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "includeAuthor is not a valid bool: %s", err)
	}

	vn5 := protopack.Message{
		protopack.Tag{Number: 3, Type: protopack.VarintType},
		protopack.Bool(vn5tmp),
	}

	err = proto.Unmarshal(vn5.Marshal(), out)
	if err != nil {
		return err
	}

	vn0 := input.PathParam("shelf")

	vn1tmp, err := strconv.ParseInt(vn0, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "shelf is not a valid int: %s", err)
	}

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.VarintType},
		protopack.Varint(vn1tmp),
	}

	err = proto.Unmarshal(vn1.Marshal(), out)
	if err != nil {
		return err
	}

	vn2 := input.PathParam("book")

	vn3tmp, err := strconv.ParseInt(vn2, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "book is not a valid int: %s", err)
	}

	vn3 := protopack.Message{
		protopack.Tag{Number: 2, Type: protopack.VarintType},
		protopack.Varint(vn3tmp),
	}

	err = proto.Unmarshal(vn3.Marshal(), out)
	if err != nil {
		return err
	}

	return nil
}

func _BookstoreService_DeleteBook_APIGW_Handler(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error) {
	in := new(DeleteBookRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BookstoreServiceServer).DeleteBook(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BookstoreService_DeleteBook_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BookstoreServiceServer).DeleteBook(ctx, req.(*DeleteBookRequest))
	}

	rv, err := interceptor(ctx, in, info, handler)
	if err != nil {
		return nil, err
	}
	return rv.(proto.Message), nil
}

func _BookstoreService_DeleteBook_APIGW_Decoder(ctx context.Context, input apigw_v1.DecoderInput, out proto.Message) error {
	var err error
	_ = err

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	vn0 := input.PathParam("shelf")

	vn1tmp, err := strconv.ParseInt(vn0, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "shelf is not a valid int: %s", err)
	}

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.VarintType},
		protopack.Varint(vn1tmp),
	}

	err = proto.Unmarshal(vn1.Marshal(), out)
	if err != nil {
		return err
	}

	vn2 := input.PathParam("book")

	vn3tmp, err := strconv.ParseInt(vn2, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "book is not a valid int: %s", err)
	}

	vn3 := protopack.Message{
		protopack.Tag{Number: 2, Type: protopack.VarintType},
		protopack.Varint(vn3tmp),
	}

	err = proto.Unmarshal(vn3.Marshal(), out)
	if err != nil {
		return err
	}

	return nil
}

func _BookstoreService_UpdateBook_APIGW_Handler(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error) {
	in := new(UpdateBookRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BookstoreServiceServer).UpdateBook(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BookstoreService_UpdateBook_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BookstoreServiceServer).UpdateBook(ctx, req.(*UpdateBookRequest))
	}

	rv, err := interceptor(ctx, in, info, handler)
	if err != nil {
		return nil, err
	}
	return rv.(proto.Message), nil
}

func _BookstoreService_UpdateBook_APIGW_Decoder(ctx context.Context, input apigw_v1.DecoderInput, out proto.Message) error {
	var err error
	_ = err

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	vn0 := input.PathParam("shelf")

	vn1tmp, err := strconv.ParseInt(vn0, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "shelf is not a valid int: %s", err)
	}

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.VarintType},
		protopack.Varint(vn1tmp),
	}

	err = proto.Unmarshal(vn1.Marshal(), out)
	if err != nil {
		return err
	}

	return nil
}

func _BookstoreService_GetAuthor_APIGW_Handler(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error) {
	in := new(GetAuthorRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BookstoreServiceServer).GetAuthor(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BookstoreService_GetAuthor_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BookstoreServiceServer).GetAuthor(ctx, req.(*GetAuthorRequest))
	}

	rv, err := interceptor(ctx, in, info, handler)
	if err != nil {
		return nil, err
	}
	return rv.(proto.Message), nil
}

func _BookstoreService_GetAuthor_APIGW_Decoder(ctx context.Context, input apigw_v1.DecoderInput, out proto.Message) error {
	var err error
	_ = err

	vn0 := input.PathParam("author")

	vn1tmp, err := strconv.ParseInt(vn0, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "author is not a valid int: %s", err)
	}

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.VarintType},
		protopack.Varint(vn1tmp),
	}

	err = proto.Unmarshal(vn1.Marshal(), out)
	if err != nil {
		return err
	}

	return nil
}
