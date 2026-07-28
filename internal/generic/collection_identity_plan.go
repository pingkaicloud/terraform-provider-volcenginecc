package generic

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// mergeIdentityCollectionPlans merges unknown computed element fields from
// prior state after pairing unordered collection elements by explicit identity.
func (r *genericResource) mergeIdentityCollectionPlans(config, prior, planned tftypes.Value) (tftypes.Value, error) {
	result := planned
	for _, identity := range r.collectionIdentities {
		attributeNames, err := r.collectionAttributeNames(identity.PropertyPath)
		if err != nil {
			return planned, err
		}
		if len(attributeNames) != 1 {
			// Nested collection traversal requires selecting parent collection
			// elements by identity first. Until that path is implemented, leave
			// its framework plan unchanged instead of guessing by index.
			continue
		}
		attributePath := terraformAttributePath(attributeNames)

		configCollection, err := terraformValueAtPath(config, attributePath)
		if err != nil {
			return planned, fmt.Errorf("reading config collection %q: %w", identity.PropertyPath, err)
		}
		priorCollection, err := terraformValueAtPath(prior, attributePath)
		if err != nil {
			return planned, fmt.Errorf("reading prior collection %q: %w", identity.PropertyPath, err)
		}
		plannedCollection, err := terraformValueAtPath(result, attributePath)
		if err != nil {
			return planned, fmt.Errorf("reading planned collection %q: %w", identity.PropertyPath, err)
		}

		computedFields, err := r.collectionComputedFields(attributeNames)
		if err != nil {
			return planned, err
		}
		identifierNames, err := r.identifierAttributeNames(identity.IdentifierPaths)
		if err != nil {
			return planned, err
		}
		merged, err := mergeTerraformCollectionPlan(configCollection, priorCollection, plannedCollection, identifierNames, computedFields)
		if err != nil {
			// An incomplete or duplicate identity is an explicit safe
			// degradation: preserve the framework plan without guessing.
			continue
		}

		result, err = replaceTerraformValueAtPath(result, attributePath, merged)
		if err != nil {
			return planned, fmt.Errorf("setting planned collection %q: %w", identity.PropertyPath, err)
		}
	}
	return result, nil
}

// collectionAttributeNames converts a Cloud Control property pointer into
// Terraform attribute names while retaining nested collection paths.
func (r *genericResource) collectionAttributeNames(propertyPath string) ([]string, error) {
	segments := strings.Split(strings.TrimPrefix(propertyPath, "/"), "/")
	names := make([]string, 0, len(segments))
	for _, segment := range segments {
		name, ok := r.ccToTfNameMap[segment]
		if !ok {
			return nil, fmt.Errorf("attribute name mapping not found for collection segment %q", segment)
		}
		names = append(names, name)
	}
	return names, nil
}

// identifierAttributeNames converts element-relative Cloud Control identity
// pointers into Terraform attribute paths in their declared order.
func (r *genericResource) identifierAttributeNames(identifierPaths []string) ([][]string, error) {
	result := make([][]string, 0, len(identifierPaths))
	for _, identifierPath := range identifierPaths {
		segments := strings.Split(strings.TrimPrefix(identifierPath, "/"), "/")
		names := make([]string, 0, len(segments))
		for _, segment := range segments {
			name, ok := r.ccToTfNameMap[segment]
			if !ok {
				return nil, fmt.Errorf("attribute name mapping not found for identity segment %q", segment)
			}
			names = append(names, name)
		}
		result = append(result, names)
	}
	return result, nil
}

// collectionComputedFields returns the top-level computed fields of an
// identity-bearing nested collection. Nested paths are rejected until their
// owning collection can be resolved without ambiguous element traversal.
func (r *genericResource) collectionComputedFields(attributeNames []string) (map[string]bool, error) {
	if len(attributeNames) != 1 {
		return nil, fmt.Errorf("nested identity collection paths are not supported in plan merge: %v", attributeNames)
	}
	attribute, ok := r.tfSchema.Attributes[attributeNames[0]]
	if !ok {
		return nil, fmt.Errorf("collection attribute %q not found in Terraform schema", attributeNames[0])
	}

	var nested map[string]schema.Attribute
	switch value := attribute.(type) {
	case schema.SetNestedAttribute:
		nested = value.NestedObject.Attributes
	case schema.ListNestedAttribute:
		nested = value.NestedObject.Attributes
	default:
		return nil, fmt.Errorf("identity collection %q is not a nested set or list", attributeNames[0])
	}

	result := make(map[string]bool, len(nested))
	for name, child := range nested {
		result[name] = child.IsComputed()
	}
	return result, nil
}

// terraformAttributePath builds a Terraform value path from attribute names.
func terraformAttributePath(names []string) *tftypes.AttributePath {
	result := tftypes.NewAttributePath()
	for _, name := range names {
		result = result.WithAttributeName(name)
	}
	return result
}

// terraformValueAtPath reads a Terraform value and verifies that the complete
// requested path was consumed.
func terraformValueAtPath(value tftypes.Value, attributePath *tftypes.AttributePath) (tftypes.Value, error) {
	found, remaining, err := tftypes.WalkAttributePath(value, attributePath)
	if err != nil {
		return tftypes.Value{}, err
	}
	if remaining != nil && len(remaining.Steps()) > 0 {
		return tftypes.Value{}, fmt.Errorf("path was not fully consumed")
	}
	result, ok := found.(tftypes.Value)
	if !ok {
		return tftypes.Value{}, fmt.Errorf("path resolved to %T instead of tftypes.Value", found)
	}
	return result, nil
}

// replaceTerraformValueAtPath replaces one value during a type-preserving
// Terraform value transform.
func replaceTerraformValueAtPath(root tftypes.Value, target *tftypes.AttributePath, replacement tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(root, func(current *tftypes.AttributePath, value tftypes.Value) (tftypes.Value, error) {
		if current.Equal(target) {
			return replacement, nil
		}
		return value, nil
	})
}

// mergeTerraformCollectionPlan pairs prior, config, and planned objects by
// identity, then restores only unknown computed fields omitted from config.
func mergeTerraformCollectionPlan(config, prior, planned tftypes.Value, identifiers [][]string, computedFields map[string]bool) (tftypes.Value, error) {
	if config.IsNull() || !config.IsKnown() || prior.IsNull() || !prior.IsKnown() || planned.IsNull() || !planned.IsKnown() {
		return planned, nil
	}

	configElements, err := terraformCollectionElements(config)
	if err != nil {
		return planned, err
	}
	priorElements, err := terraformCollectionElements(prior)
	if err != nil {
		return planned, err
	}
	plannedElements, err := terraformCollectionElements(planned)
	if err != nil {
		return planned, err
	}
	priorByIdentity, err := indexTerraformElements(priorElements, identifiers)
	if err != nil {
		return planned, err
	}
	configByIdentity, configIndexErr := indexTerraformElements(configElements, identifiers)
	configByIdentityOK := configIndexErr == nil

	merged := make([]tftypes.Value, 0, len(plannedElements))
	for plannedIndex, plannedElement := range plannedElements {
		key, err := terraformElementIdentity(plannedElement, identifiers)
		if err != nil {
			return planned, err
		}
		priorElement, existed := priorByIdentity[key]
		if !existed {
			merged = append(merged, plannedElement)
			continue
		}
		configElement, configured := configElementForPlannedIdentity(configElements, configByIdentity, configByIdentityOK, key, plannedIndex)
		if !configured {
			merged = append(merged, plannedElement)
			continue
		}
		mergedElement, err := mergeTerraformElementPlan(configElement, priorElement, plannedElement, computedFields)
		if err != nil {
			return planned, err
		}
		merged = append(merged, mergedElement)
	}
	return tftypes.NewValue(planned.Type(), merged), nil
}

// configElementForPlannedIdentity returns the config element used for field
// ownership checks, falling back to planned position only after identity pairing.
func configElementForPlannedIdentity(
	configElements []tftypes.Value,
	configByIdentity map[string]tftypes.Value,
	configByIdentityOK bool,
	key string,
	plannedIndex int,
) (tftypes.Value, bool) {
	if configByIdentityOK {
		if configElement, ok := configByIdentity[key]; ok {
			return configElement, true
		}
	}
	if plannedIndex >= len(configElements) {
		return tftypes.Value{}, false
	}
	return configElements[plannedIndex], true
}

// canonicalizeIdentityCollectionState sorts remote or planned collections by
// the objective elementIdentifier order while preserving complete element
// values.
func (r *genericResource) canonicalizeIdentityCollectionState(remote tftypes.Value) (tftypes.Value, error) {
	return r.canonicalizeIdentityCollectionStateWithMode(remote, false)
}

// canonicalizeIdentityCollectionPlan sorts every collection whose identity is
// fully known and leaves unsafe plan-time identities unchanged.
func (r *genericResource) canonicalizeIdentityCollectionPlan(planned tftypes.Value) tftypes.Value {
	result, _ := r.canonicalizeIdentityCollectionStateWithMode(planned, true)
	return result
}

// canonicalizeIdentityCollectionStateWithMode implements strict Read/state
// sorting and best-effort Plan sorting without ever using index fallback.
func (r *genericResource) canonicalizeIdentityCollectionStateWithMode(remote tftypes.Value, allowUnsafe bool) (tftypes.Value, error) {
	result := remote
	for _, identity := range r.collectionIdentities {
		attributeNames, err := r.collectionAttributeNames(identity.PropertyPath)
		if err != nil {
			if allowUnsafe {
				continue
			}
			return remote, err
		}
		if len(attributeNames) != 1 {
			continue
		}
		attributePath := terraformAttributePath(attributeNames)
		remoteCollection, err := terraformValueAtPath(result, attributePath)
		if err != nil {
			if allowUnsafe {
				continue
			}
			return remote, fmt.Errorf("reading remote collection %q: %w", identity.PropertyPath, err)
		}
		identifierNames, err := r.identifierAttributeNames(identity.IdentifierPaths)
		if err != nil {
			if allowUnsafe {
				continue
			}
			return remote, err
		}
		canonical, err := canonicalizeTerraformCollectionState(remoteCollection, identifierNames)
		if err != nil {
			if allowUnsafe {
				continue
			}
			return remote, fmt.Errorf("canonicalizing collection %q: %w", identity.PropertyPath, err)
		}
		result, err = replaceTerraformValueAtPath(result, attributePath, canonical)
		if err != nil {
			if allowUnsafe {
				continue
			}
			return remote, fmt.Errorf("setting remote collection %q: %w", identity.PropertyPath, err)
		}
	}
	return result, nil
}

// canonicalizeTerraformCollectionState orders a known Terraform collection by
// its scalar identity tuple. Null or unknown collections keep their value.
func canonicalizeTerraformCollectionState(remote tftypes.Value, identifiers [][]string) (tftypes.Value, error) {
	if remote.IsNull() || !remote.IsKnown() {
		return remote, nil
	}
	remoteElements, err := terraformCollectionElements(remote)
	if err != nil {
		return remote, err
	}
	type sortableElement struct {
		value    tftypes.Value
		identity []canonicalIdentityScalar
	}
	sortable := make([]sortableElement, 0, len(remoteElements))
	seen := make(map[string]struct{}, len(remoteElements))
	for _, remoteElement := range remoteElements {
		identity, err := terraformElementIdentityScalars(remoteElement, identifiers)
		if err != nil {
			return remote, err
		}
		key, err := collectionIdentityKey(identity)
		if err != nil {
			return remote, err
		}
		if _, exists := seen[string(key)]; exists {
			return remote, fmt.Errorf("duplicate collection identity %s", key)
		}
		seen[string(key)] = struct{}{}
		sortable = append(sortable, sortableElement{value: remoteElement, identity: identity})
	}
	var compareErr error
	sort.SliceStable(sortable, func(left, right int) bool {
		comparison, err := compareCanonicalIdentity(sortable[left].identity, sortable[right].identity)
		if err != nil {
			compareErr = err
			return false
		}
		return comparison < 0
	})
	if compareErr != nil {
		return remote, compareErr
	}
	canonical := make([]tftypes.Value, len(sortable))
	for index, element := range sortable {
		canonical[index] = element.value
	}
	return tftypes.NewValue(remote.Type(), canonical), nil
}

// terraformCollectionElements decodes either a Terraform set or list without
// imposing index identity semantics.
func terraformCollectionElements(value tftypes.Value) ([]tftypes.Value, error) {
	var elements []tftypes.Value
	if err := value.As(&elements); err != nil {
		return nil, err
	}
	return elements, nil
}

// indexTerraformElements indexes known unique identities and rejects unsafe
// collections rather than selecting an arbitrary duplicate.
func indexTerraformElements(elements []tftypes.Value, identifiers [][]string) (map[string]tftypes.Value, error) {
	result := make(map[string]tftypes.Value, len(elements))
	for _, element := range elements {
		key, err := terraformElementIdentity(element, identifiers)
		if err != nil {
			return nil, err
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate collection identity %s", key)
		}
		result[key] = element
	}
	return result, nil
}

// terraformElementIdentity encodes typed, known scalar identity values in
// declaration order so different Terraform value types cannot collide.
func terraformElementIdentity(element tftypes.Value, identifiers [][]string) (string, error) {
	identity, err := terraformElementIdentityScalars(element, identifiers)
	if err != nil {
		return "", err
	}
	key, err := collectionIdentityKey(identity)
	return string(key), err
}

// terraformElementIdentityScalars extracts known Terraform string, number, or
// boolean identity fields in elementIdentifier order.
func terraformElementIdentityScalars(element tftypes.Value, identifiers [][]string) ([]canonicalIdentityScalar, error) {
	current := element
	parts := make([]canonicalIdentityScalar, 0, len(identifiers))
	for _, identifier := range identifiers {
		current = element
		for _, name := range identifier {
			var object map[string]tftypes.Value
			if err := current.As(&object); err != nil {
				return nil, err
			}
			var ok bool
			current, ok = object[name]
			if !ok {
				return nil, fmt.Errorf("identity attribute %q is missing", name)
			}
		}
		if current.IsNull() || !current.IsKnown() {
			return nil, fmt.Errorf("identity attribute is null or unknown")
		}
		switch {
		case current.Type().Is(tftypes.String):
			var value string
			if err := current.As(&value); err != nil {
				return nil, err
			}
			parts = append(parts, canonicalIdentityScalar{kind: "string", stringValue: value})
		case current.Type().Is(tftypes.Number):
			value := new(big.Float)
			if err := current.As(value); err != nil {
				return nil, err
			}
			rational, _ := value.Rat(nil)
			parts = append(parts, canonicalIdentityScalar{kind: "number", numberValue: rational})
		case current.Type().Is(tftypes.Bool):
			var value bool
			if err := current.As(&value); err != nil {
				return nil, err
			}
			parts = append(parts, canonicalIdentityScalar{kind: "boolean", boolValue: value})
		default:
			return nil, fmt.Errorf("identity attribute type %s is not a supported scalar", current.Type())
		}
	}
	return parts, nil
}

// mergeTerraformElementPlan restores prior values when a computed field is
// null in config. This also corrects values copied from the wrong Set element
// by attribute modifiers before resource-level identity pairing runs.
func mergeTerraformElementPlan(config, prior, planned tftypes.Value, computedFields map[string]bool) (tftypes.Value, error) {
	var configObject, priorObject, plannedObject map[string]tftypes.Value
	if err := config.As(&configObject); err != nil {
		return planned, err
	}
	if err := prior.As(&priorObject); err != nil {
		return planned, err
	}
	if err := planned.As(&plannedObject); err != nil {
		return planned, err
	}
	for name := range plannedObject {
		if !computedFields[name] {
			continue
		}
		configValue, configured := configObject[name]
		priorValue, existed := priorObject[name]
		if configured && configValue.IsNull() && existed && priorValue.IsKnown() {
			plannedObject[name] = priorValue
		}
	}
	return tftypes.NewValue(planned.Type(), plannedObject), nil
}
