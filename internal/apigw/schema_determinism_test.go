package apigw

import (
	"testing"

	pgs "github.com/lyft/protoc-gen-star/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Mock types for testing schema generation determinism.
//
// pgs interfaces have unexported methods (accept, childAtPath, etc.) so we
// embed the interface to satisfy them. The unexported methods will panic if
// called, but our test paths never invoke them.

// mockSourceCodeInfo implements pgs.SourceCodeInfo (all exported methods).
type mockSourceCodeInfo struct{}

func (m *mockSourceCodeInfo) Location() *descriptorpb.SourceCodeInfo_Location { return nil }
func (m *mockSourceCodeInfo) LeadingComments() string                         { return "" }
func (m *mockSourceCodeInfo) LeadingDetachedComments() []string               { return nil }
func (m *mockSourceCodeInfo) TrailingComments() string                        { return "" }

// mockFieldType implements pgs.FieldType via embedding + overrides.
type mockFieldType struct {
	pgs.FieldType
	embed    pgs.Message
	isEmbed  bool
	isScalar bool
	pt       pgs.ProtoType
}

func (ft *mockFieldType) IsRepeated() bool           { return false }
func (ft *mockFieldType) IsMap() bool                { return false }
func (ft *mockFieldType) IsEnum() bool               { return false }
func (ft *mockFieldType) IsEmbed() bool              { return ft.isEmbed }
func (ft *mockFieldType) Embed() pgs.Message         { return ft.embed }
func (ft *mockFieldType) ProtoType() pgs.ProtoType   { return ft.pt }
func (ft *mockFieldType) Element() pgs.FieldTypeElem { return nil }
func (ft *mockFieldType) Key() pgs.FieldTypeElem     { return nil }

// mockField implements pgs.Field via embedding + overrides.
type mockField struct {
	pgs.Field
	name   pgs.Name
	jsonN  *string
	msg    pgs.Message
	oneOf  pgs.OneOf
	ftype  pgs.FieldType
	descPB *descriptorpb.FieldDescriptorProto
}

func (f *mockField) Name() pgs.Name                                 { return f.name }
func (f *mockField) FullyQualifiedName() string                     { return "." + string(f.name) }
func (f *mockField) Message() pgs.Message                           { return f.msg }
func (f *mockField) OneOf() pgs.OneOf                               { return f.oneOf }
func (f *mockField) InRealOneOf() bool                              { return f.oneOf != nil }
func (f *mockField) Type() pgs.FieldType                            { return f.ftype }
func (f *mockField) Descriptor() *descriptorpb.FieldDescriptorProto { return f.descPB }
func (f *mockField) SourceCodeInfo() pgs.SourceCodeInfo             { return &mockSourceCodeInfo{} }
func (f *mockField) Extension(_ *protoimpl.ExtensionInfo, _ interface{}) (bool, error) {
	return false, nil
}

// mockOneOf implements pgs.OneOf via embedding + overrides.
type mockOneOf struct {
	pgs.OneOf
	name   pgs.Name
	fields []pgs.Field
}

func (o *mockOneOf) Name() pgs.Name      { return o.name }
func (o *mockOneOf) Fields() []pgs.Field { return o.fields }

// mockMessage implements pgs.Message via embedding + overrides.
type mockMessage struct {
	pgs.Message
	name           pgs.Name
	fqn            string
	nonOneOfFields []pgs.Field
	oneOfs         []pgs.OneOf
	descPB         *descriptorpb.DescriptorProto
}

func (m *mockMessage) Name() pgs.Name                            { return m.name }
func (m *mockMessage) FullyQualifiedName() string                { return m.fqn }
func (m *mockMessage) IsWellKnown() bool                         { return false }
func (m *mockMessage) WellKnownType() pgs.WellKnownType          { return pgs.UnknownWKT }
func (m *mockMessage) NonOneOfFields() []pgs.Field               { return m.nonOneOfFields }
func (m *mockMessage) OneOfs() []pgs.OneOf                       { return m.oneOfs }
func (m *mockMessage) RealOneOfs() []pgs.OneOf                   { return m.oneOfs }
func (m *mockMessage) SourceCodeInfo() pgs.SourceCodeInfo        { return &mockSourceCodeInfo{} }
func (m *mockMessage) Descriptor() *descriptorpb.DescriptorProto { return m.descPB }
func (m *mockMessage) Messages() []pgs.Message                   { return nil }
func (m *mockMessage) AllMessages() []pgs.Message                { return nil }
func (m *mockMessage) MapEntries() []pgs.Message                 { return nil }
func (m *mockMessage) Enums() []pgs.Enum                         { return nil }
func (m *mockMessage) AllEnums() []pgs.Enum                      { return nil }
func (m *mockMessage) DefinedExtensions() []pgs.Extension        { return nil }
func (m *mockMessage) Extension(_ *protoimpl.ExtensionInfo, _ interface{}) (bool, error) {
	return false, nil
}

// Helper constructors

func newMockDescriptorProto() *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Options: &descriptorpb.MessageOptions{},
	}
}

func newMockFieldDescriptorProto(jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		JsonName: proto.String(jsonName),
		Options:  &descriptorpb.FieldOptions{},
	}
}

func newStringField(name string, parent pgs.Message) *mockField {
	return &mockField{
		name:  pgs.Name(name),
		jsonN: proto.String(name),
		msg:   parent,
		ftype: &mockFieldType{
			isScalar: true,
			pt:       pgs.StringT,
		},
		descPB: newMockFieldDescriptorProto(name),
	}
}

func newEmbedField(name string, parent pgs.Message, embed pgs.Message, oneOf pgs.OneOf) *mockField {
	return &mockField{
		name:  pgs.Name(name),
		jsonN: proto.String(name),
		msg:   parent,
		oneOf: oneOf,
		ftype: &mockFieldType{
			isEmbed: true,
			embed:   embed,
		},
		descPB: newMockFieldDescriptorProto(name),
	}
}

// buildConnectorRefScenario sets up the ConnectorRef test scenario.
//
// This mirrors the real-world c1 automations API where ConnectorRef is embedded in:
//   - AccountLifecycleAction.connectorRef (non-OneOf → nullable=nil)
//   - ConnectorAction.connectorRef        (OneOf → nullable=true)
func buildConnectorRefScenario() (*mockMessage, *mockMessage) {
	connectorRef := &mockMessage{
		name:   "ConnectorRef",
		fqn:    ".test.v1.ConnectorRef",
		descPB: newMockDescriptorProto(),
	}
	connectorRef.nonOneOfFields = []pgs.Field{
		newStringField("appId", connectorRef),
		newStringField("id", connectorRef),
	}

	// ParentA: AccountLifecycleAction (non-OneOf reference to ConnectorRef)
	parentA := &mockMessage{
		name:   "AccountLifecycleAction",
		fqn:    ".test.v1.AccountLifecycleAction",
		descPB: newMockDescriptorProto(),
	}
	parentA.nonOneOfFields = []pgs.Field{
		newEmbedField("connectorRef", parentA, connectorRef, nil),
		newStringField("actionName", parentA),
	}

	// ParentB: ConnectorAction (OneOf reference to ConnectorRef)
	parentB := &mockMessage{
		name:   "ConnectorAction",
		fqn:    ".test.v1.ConnectorAction",
		descPB: newMockDescriptorProto(),
	}
	connectorIdentifier := &mockOneOf{name: "connector_identifier"}
	connectorIdentifier.fields = []pgs.Field{
		newEmbedField("connectorRef", parentB, connectorRef, connectorIdentifier),
	}
	parentB.nonOneOfFields = []pgs.Field{newStringField("actionName", parentB)}
	parentB.oneOfs = []pgs.OneOf{connectorIdentifier}

	return parentA, parentB
}

// buildUserRefScenario sets up the UserRef test scenario.
//
// This mirrors the c1 automations API where UserRef is referenced from:
//   - CreateRevokeTasks.userRef (non-OneOf → nullable=nil)
//   - UpdateUser.userRef        (OneOf "user" → nullable=true)
func buildUserRefScenario() (*mockMessage, *mockMessage) {
	userRef := &mockMessage{
		name:   "UserRef",
		fqn:    ".test.v1.UserRef",
		descPB: newMockDescriptorProto(),
	}
	userRef.nonOneOfFields = []pgs.Field{newStringField("id", userRef)}

	// NonOneOf parent: CreateRevokeTasks
	nonOneOfParent := &mockMessage{
		name:   "CreateRevokeTasks",
		fqn:    ".test.v1.CreateRevokeTasks",
		descPB: newMockDescriptorProto(),
	}
	nonOneOfParent.nonOneOfFields = []pgs.Field{
		newEmbedField("userRef", nonOneOfParent, userRef, nil),
		newStringField("revokeAll", nonOneOfParent),
	}

	// OneOf parent: UpdateUser (has userRef inside oneof "user")
	oneOfParent := &mockMessage{
		name:   "UpdateUser",
		fqn:    ".test.v1.UpdateUser",
		descPB: newMockDescriptorProto(),
	}
	userOneOf := &mockOneOf{name: "user"}
	uuUserIdCel := newStringField("userIdCel", oneOfParent)
	uuUserIdCel.oneOf = userOneOf
	userOneOf.fields = []pgs.Field{
		newEmbedField("userRef", oneOfParent, userRef, userOneOf),
		uuUserIdCel,
	}
	oneOfParent.nonOneOfFields = []pgs.Field{newStringField("useSubjectUser", oneOfParent)}
	oneOfParent.oneOfs = []pgs.OneOf{userOneOf}

	return nonOneOfParent, oneOfParent
}

// getNullable is a helper that extracts nullable state from a schema in the container.
func getNullable(sc *schemaContainer, fqn string) (bool, bool) {
	proxy := sc.schemas.Value(fqn)
	if proxy == nil {
		return false, false
	}
	schema := proxy.Schema()
	if schema == nil {
		return false, false
	}
	return schema.Nullable != nil && *schema.Nullable, true
}

// TestSchemaDeterminism_ConnectorRef tests that ConnectorRef produces a
// deterministic schema when processed via msgTracker.SortedKeys().
func TestSchemaDeterminism_ConnectorRef(t *testing.T) {
	// Verify the bug: processing order affects nullable.
	t.Run("order_matters", func(t *testing.T) {
		parentA, parentB := buildConnectorRefScenario()

		// ParentA first → ConnectorRef NOT nullable
		sc1 := newSchemaContainer()
		sc1.Message(parentA, nil, nil, false, false)
		sc1.Message(parentB, nil, nil, false, false)
		nullable1, ok := getNullable(sc1, "test.v1.ConnectorRef")
		if !ok {
			t.Fatal("ConnectorRef schema not found")
		}

		// ParentB first → ConnectorRef IS nullable
		sc2 := newSchemaContainer()
		sc2.Message(parentB, nil, nil, false, false)
		sc2.Message(parentA, nil, nil, false, false)
		nullable2, ok := getNullable(sc2, "test.v1.ConnectorRef")
		if !ok {
			t.Fatal("ConnectorRef schema not found")
		}

		if nullable1 == nullable2 {
			t.Fatal("expected different nullable results when processing order changes")
		}
		if nullable1 {
			t.Error("expected ConnectorRef NOT nullable when non-OneOf parent is first")
		}
		if !nullable2 {
			t.Error("expected ConnectorRef nullable when OneOf parent is first")
		}
	})

	// Verify the fix: SortedKeys produces deterministic results across 100 iterations.
	// "test.v1.AccountLifecycleAction" < "test.v1.ConnectorAction" alphabetically,
	// so the non-OneOf parent is always processed first → ConnectorRef is never nullable.
	t.Run("sorted_keys_deterministic", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			parentA, parentB := buildConnectorRefScenario()

			mt := &msgTracker{}
			mt.Add(parentA)
			mt.Add(parentB)

			sc := newSchemaContainer()
			for _, k := range mt.SortedKeys() {
				sd := mt.messages[k]
				sc.Message(sd.msg, sd.filter, nil, false, false)
			}

			nullable, ok := getNullable(sc, "test.v1.ConnectorRef")
			if !ok {
				t.Fatalf("iteration %d: ConnectorRef schema not found", i)
			}
			if nullable {
				t.Fatalf("iteration %d: ConnectorRef should not be nullable with SortedKeys "+
					"(AccountLifecycleAction < ConnectorAction)", i)
			}
		}
	})
}

// TestSchemaDeterminism_UserRef tests the UserRef scenario from the c1
// automations API.
func TestSchemaDeterminism_UserRef(t *testing.T) {
	// Verify the bug: processing order affects nullable.
	t.Run("order_matters", func(t *testing.T) {
		nonOneOfParent, oneOfParent := buildUserRefScenario()

		// OneOf parent first → UserRef IS nullable
		sc1 := newSchemaContainer()
		sc1.Message(oneOfParent, nil, nil, false, false)
		sc1.Message(nonOneOfParent, nil, nil, false, false)
		nullable1, ok := getNullable(sc1, "test.v1.UserRef")
		if !ok {
			t.Fatal("UserRef schema not found")
		}

		// Non-OneOf parent first → UserRef NOT nullable
		sc2 := newSchemaContainer()
		sc2.Message(nonOneOfParent, nil, nil, false, false)
		sc2.Message(oneOfParent, nil, nil, false, false)
		nullable2, ok := getNullable(sc2, "test.v1.UserRef")
		if !ok {
			t.Fatal("UserRef schema not found")
		}

		if nullable1 == nullable2 {
			t.Fatal("expected different nullable results when processing order changes")
		}
		if !nullable1 {
			t.Error("expected UserRef nullable when OneOf parent is first")
		}
		if nullable2 {
			t.Error("expected UserRef NOT nullable when non-OneOf parent is first")
		}
	})

	// Verify the fix: SortedKeys produces deterministic results.
	// "test.v1.CreateRevokeTasks" < "test.v1.UpdateUser" alphabetically,
	// so the non-OneOf parent is always processed first → UserRef is never nullable.
	t.Run("sorted_keys_deterministic", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			nonOneOfParent, oneOfParent := buildUserRefScenario()

			mt := &msgTracker{}
			mt.Add(nonOneOfParent)
			mt.Add(oneOfParent)

			sc := newSchemaContainer()
			for _, k := range mt.SortedKeys() {
				sd := mt.messages[k]
				sc.Message(sd.msg, sd.filter, nil, false, false)
			}

			nullable, ok := getNullable(sc, "test.v1.UserRef")
			if !ok {
				t.Fatalf("iteration %d: UserRef schema not found", i)
			}
			if nullable {
				t.Fatalf("iteration %d: UserRef should not be nullable with SortedKeys "+
					"(CreateRevokeTasks < UpdateUser)", i)
			}
		}
	})
}
