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

// alignCollectionByIdentity orders remote elements by matching their identities
// to prior elements. New remote elements are appended, while missing, duplicate,
// or incomplete identities return an error instead of falling back to indexes.
func alignCollectionByIdentity(prior, remote []interface{}, identifierPaths []string) ([]interface{}, error) {
	if len(identifierPaths) == 0 {
		return nil, fmt.Errorf("collection identity requires at least one identifier path")
	}

	priorByIdentity, priorOrder, err := indexCollectionByIdentity(prior, identifierPaths)
	if err != nil {
		return nil, fmt.Errorf("indexing prior collection: %w", err)
	}

	remoteByIdentity, remoteOrder, err := indexCollectionByIdentity(remote, identifierPaths)
	if err != nil {
		return nil, fmt.Errorf("indexing remote collection: %w", err)
	}

	aligned := make([]interface{}, 0, len(remote))
	used := make(map[string]struct{}, len(remote))

	for _, identity := range priorOrder {
		if _, existed := priorByIdentity[identity]; !existed {
			continue
		}
		if remoteElement, ok := remoteByIdentity[identity]; ok {
			aligned = append(aligned, remoteElement)
			used[identity] = struct{}{}
		}
	}

	for _, identity := range remoteOrder {
		if _, ok := used[identity]; ok {
			continue
		}
		aligned = append(aligned, remoteByIdentity[identity])
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
	currentByIdentity, currentOrder, err := indexCollectionByIdentity(current, identifierPaths)
	if err != nil {
		return nil, fmt.Errorf("indexing current collection: %w", err)
	}

	_, plannedOrder, err := indexCollectionByIdentity(planned, identifierPaths)
	if err != nil {
		return nil, fmt.Errorf("indexing planned collection: %w", err)
	}

	aligned := make([]interface{}, 0, len(current))
	used := make(map[string]struct{}, len(current))
	for _, identity := range plannedOrder {
		if currentElement, ok := currentByIdentity[identity]; ok {
			aligned = append(aligned, currentElement)
			used[identity] = struct{}{}
		}
	}
	for _, identity := range currentOrder {
		if _, ok := used[identity]; ok {
			continue
		}
		aligned = append(aligned, currentByIdentity[identity])
	}

	return aligned, nil
}

// indexCollectionByIdentity extracts stable identity keys and rejects duplicate
// or incomplete identities so callers never guess element correspondence.
func indexCollectionByIdentity(collection []interface{}, identifierPaths []string) (map[string]interface{}, []string, error) {
	byIdentity := make(map[string]interface{}, len(collection))
	order := make([]string, 0, len(collection))
	for index, element := range collection {
		identity, err := collectionElementIdentity(element, identifierPaths)
		if err != nil {
			return nil, nil, fmt.Errorf("element %d: %w", index, err)
		}
		if _, exists := byIdentity[identity]; exists {
			return nil, nil, fmt.Errorf("duplicate identity %s", identity)
		}
		byIdentity[identity] = element
		order = append(order, identity)
	}
	return byIdentity, order, nil
}

// collectionElementIdentity builds a deterministic JSON key from all configured
// identifier paths. Null, missing, collection, and object values are rejected
// because they cannot safely identify an element in this prototype.
func collectionElementIdentity(element interface{}, identifierPaths []string) (string, error) {
	values := make([]interface{}, 0, len(identifierPaths))
	for _, identifierPath := range identifierPaths {
		value, found, err := valueAtJSONPointer(element, identifierPath)
		if err != nil {
			return "", fmt.Errorf("identifier path %q: %w", identifierPath, err)
		}
		if !found || value == nil {
			return "", fmt.Errorf("identifier path %q is missing or null", identifierPath)
		}
		switch value.(type) {
		case []interface{}, map[string]interface{}:
			return "", fmt.Errorf("identifier path %q does not resolve to a scalar", identifierPath)
		}
		values = append(values, value)
	}

	identity, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshalling identity: %w", err)
	}
	return string(identity), nil
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
