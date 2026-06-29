package generic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CollectionIdentity describes the JSON property path of an unordered object
// collection and the relative JSON Pointer paths that identify each element.
type CollectionIdentity struct {
	PropertyPath    string
	IdentifierPaths []string
	UniqueItems     bool
}

// collectionElementIdentityKey is a deterministic, type-preserving encoding of
// all configured identifier values. Composite identifiers retain pointer order.
type collectionElementIdentityKey string

// collectionElementPair records the logical identity and original indexes of a
// matched element. Indexes are never inferred as an identity fallback.
type collectionElementPair struct {
	Identity   collectionElementIdentityKey
	LeftIndex  int
	RightIndex int
}

// collectionElementReference records an unmatched element and its original index.
type collectionElementReference struct {
	Identity collectionElementIdentityKey
	Index    int
}

// collectionPairing separates matched and one-sided elements without deciding
// whether any non-identity field difference should be ignored.
type collectionPairing struct {
	Matched   []collectionElementPair
	LeftOnly  []collectionElementReference
	RightOnly []collectionElementReference
	Degraded  bool
	Reason    string
}

// unknownCollectionIdentityValue represents a Terraform unknown value before
// phase C wires Framework values into the collection pairing helper.
type unknownCollectionIdentityValue struct{}

var collectionIdentityUnknown = unknownCollectionIdentityValue{}

// indexedCollectionElement retains an element's original collection position.
type indexedCollectionElement struct {
	index int
}

// typedIdentityScalar preserves the Go value type in the canonical identity key.
type typedIdentityScalar struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// alignCollectionByIdentity orders remote elements by matching their identities
// to prior elements. New remote elements are appended, while missing, duplicate,
// or incomplete identities return an error instead of falling back to indexes.
func alignCollectionByIdentity(prior, remote []interface{}, identifierPaths []string) ([]interface{}, error) {
	pairing, err := pairCollectionsByIdentity(prior, remote, identifierPaths, true)
	if err != nil {
		return nil, err
	}
	if pairing.Degraded {
		return nil, fmt.Errorf("collection identity unavailable: %s", pairing.Reason)
	}

	aligned := make([]interface{}, 0, len(remote))
	for _, pair := range pairing.Matched {
		aligned = append(aligned, remote[pair.RightIndex])
	}
	for _, reference := range pairing.RightOnly {
		aligned = append(aligned, remote[reference.Index])
	}

	return aligned, nil
}

// normalizeIdentityCollections aligns current unordered collections to planned
// identity order before JSON Patch generation. Current-only elements are kept at
// the end so removals remain explicit, while planned-only elements remain absent
// from current so additions remain explicit.
func normalizeIdentityCollections(current, planned string, identities []CollectionIdentity) (string, string, error) {
	var currentRoot interface{}
	if err := json.Unmarshal([]byte(current), &currentRoot); err != nil {
		return "", "", fmt.Errorf("unmarshalling current desired state: %w", err)
	}

	var plannedRoot interface{}
	if err := json.Unmarshal([]byte(planned), &plannedRoot); err != nil {
		return "", "", fmt.Errorf("unmarshalling planned desired state: %w", err)
	}

	for _, identity := range identities {
		currentCollection, err := collectionAtJSONPointer(currentRoot, identity.PropertyPath)
		if err != nil {
			return "", "", fmt.Errorf("reading current collection %q: %w", identity.PropertyPath, err)
		}
		plannedCollection, err := collectionAtJSONPointer(plannedRoot, identity.PropertyPath)
		if err != nil {
			return "", "", fmt.Errorf("reading planned collection %q: %w", identity.PropertyPath, err)
		}

		alignedCurrent, err := alignCurrentToPlannedIdentity(currentCollection, plannedCollection, identity.IdentifierPaths)
		if err != nil {
			return "", "", fmt.Errorf("normalizing collection %q: %w", identity.PropertyPath, err)
		}
		if err := setCollectionAtJSONPointer(currentRoot, identity.PropertyPath, alignedCurrent); err != nil {
			return "", "", fmt.Errorf("setting current collection %q: %w", identity.PropertyPath, err)
		}
	}

	normalizedCurrent, err := json.Marshal(currentRoot)
	if err != nil {
		return "", "", fmt.Errorf("marshalling normalized current desired state: %w", err)
	}
	normalizedPlanned, err := json.Marshal(plannedRoot)
	if err != nil {
		return "", "", fmt.Errorf("marshalling normalized planned desired state: %w", err)
	}

	return string(normalizedCurrent), string(normalizedPlanned), nil
}

// alignCurrentToPlannedIdentity orders current elements by planned identity and
// appends current-only elements. This makes index-based JSON Patch operations
// target the intended identity without hiding additions or removals.
func alignCurrentToPlannedIdentity(current, planned []interface{}, identifierPaths []string) ([]interface{}, error) {
	pairing, err := pairCollectionsByIdentity(planned, current, identifierPaths, true)
	if err != nil {
		return nil, err
	}
	if pairing.Degraded {
		return nil, fmt.Errorf("collection identity unavailable: %s", pairing.Reason)
	}

	aligned := make([]interface{}, 0, len(current))
	for _, pair := range pairing.Matched {
		aligned = append(aligned, current[pair.RightIndex])
	}
	for _, reference := range pairing.RightOnly {
		aligned = append(aligned, current[reference.Index])
	}

	return aligned, nil
}

// pairCollectionsByIdentity pairs two unordered object collections by explicit
// identity. It preserves left order for matches and removals, right order for
// additions, and never falls back to array indexes.
func pairCollectionsByIdentity(left, right []interface{}, identifierPaths []string, uniqueItems bool) (collectionPairing, error) {
	if !uniqueItems && hasDuplicateCompleteElements(left, right) {
		return collectionPairing{
			Degraded: true,
			Reason:   "multiset contains complete duplicate elements without a safe identity",
		}, nil
	}
	if len(identifierPaths) == 0 {
		return collectionPairing{Degraded: true, Reason: "no elementIdentifier is configured"}, nil
	}

	leftByIdentity, leftOrder, err := indexCollectionByIdentity(left, identifierPaths)
	if err != nil {
		return collectionPairing{}, fmt.Errorf("indexing left collection: %w", err)
	}
	rightByIdentity, rightOrder, err := indexCollectionByIdentity(right, identifierPaths)
	if err != nil {
		return collectionPairing{}, fmt.Errorf("indexing right collection: %w", err)
	}

	result := collectionPairing{}
	for _, identity := range leftOrder {
		leftElement := leftByIdentity[identity]
		if rightElement, ok := rightByIdentity[identity]; ok {
			result.Matched = append(result.Matched, collectionElementPair{
				Identity:   identity,
				LeftIndex:  leftElement.index,
				RightIndex: rightElement.index,
			})
			continue
		}
		result.LeftOnly = append(result.LeftOnly, collectionElementReference{
			Identity: identity,
			Index:    leftElement.index,
		})
	}
	for _, identity := range rightOrder {
		if _, ok := leftByIdentity[identity]; ok {
			continue
		}
		result.RightOnly = append(result.RightOnly, collectionElementReference{
			Identity: identity,
			Index:    rightByIdentity[identity].index,
		})
	}

	return result, nil
}

// indexCollectionByIdentity extracts typed identity keys and rejects duplicate
// or incomplete identities so callers never guess element correspondence.
func indexCollectionByIdentity(collection []interface{}, identifierPaths []string) (map[collectionElementIdentityKey]indexedCollectionElement, []collectionElementIdentityKey, error) {
	byIdentity := make(map[collectionElementIdentityKey]indexedCollectionElement, len(collection))
	order := make([]collectionElementIdentityKey, 0, len(collection))
	for index, element := range collection {
		identity, err := collectionElementIdentity(element, identifierPaths)
		if err != nil {
			return nil, nil, fmt.Errorf("element %d: %w", index, err)
		}
		if _, exists := byIdentity[identity]; exists {
			return nil, nil, fmt.Errorf("duplicate identity %s", identity)
		}
		byIdentity[identity] = indexedCollectionElement{
			index: index,
		}
		order = append(order, identity)
	}
	return byIdentity, order, nil
}

// collectionElementIdentity builds a deterministic JSON key from all configured
// identifier paths. Null, missing, collection, and object values are rejected
// because they cannot safely identify an element.
func collectionElementIdentity(element interface{}, identifierPaths []string) (collectionElementIdentityKey, error) {
	values := make([]typedIdentityScalar, 0, len(identifierPaths))
	for _, identifierPath := range identifierPaths {
		value, found, err := valueAtJSONPointer(element, identifierPath)
		if err != nil {
			return "", fmt.Errorf("identifier path %q: %w", identifierPath, err)
		}
		if !found {
			return "", fmt.Errorf("identifier path %q is missing", identifierPath)
		}
		if value == nil {
			return "", fmt.Errorf("identifier path %q is null", identifierPath)
		}
		if _, unknown := value.(unknownCollectionIdentityValue); unknown {
			return "", fmt.Errorf("identifier path %q is unknown", identifierPath)
		}
		switch value.(type) {
		case []interface{}, map[string]interface{}:
			return "", fmt.Errorf("identifier path %q does not resolve to a scalar", identifierPath)
		}
		values = append(values, typedIdentityScalar{
			Type:  fmt.Sprintf("%T", value),
			Value: value,
		})
	}

	identity, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshalling identity: %w", err)
	}
	return collectionElementIdentityKey(identity), nil
}

// hasDuplicateCompleteElements reports whether either side contains identical
// complete objects, which a Multiset cannot safely pair without explicit identity.
func hasDuplicateCompleteElements(collections ...[]interface{}) bool {
	for _, collection := range collections {
		seen := make(map[string]struct{}, len(collection))
		for _, element := range collection {
			encoded, err := json.Marshal(element)
			if err != nil {
				continue
			}
			key := string(encoded)
			if _, ok := seen[key]; ok {
				return true
			}
			seen[key] = struct{}{}
		}
	}
	return false
}

// collectionAtJSONPointer returns the array at a JSON Pointer path.
func collectionAtJSONPointer(root interface{}, pointer string) ([]interface{}, error) {
	value, found, err := valueAtJSONPointer(root, pointer)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("path not found")
	}
	collection, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", value)
	}
	return collection, nil
}

// setCollectionAtJSONPointer replaces the array at a JSON Pointer path.
func setCollectionAtJSONPointer(root interface{}, pointer string, collection []interface{}) error {
	segments, err := jsonPointerSegments(pointer)
	if err != nil {
		return err
	}
	if len(segments) == 0 {
		return fmt.Errorf("root collection path is not supported")
	}

	current := root
	for _, segment := range segments[:len(segments)-1] {
		object, ok := current.(map[string]interface{})
		if !ok {
			return fmt.Errorf("segment %q traverses %T", segment, current)
		}
		next, ok := object[segment]
		if !ok {
			return fmt.Errorf("segment %q not found", segment)
		}
		current = next
	}

	object, ok := current.(map[string]interface{})
	if !ok {
		return fmt.Errorf("collection parent is %T", current)
	}
	last := segments[len(segments)-1]
	if _, ok := object[last]; !ok {
		return fmt.Errorf("segment %q not found", last)
	}
	object[last] = collection
	return nil
}

// valueAtJSONPointer resolves object-only JSON Pointer paths used by collection
// metadata and element identifiers.
func valueAtJSONPointer(root interface{}, pointer string) (interface{}, bool, error) {
	segments, err := jsonPointerSegments(pointer)
	if err != nil {
		return nil, false, err
	}

	current := root
	for _, segment := range segments {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("segment %q traverses %T", segment, current)
		}
		next, ok := object[segment]
		if !ok {
			return nil, false, nil
		}
		current = next
	}
	return current, true, nil
}

// jsonPointerSegments decodes RFC 6901 object path segments.
func jsonPointerSegments(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("JSON Pointer must start with /")
	}
	rawSegments := strings.Split(pointer[1:], "/")
	segments := make([]string, len(rawSegments))
	for index, segment := range rawSegments {
		segment = strings.ReplaceAll(segment, "~1", "/")
		segment = strings.ReplaceAll(segment, "~0", "~")
		segments[index] = segment
	}
	return segments, nil
}
