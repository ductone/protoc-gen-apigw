// Code generated by protoc-gen-apigw 0.1.22 from bookstore/v1/bookstore.proto. DO NOT EDIT
package v1

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	_ "embed"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protopack"
)

func RegisterGatewayBookstoreServiceServer(s apigw_v1.ServiceRegistrar, srv BookstoreServiceServer) {
	s.RegisterService(&apigw_desc_BookstoreServiceServer, srv)
}

//go:embed bookstore.pb.bookstore_service.oas31.yaml
var APIGW_OAS31_BookstoreServiceServer string

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
			Route:   "/shelves/{book.shelf_id}/books/{book.id}",
			Handler: _BookstoreService_DeleteBook_APIGW_Handler,
			Decoder: _BookstoreService_DeleteBook_APIGW_Decoder,
		},
		{
			Name:    BookstoreService_UpdateBook_FullMethodName,
			Method:  http.MethodPatch,
			Route:   "/shelves/{book.shelf_id}/books/{book.id}",
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

	unmarshalOpts := proto.UnmarshalOptions{AllowPartial: true, Merge: true, RecursionLimit: protowire.DefaultRecursionLimit}
	_ = unmarshalOpts

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

	unmarshalOpts := proto.UnmarshalOptions{AllowPartial: true, Merge: true, RecursionLimit: protowire.DefaultRecursionLimit}
	_ = unmarshalOpts

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.UnmarshalOptions{AllowPartial: true}.Unmarshal(bodyData, out)
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

	unmarshalOpts := proto.UnmarshalOptions{AllowPartial: true, Merge: true, RecursionLimit: protowire.DefaultRecursionLimit}
	_ = unmarshalOpts

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.UnmarshalOptions{AllowPartial: true}.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	vn0 := input.PathParam("0")

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.BytesType},
		protopack.String(vn0),
	}

	err = unmarshalOpts.Unmarshal(vn1.Marshal(), out)
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

	unmarshalOpts := proto.UnmarshalOptions{AllowPartial: true, Merge: true, RecursionLimit: protowire.DefaultRecursionLimit}
	_ = unmarshalOpts

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.UnmarshalOptions{AllowPartial: true}.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	vn0 := input.PathParam("0")

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.BytesType},
		protopack.String(vn0),
	}

	err = unmarshalOpts.Unmarshal(vn1.Marshal(), out)
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

	unmarshalOpts := proto.UnmarshalOptions{AllowPartial: true, Merge: true, RecursionLimit: protowire.DefaultRecursionLimit}
	_ = unmarshalOpts

	vn4 := input.Query().Get("author")

	vn5tmp, err := strconv.ParseBool(strings.ToLower(vn4))
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "includeAuthor is not a valid bool: %s", err)
	}

	vn5 := protopack.Message{
		protopack.Tag{Number: 3, Type: protopack.VarintType},
		protopack.Bool(vn5tmp),
	}

	err = unmarshalOpts.Unmarshal(vn5.Marshal(), out)
	if err != nil {
		return err
	}

	vn0 := input.PathParam("0")

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.BytesType},
		protopack.String(vn0),
	}

	err = unmarshalOpts.Unmarshal(vn1.Marshal(), out)
	if err != nil {
		return err
	}

	vn2 := input.PathParam("1")

	vn3tmp, err := strconv.ParseInt(vn2, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "book is not a valid int: %s", err)
	}

	vn3 := protopack.Message{
		protopack.Tag{Number: 2, Type: protopack.VarintType},
		protopack.Varint(vn3tmp),
	}

	err = unmarshalOpts.Unmarshal(vn3.Marshal(), out)
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

	unmarshalOpts := proto.UnmarshalOptions{AllowPartial: true, Merge: true, RecursionLimit: protowire.DefaultRecursionLimit}
	_ = unmarshalOpts

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.UnmarshalOptions{AllowPartial: true}.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	vn0 := input.PathParam("0")

	// book.shelf_id
	vn1 := protopack.Message{
		protopack.Tag{Number: 5, Type: protopack.BytesType},
		protopack.String(vn0),
	}
	// book.shelf_id
	vn2 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.BytesType},
		protopack.Bytes(vn1.Marshal()),
	}

	err = unmarshalOpts.Unmarshal(vn2.Marshal(), out)
	if err != nil {
		return err
	}

	vn3 := input.PathParam("1")

	// book.id
	vn4 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.BytesType},
		protopack.String(vn3),
	}
	// book.id
	vn5 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.BytesType},
		protopack.Bytes(vn4.Marshal()),
	}

	err = unmarshalOpts.Unmarshal(vn5.Marshal(), out)
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

	unmarshalOpts := proto.UnmarshalOptions{AllowPartial: true, Merge: true, RecursionLimit: protowire.DefaultRecursionLimit}
	_ = unmarshalOpts

	bodyData, err := io.ReadAll(input.Body())
	if err != nil {
		return err
	}
	if len(bodyData) > 0 {
		err = protojson.UnmarshalOptions{AllowPartial: true}.Unmarshal(bodyData, out)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to unmarshal body: %s", err)
		}
	}

	vn0 := input.PathParam("0")

	// book.shelf_id
	vn1 := protopack.Message{
		protopack.Tag{Number: 5, Type: protopack.BytesType},
		protopack.String(vn0),
	}
	// book.shelf_id
	vn2 := protopack.Message{
		protopack.Tag{Number: 2, Type: protopack.BytesType},
		protopack.Bytes(vn1.Marshal()),
	}

	err = unmarshalOpts.Unmarshal(vn2.Marshal(), out)
	if err != nil {
		return err
	}

	vn3 := input.PathParam("1")

	// book.id
	vn4 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.BytesType},
		protopack.String(vn3),
	}
	// book.id
	vn5 := protopack.Message{
		protopack.Tag{Number: 2, Type: protopack.BytesType},
		protopack.Bytes(vn4.Marshal()),
	}

	err = unmarshalOpts.Unmarshal(vn5.Marshal(), out)
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

	unmarshalOpts := proto.UnmarshalOptions{AllowPartial: true, Merge: true, RecursionLimit: protowire.DefaultRecursionLimit}
	_ = unmarshalOpts

	vn0 := input.PathParam("0")

	vn1tmp, err := strconv.ParseInt(vn0, 10, 64)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "author is not a valid int: %s", err)
	}

	vn1 := protopack.Message{
		protopack.Tag{Number: 1, Type: protopack.VarintType},
		protopack.Varint(vn1tmp),
	}

	err = unmarshalOpts.Unmarshal(vn1.Marshal(), out)
	if err != nil {
		return err
	}

	return nil
}
