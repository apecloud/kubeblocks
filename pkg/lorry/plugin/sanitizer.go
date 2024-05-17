/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package plugin

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/protobuf/descriptor"
	"github.com/golang/protobuf/proto"
	protobufdescriptor "github.com/golang/protobuf/protoc-gen-go/descriptor"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

// StripSecrets returns a wrapper around the original lorry gRPC message
// which has a Stringer implementation that serializes the message
// as one-line JSON, but without including secret information.
// Instead of the secret value(s), the string "***stripped***" is
// included in the result.
//
// StripSecrets itself is fast and therefore it is cheap to pass the
// result to logging functions which may or may not end up serializing
// the parameter depending on the current log level.
func StripSecrets(msg interface{}) fmt.Stringer {
	return &stripSecrets{msg, isKBSecret}
}

type stripSecrets struct {
	msg interface{}

	isSecretField func(field *protobufdescriptor.FieldDescriptorProto) bool
}

func (s *stripSecrets) String() string {
	// First convert to a generic representation. That's less efficient
	// than using reflect directly, but easier to work with.
	var parsed interface{}
	b, err := json.Marshal(s.msg)
	if err != nil {
		return fmt.Sprintf("<<json.Marshal %T: %s>>", s.msg, err)
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return fmt.Sprintf("<<json.Unmarshal %T: %s>>", s.msg, err)
	}

	// Now remove secrets from the generic representation of the message.
	s.strip(parsed, s.msg)

	// Re-encoded the stripped representation and return that.
	b, err = json.Marshal(parsed)
	if err != nil {
		return fmt.Sprintf("<<json.Marshal %T: %s>>", s.msg, err)
	}
	return string(b)
}

func (s *stripSecrets) strip(parsed interface{}, msg interface{}) {
	protobufMsg, ok := msg.(descriptor.Message)
	if !ok {
		// Not a protobuf message, so we are done.
		return
	}

	// The corresponding map in the parsed JSON representation.
	parsedFields, ok := parsed.(map[string]interface{})
	if !ok {
		// Probably nil.
		return
	}

	// Walk through all fields and replace those with ***stripped*** that
	// are marked as secret. This relies on protobuf adding "json:" tags
	// on each field where the name matches the field name in the protobuf
	// spec (like volume_capabilities). The field.GetJsonName() method returns
	// a different name (volumeCapabilities) which we don't use.
	_, md := descriptor.ForMessage(protobufMsg)
	fields := md.GetField()
	for _, field := range fields {
		if s.isSecretField(field) {
			// Overwrite only if already set.
			if _, ok := parsedFields[field.GetName()]; ok {
				parsedFields[field.GetName()] = "***stripped***"
			}
		} else if field.GetType() == protobufdescriptor.FieldDescriptorProto_TYPE_MESSAGE {
			// When we get here,
			// the type name is something like ".csi.v1.CapacityRange" (leading dot!)
			// and looking up "csi.v1.CapacityRange"
			// returns the type of a pointer to a pointer
			// to CapacityRange. We need a pointer to such
			// a value for recursive stripping.
			typeName := field.GetTypeName()
			typeName = strings.TrimPrefix(typeName, ".")
			t := proto.MessageType(typeName)
			if t == nil || t.Kind() != reflect.Ptr {
				// Shouldn't happen, but
				// better check anyway instead
				// of panicking.
				continue
			}
			v := reflect.New(t.Elem())

			// Recursively strip the message(s) that
			// the field contains.
			i := v.Interface()
			entry := parsedFields[field.GetName()]
			if slice, ok := entry.([]interface{}); ok {
				// Array of values, like VolumeCapabilities in CreateVolumeRequest.
				for _, entry := range slice {
					s.strip(entry, i)
				}
			} else {
				// Single value.
				s.strip(entry, i)
			}
		}
	}
}

// isKBSecret uses the kb_secret extension to
// determine whether a field contains secrets.
func isKBSecret(field *protobufdescriptor.FieldDescriptorProto) bool {
	ex, err := proto.GetExtension(field.Options, kbSecret)
	return err == nil && ex != nil && *ex.(*bool)
}

// Copied from the dbplugin spec db_plugin.pb.go
// to avoid a package dependency that would prevent usage of this package
// in repos using an older version of the spec.
//
// Future revision of the DB plugin spec must not change this extensions, otherwise
// they will break filtering in binaries based on the 1.0 version of the spec.
var kbSecret = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         1059,
	Name:          "plugin.v1.kb_secret",
	Tag:           "varint,1059,opt,name=kb_secret",
	Filename:      "db_plugin.proto",
}
