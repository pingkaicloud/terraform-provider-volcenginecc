package ccschema

import (
	"fmt"
	"strings"
)

// ValidateCollectionIdentities validates elementIdentifier metadata after schema
// references have been expanded. Create-only identifiers are allowed because they
// can be stable identities when the service returns them on every Read; callers
// must establish that readback contract before adding the metadata to a schema.
func (r *Resource) ValidateCollectionIdentities() error {
	if r == nil {
		return nil
	}

	resourceType := ""
	if r.TypeName != nil {
		resourceType = *r.TypeName
	}

	return r.validateCollectionIdentitiesInProperties(resourceType, nil, r.Properties)
}

// validateCollectionIdentitiesInProperties recursively validates collection
// identity metadata while retaining the full resource-relative collection path.
func (r *Resource) validateCollectionIdentitiesInProperties(resourceType string, parentPath []string, properties map[string]*Property) error {
	for name, property := range properties {
		if property == nil {
			continue
		}

		propertyPath := append(append([]string(nil), parentPath...), name)
		collectionPath := "/properties/" + strings.Join(propertyPath, "/")

		if len(property.ElementIdentifier) > 0 {
			if err := r.validateCollectionIdentity(resourceType, collectionPath, property); err != nil {
				return err
			}
		}

		switch property.Type.String() {
		case PropertyTypeObject, "":
			if err := r.validateCollectionIdentitiesInProperties(resourceType, propertyPath, property.Properties); err != nil {
				return err
			}
		case PropertyTypeArray:
			if property.Items != nil && property.Items.Type.String() == PropertyTypeObject {
				itemPath := append(append([]string(nil), propertyPath...), "*")
				if err := r.validateCollectionIdentitiesInProperties(resourceType, itemPath, property.Items.Properties); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateCollectionIdentity validates one unordered object collection and
// rejects ambiguous identity fields instead of permitting index-based fallback.
func (r *Resource) validateCollectionIdentity(resourceType, collectionPath string, property *Property) error {
	invalid := func(pointer, reason string) error {
		return fmt.Errorf("resource %s collection %s elementIdentifier %q: %s", resourceType, collectionPath, pointer, reason)
	}

	if property.Type.String() != PropertyTypeArray {
		return invalid(property.ElementIdentifier[0], "is only valid for type=array")
	}
	if property.InsertionOrder == nil || *property.InsertionOrder {
		return invalid(property.ElementIdentifier[0], "requires insertionOrder=false")
	}
	if property.Items == nil || property.Items.Type.String() != PropertyTypeObject {
		return invalid(property.ElementIdentifier[0], "requires items to resolve to type=object")
	}

	seen := make(map[string]struct{}, len(property.ElementIdentifier))
	for _, pointer := range property.ElementIdentifier {
		if _, ok := seen[pointer]; ok {
			return invalid(pointer, "is duplicated")
		}
		seen[pointer] = struct{}{}

		segments, err := relativeJSONPointerSegments(pointer)
		if err != nil {
			return invalid(pointer, err.Error())
		}

		current := property.Items
		for _, segment := range segments {
			if current.Type.String() != PropertyTypeObject {
				return invalid(pointer, fmt.Sprintf("segment %q traverses a non-object property", segment))
			}
			next, ok := current.Properties[segment]
			if !ok || next == nil {
				return invalid(pointer, fmt.Sprintf("does not resolve at segment %q", segment))
			}
			current = next
		}
		if current.Type.String() == PropertyTypeArray || current.Type.String() == PropertyTypeObject || current.Type.String() == "" {
			return invalid(pointer, "must resolve to a scalar property")
		}

		absolutePath := collectionPath + "/*" + pointer
		if r.ReadOnlyProperties.ContainsPathString(absolutePath) {
			return invalid(pointer, "must not reference a readOnly property")
		}
		if r.WriteOnlyProperties.ContainsPathString(absolutePath) {
			return invalid(pointer, "must not reference a writeOnly property")
		}
	}

	return nil
}

// relativeJSONPointerSegments parses and decodes an RFC 6901 pointer whose root
// is the containing array element.
func relativeJSONPointerSegments(pointer string) ([]string, error) {
	if pointer == "" || !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("must be a non-empty JSON Pointer relative to the array element")
	}

	raw := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	segments := make([]string, len(raw))
	for index, segment := range raw {
		var decoded strings.Builder
		for i := 0; i < len(segment); i++ {
			if segment[i] != '~' {
				decoded.WriteByte(segment[i])
				continue
			}
			if i+1 >= len(segment) || (segment[i+1] != '0' && segment[i+1] != '1') {
				return nil, fmt.Errorf("contains an invalid JSON Pointer escape")
			}
			i++
			if segment[i] == '0' {
				decoded.WriteByte('~')
			} else {
				decoded.WriteByte('/')
			}
		}
		segments[index] = decoded.String()
	}

	return segments, nil
}
