// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: webhooks/v1/response.proto

package v1beta

import (
	_ "github.com/ductone/protoc-gen-apigw/apigw/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ResponseTest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// version contains the constant value "v1". Future versions of the Webhook Response
	// will use a different string.
	Version       string `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponseTest) Reset() {
	*x = ResponseTest{}
	mi := &file_webhooks_v1_response_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponseTest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponseTest) ProtoMessage() {}

func (x *ResponseTest) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponseTest.ProtoReflect.Descriptor instead.
func (*ResponseTest) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{0}
}

func (x *ResponseTest) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

type PolicyStep struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	PolicyStepId  string                 `protobuf:"bytes,1,opt,name=policy_step_id,json=policyStepId,proto3" json:"policy_step_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PolicyStep) Reset() {
	*x = PolicyStep{}
	mi := &file_webhooks_v1_response_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PolicyStep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PolicyStep) ProtoMessage() {}

func (x *PolicyStep) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PolicyStep.ProtoReflect.Descriptor instead.
func (*PolicyStep) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{1}
}

func (x *PolicyStep) GetPolicyStepId() string {
	if x != nil {
		return x.PolicyStepId
	}
	return ""
}

type ResponsePolicyApprovalStep struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// version contains the constant value "v1". Future versions of the Webhook Response
	// will use a different string.
	Version string `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	// Types that are valid to be assigned to Action:
	//
	//	*ResponsePolicyApprovalStep_Approve
	//	*ResponsePolicyApprovalStep_Deny
	//	*ResponsePolicyApprovalStep_Reassign
	//	*ResponsePolicyApprovalStep_ReplacePolicy
	Action        isResponsePolicyApprovalStep_Action `protobuf_oneof:"action"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponsePolicyApprovalStep) Reset() {
	*x = ResponsePolicyApprovalStep{}
	mi := &file_webhooks_v1_response_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponsePolicyApprovalStep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponsePolicyApprovalStep) ProtoMessage() {}

func (x *ResponsePolicyApprovalStep) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponsePolicyApprovalStep.ProtoReflect.Descriptor instead.
func (*ResponsePolicyApprovalStep) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{2}
}

func (x *ResponsePolicyApprovalStep) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *ResponsePolicyApprovalStep) GetAction() isResponsePolicyApprovalStep_Action {
	if x != nil {
		return x.Action
	}
	return nil
}

func (x *ResponsePolicyApprovalStep) GetApprove() *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove {
	if x != nil {
		if x, ok := x.Action.(*ResponsePolicyApprovalStep_Approve); ok {
			return x.Approve
		}
	}
	return nil
}

func (x *ResponsePolicyApprovalStep) GetDeny() *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny {
	if x != nil {
		if x, ok := x.Action.(*ResponsePolicyApprovalStep_Deny); ok {
			return x.Deny
		}
	}
	return nil
}

func (x *ResponsePolicyApprovalStep) GetReassign() *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign {
	if x != nil {
		if x, ok := x.Action.(*ResponsePolicyApprovalStep_Reassign); ok {
			return x.Reassign
		}
	}
	return nil
}

func (x *ResponsePolicyApprovalStep) GetReplacePolicy() *ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy {
	if x != nil {
		if x, ok := x.Action.(*ResponsePolicyApprovalStep_ReplacePolicy); ok {
			return x.ReplacePolicy
		}
	}
	return nil
}

type isResponsePolicyApprovalStep_Action interface {
	isResponsePolicyApprovalStep_Action()
}

type ResponsePolicyApprovalStep_Approve struct {
	Approve *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove `protobuf:"bytes,100,opt,name=approve,proto3,oneof"`
}

type ResponsePolicyApprovalStep_Deny struct {
	Deny *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny `protobuf:"bytes,101,opt,name=deny,proto3,oneof"`
}

type ResponsePolicyApprovalStep_Reassign struct {
	Reassign *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign `protobuf:"bytes,102,opt,name=reassign,proto3,oneof"`
}

type ResponsePolicyApprovalStep_ReplacePolicy struct {
	ReplacePolicy *ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy `protobuf:"bytes,103,opt,name=replace_policy,json=replacePolicy,proto3,oneof"`
}

func (*ResponsePolicyApprovalStep_Approve) isResponsePolicyApprovalStep_Action() {}

func (*ResponsePolicyApprovalStep_Deny) isResponsePolicyApprovalStep_Action() {}

func (*ResponsePolicyApprovalStep_Reassign) isResponsePolicyApprovalStep_Action() {}

func (*ResponsePolicyApprovalStep_ReplacePolicy) isResponsePolicyApprovalStep_Action() {}

type ResponsePolicyPostAction struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// version contains the constant value "v1". Future versions of the Webhook Response
	// will use a different string.
	Version       string `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponsePolicyPostAction) Reset() {
	*x = ResponsePolicyPostAction{}
	mi := &file_webhooks_v1_response_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponsePolicyPostAction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponsePolicyPostAction) ProtoMessage() {}

func (x *ResponsePolicyPostAction) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponsePolicyPostAction.ProtoReflect.Descriptor instead.
func (*ResponsePolicyPostAction) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{3}
}

func (x *ResponsePolicyPostAction) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

type ResponseProvisionStep struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// version contains the constant value "v1". Future versions of the Webhook Response
	// will use a different string.
	Version string `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	// Types that are valid to be assigned to Outcome:
	//
	//	*ResponseProvisionStep_Complete
	//	*ResponseProvisionStep_Errored
	Outcome       isResponseProvisionStep_Outcome `protobuf_oneof:"outcome"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponseProvisionStep) Reset() {
	*x = ResponseProvisionStep{}
	mi := &file_webhooks_v1_response_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponseProvisionStep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponseProvisionStep) ProtoMessage() {}

func (x *ResponseProvisionStep) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponseProvisionStep.ProtoReflect.Descriptor instead.
func (*ResponseProvisionStep) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{4}
}

func (x *ResponseProvisionStep) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *ResponseProvisionStep) GetOutcome() isResponseProvisionStep_Outcome {
	if x != nil {
		return x.Outcome
	}
	return nil
}

func (x *ResponseProvisionStep) GetComplete() *ResponseProvisionStep_ResponseProvisionStepComplete {
	if x != nil {
		if x, ok := x.Outcome.(*ResponseProvisionStep_Complete); ok {
			return x.Complete
		}
	}
	return nil
}

func (x *ResponseProvisionStep) GetErrored() *ResponseProvisionStep_ResponseProvisionStepErrored {
	if x != nil {
		if x, ok := x.Outcome.(*ResponseProvisionStep_Errored); ok {
			return x.Errored
		}
	}
	return nil
}

type isResponseProvisionStep_Outcome interface {
	isResponseProvisionStep_Outcome()
}

type ResponseProvisionStep_Complete struct {
	Complete *ResponseProvisionStep_ResponseProvisionStepComplete `protobuf:"bytes,100,opt,name=complete,proto3,oneof"`
}

type ResponseProvisionStep_Errored struct {
	Errored *ResponseProvisionStep_ResponseProvisionStepErrored `protobuf:"bytes,101,opt,name=errored,proto3,oneof"`
}

func (*ResponseProvisionStep_Complete) isResponseProvisionStep_Outcome() {}

func (*ResponseProvisionStep_Errored) isResponseProvisionStep_Outcome() {}

type ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// optional comment
	Comment       string `protobuf:"bytes,1,opt,name=comment,proto3" json:"comment,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove) Reset() {
	*x = ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove{}
	mi := &file_webhooks_v1_response_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove) ProtoMessage() {}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove.ProtoReflect.Descriptor instead.
func (*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{2, 0}
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

type ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// optional comment
	Comment       string `protobuf:"bytes,1,opt,name=comment,proto3" json:"comment,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny) Reset() {
	*x = ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny{}
	mi := &file_webhooks_v1_response_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny) ProtoMessage() {}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny.ProtoReflect.Descriptor instead.
func (*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{2, 1}
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

type ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// optional comment
	Comment        string   `protobuf:"bytes,1,opt,name=comment,proto3" json:"comment,omitempty"`
	NewStepUserIds []string `protobuf:"bytes,2,rep,name=new_step_user_ids,json=newStepUserIds,proto3" json:"new_step_user_ids,omitempty"`
	unknownFields  protoimpl.UnknownFields
	sizeCache      protoimpl.SizeCache
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign) Reset() {
	*x = ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign{}
	mi := &file_webhooks_v1_response_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign) ProtoMessage() {}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign.ProtoReflect.Descriptor instead.
func (*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{2, 2}
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign) GetNewStepUserIds() []string {
	if x != nil {
		return x.NewStepUserIds
	}
	return nil
}

type ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Comment       string                 `protobuf:"bytes,1,opt,name=comment,proto3" json:"comment,omitempty"`
	PolicySteps   []*PolicyStep          `protobuf:"bytes,2,rep,name=policy_steps,json=policySteps,proto3" json:"policy_steps,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy) Reset() {
	*x = ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy{}
	mi := &file_webhooks_v1_response_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy) ProtoMessage() {}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy.ProtoReflect.Descriptor instead.
func (*ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{2, 3}
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

func (x *ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy) GetPolicySteps() []*PolicyStep {
	if x != nil {
		return x.PolicySteps
	}
	return nil
}

type ResponseProvisionStep_ResponseProvisionStepComplete struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// optional comment
	Comment       string `protobuf:"bytes,1,opt,name=comment,proto3" json:"comment,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponseProvisionStep_ResponseProvisionStepComplete) Reset() {
	*x = ResponseProvisionStep_ResponseProvisionStepComplete{}
	mi := &file_webhooks_v1_response_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponseProvisionStep_ResponseProvisionStepComplete) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponseProvisionStep_ResponseProvisionStepComplete) ProtoMessage() {}

func (x *ResponseProvisionStep_ResponseProvisionStepComplete) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponseProvisionStep_ResponseProvisionStepComplete.ProtoReflect.Descriptor instead.
func (*ResponseProvisionStep_ResponseProvisionStepComplete) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{4, 0}
}

func (x *ResponseProvisionStep_ResponseProvisionStepComplete) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

type ResponseProvisionStep_ResponseProvisionStepErrored struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// optional comment
	Comment       string `protobuf:"bytes,1,opt,name=comment,proto3" json:"comment,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResponseProvisionStep_ResponseProvisionStepErrored) Reset() {
	*x = ResponseProvisionStep_ResponseProvisionStepErrored{}
	mi := &file_webhooks_v1_response_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponseProvisionStep_ResponseProvisionStepErrored) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponseProvisionStep_ResponseProvisionStepErrored) ProtoMessage() {}

func (x *ResponseProvisionStep_ResponseProvisionStepErrored) ProtoReflect() protoreflect.Message {
	mi := &file_webhooks_v1_response_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponseProvisionStep_ResponseProvisionStepErrored.ProtoReflect.Descriptor instead.
func (*ResponseProvisionStep_ResponseProvisionStepErrored) Descriptor() ([]byte, []int) {
	return file_webhooks_v1_response_proto_rawDescGZIP(), []int{4, 1}
}

func (x *ResponseProvisionStep_ResponseProvisionStepErrored) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

var File_webhooks_v1_response_proto protoreflect.FileDescriptor

const file_webhooks_v1_response_proto_rawDesc = "" +
	"\n" +
	"\x1awebhooks/v1/response.proto\x12\vwebhooks.v1\x1a\x14apigw/v1/apigw.proto\"2\n" +
	"\fResponseTest\x12\x18\n" +
	"\aversion\x18\x01 \x01(\tR\aversion:\b\xaa\xde\x03\x04\n" +
	"\x02\x18\x01\"2\n" +
	"\n" +
	"PolicyStep\x12$\n" +
	"\x0epolicy_step_id\x18\x01 \x01(\tR\fpolicyStepId\"\xd2\x06\n" +
	"\x1aResponsePolicyApprovalStep\x12\x18\n" +
	"\aversion\x18\x01 \x01(\tR\aversion\x12e\n" +
	"\aapprove\x18d \x01(\v2I.webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepApproveH\x00R\aapprove\x12\\\n" +
	"\x04deny\x18e \x01(\v2F.webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepDenyH\x00R\x04deny\x12h\n" +
	"\breassign\x18f \x01(\v2J.webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepReassignH\x00R\breassign\x12t\n" +
	"\x0ereplace_policy\x18g \x01(\v2K.webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalReplacePolicyH\x00R\rreplacePolicy\x1a=\n" +
	"!ResponsePolicyApprovalStepApprove\x12\x18\n" +
	"\acomment\x18\x01 \x01(\tR\acomment\x1a:\n" +
	"\x1eResponsePolicyApprovalStepDeny\x12\x18\n" +
	"\acomment\x18\x01 \x01(\tR\acomment\x1ai\n" +
	"\"ResponsePolicyApprovalStepReassign\x12\x18\n" +
	"\acomment\x18\x01 \x01(\tR\acomment\x12)\n" +
	"\x11new_step_user_ids\x18\x02 \x03(\tR\x0enewStepUserIds\x1a{\n" +
	"#ResponsePolicyApprovalReplacePolicy\x12\x18\n" +
	"\acomment\x18\x01 \x01(\tR\acomment\x12:\n" +
	"\fpolicy_steps\x18\x02 \x03(\v2\x17.webhooks.v1.PolicyStepR\vpolicySteps:\b\xaa\xde\x03\x04\n" +
	"\x02\x18\x01B\b\n" +
	"\x06action\">\n" +
	"\x18ResponsePolicyPostAction\x12\x18\n" +
	"\aversion\x18\x01 \x01(\tR\aversion:\b\xaa\xde\x03\x04\n" +
	"\x02\x18\x01\"\xf8\x02\n" +
	"\x15ResponseProvisionStep\x12\x18\n" +
	"\aversion\x18\x01 \x01(\tR\aversion\x12^\n" +
	"\bcomplete\x18d \x01(\v2@.webhooks.v1.ResponseProvisionStep.ResponseProvisionStepCompleteH\x00R\bcomplete\x12[\n" +
	"\aerrored\x18e \x01(\v2?.webhooks.v1.ResponseProvisionStep.ResponseProvisionStepErroredH\x00R\aerrored\x1a9\n" +
	"\x1dResponseProvisionStepComplete\x12\x18\n" +
	"\acomment\x18\x01 \x01(\tR\acomment\x1a8\n" +
	"\x1cResponseProvisionStepErrored\x12\x18\n" +
	"\acomment\x18\x01 \x01(\tR\acomment:\b\xaa\xde\x03\x04\n" +
	"\x02\x18\x01B\t\n" +
	"\aoutcomeB1Z/gitlab.com/ductone/c1/pkg/pb/c1/webhooks/v1betab\x06proto3"

var (
	file_webhooks_v1_response_proto_rawDescOnce sync.Once
	file_webhooks_v1_response_proto_rawDescData []byte
)

func file_webhooks_v1_response_proto_rawDescGZIP() []byte {
	file_webhooks_v1_response_proto_rawDescOnce.Do(func() {
		file_webhooks_v1_response_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_webhooks_v1_response_proto_rawDesc), len(file_webhooks_v1_response_proto_rawDesc)))
	})
	return file_webhooks_v1_response_proto_rawDescData
}

var file_webhooks_v1_response_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_webhooks_v1_response_proto_goTypes = []any{
	(*ResponseTest)(nil),               // 0: webhooks.v1.ResponseTest
	(*PolicyStep)(nil),                 // 1: webhooks.v1.PolicyStep
	(*ResponsePolicyApprovalStep)(nil), // 2: webhooks.v1.ResponsePolicyApprovalStep
	(*ResponsePolicyPostAction)(nil),   // 3: webhooks.v1.ResponsePolicyPostAction
	(*ResponseProvisionStep)(nil),      // 4: webhooks.v1.ResponseProvisionStep
	(*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepApprove)(nil),   // 5: webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepApprove
	(*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepDeny)(nil),      // 6: webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepDeny
	(*ResponsePolicyApprovalStep_ResponsePolicyApprovalStepReassign)(nil),  // 7: webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepReassign
	(*ResponsePolicyApprovalStep_ResponsePolicyApprovalReplacePolicy)(nil), // 8: webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalReplacePolicy
	(*ResponseProvisionStep_ResponseProvisionStepComplete)(nil),            // 9: webhooks.v1.ResponseProvisionStep.ResponseProvisionStepComplete
	(*ResponseProvisionStep_ResponseProvisionStepErrored)(nil),             // 10: webhooks.v1.ResponseProvisionStep.ResponseProvisionStepErrored
}
var file_webhooks_v1_response_proto_depIdxs = []int32{
	5,  // 0: webhooks.v1.ResponsePolicyApprovalStep.approve:type_name -> webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepApprove
	6,  // 1: webhooks.v1.ResponsePolicyApprovalStep.deny:type_name -> webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepDeny
	7,  // 2: webhooks.v1.ResponsePolicyApprovalStep.reassign:type_name -> webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalStepReassign
	8,  // 3: webhooks.v1.ResponsePolicyApprovalStep.replace_policy:type_name -> webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalReplacePolicy
	9,  // 4: webhooks.v1.ResponseProvisionStep.complete:type_name -> webhooks.v1.ResponseProvisionStep.ResponseProvisionStepComplete
	10, // 5: webhooks.v1.ResponseProvisionStep.errored:type_name -> webhooks.v1.ResponseProvisionStep.ResponseProvisionStepErrored
	1,  // 6: webhooks.v1.ResponsePolicyApprovalStep.ResponsePolicyApprovalReplacePolicy.policy_steps:type_name -> webhooks.v1.PolicyStep
	7,  // [7:7] is the sub-list for method output_type
	7,  // [7:7] is the sub-list for method input_type
	7,  // [7:7] is the sub-list for extension type_name
	7,  // [7:7] is the sub-list for extension extendee
	0,  // [0:7] is the sub-list for field type_name
}

func init() { file_webhooks_v1_response_proto_init() }
func file_webhooks_v1_response_proto_init() {
	if File_webhooks_v1_response_proto != nil {
		return
	}
	file_webhooks_v1_response_proto_msgTypes[2].OneofWrappers = []any{
		(*ResponsePolicyApprovalStep_Approve)(nil),
		(*ResponsePolicyApprovalStep_Deny)(nil),
		(*ResponsePolicyApprovalStep_Reassign)(nil),
		(*ResponsePolicyApprovalStep_ReplacePolicy)(nil),
	}
	file_webhooks_v1_response_proto_msgTypes[4].OneofWrappers = []any{
		(*ResponseProvisionStep_Complete)(nil),
		(*ResponseProvisionStep_Errored)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_webhooks_v1_response_proto_rawDesc), len(file_webhooks_v1_response_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_webhooks_v1_response_proto_goTypes,
		DependencyIndexes: file_webhooks_v1_response_proto_depIdxs,
		MessageInfos:      file_webhooks_v1_response_proto_msgTypes,
	}.Build()
	File_webhooks_v1_response_proto = out.File
	file_webhooks_v1_response_proto_goTypes = nil
	file_webhooks_v1_response_proto_depIdxs = nil
}
