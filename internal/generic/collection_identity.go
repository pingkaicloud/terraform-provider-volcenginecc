package generic

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
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

// typedIdentityScalar preserves the schema-level scalar type in the canonical
// identity key. Value is a canonical string so equivalent numeric encodings do
// not produce different identities.
type typedIdentityScalar struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// canonicalIdentityScalar is one sortable component of a collection identity.
// Composite identities compare these components in elementIdentifier order.
type canonicalIdentityScalar struct {
	kind        string
	stringValue string
	numberValue *big.Rat
	boolValue   bool
}

// canonicalizeCollectionByIdentity sorts one unordered object collection by
// elementIdentifier. Missing, null, complex, or duplicate identities fail
// instead of falling back to the input array indexes. Strings use raw UTF-8
// byte order, numbers compare by exact rational value, and false sorts before
// true so Provider and service implementations can share one total order.
func canonicalizeCollectionByIdentity(collection []interface{}, identifierPaths []string) ([]interface{}, error) {
	type sortableElement struct {
		value    interface{}
		identity []canonicalIdentityScalar
	}

	sortable := make([]sortableElement, 0, len(collection))
	seen := make(map[collectionElementIdentityKey]struct{}, len(collection))
	for index, element := range collection {
		identity, err := collectionElementIdentityScalars(element, identifierPaths)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", index, err)
		}
		key, err := collectionIdentityKey(identity)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", index, err)
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate identity %s", key)
		}
		seen[key] = struct{}{}
		sortable = append(sortable, sortableElement{
			value:    element,
			identity: identity,
		})
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
		return nil, compareErr
	}

	result := make([]interface{}, len(sortable))
	for index, element := range sortable {
		result[index] = element.value
	}
	return result, nil
}

// canonicalizeIdentityCollections sorts current and planned desired-state
// collections independently by the same objective identity order before JSON
// Patch generation. The service must apply the same order to its actual patch
// base so snapshot permutations cannot change the meaning of an array index.
func canonicalizeIdentityCollections(current, planned string, identities []CollectionIdentity) (string, string, error) {
	currentRoot, err := decodeIdentityJSON(current)
	if err != nil {
		return "", "", fmt.Errorf("unmarshalling current desired state: %w", err)
	}
	plannedRoot, err := decodeIdentityJSON(planned)
	if err != nil {
		return "", "", fmt.Errorf("unmarshalling planned desired state: %w", err)
	}

	if err := canonicalizeIdentityCollectionsInValue(currentRoot, identities); err != nil {
		return "", "", fmt.Errorf("canonicalizing current desired state: %w", err)
	}
	if err := canonicalizeIdentityCollectionsInValue(plannedRoot, identities); err != nil {
		return "", "", fmt.Errorf("canonicalizing planned desired state: %w", err)
	}

	canonicalCurrent, err := json.Marshal(currentRoot)
	if err != nil {
		return "", "", fmt.Errorf("marshalling canonical current desired state: %w", err)
	}
	canonicalPlanned, err := json.Marshal(plannedRoot)
	if err != nil {
		return "", "", fmt.Errorf("marshalling canonical planned desired state: %w", err)
	}

	return string(canonicalCurrent), string(canonicalPlanned), nil
}

// canonicalizeIdentityDesiredState strictly sorts all identity collections in
// one desired-state document without falling back to input array indexes.
func canonicalizeIdentityDesiredState(state string, identities []CollectionIdentity) (string, error) {
	root, err := decodeIdentityJSON(state)
	if err != nil {
		return "", err
	}
	if err := canonicalizeIdentityCollectionsInValue(root, identities); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(canonical), nil
}

// canonicalizeIdentityCollectionsInValue applies canonical identity order to
// every present collection path in a decoded desired-state document.
func canonicalizeIdentityCollectionsInValue(root interface{}, identities []CollectionIdentity) error {
	ordered, err := collectionIdentitiesInnerFirst(identities)
	if err != nil {
		return err
	}
	for _, identity := range ordered {
		segments, err := jsonPointerSegments(identity.PropertyPath)
		if err != nil {
			return fmt.Errorf("parsing collection %q: %w", identity.PropertyPath, err)
		}
		if len(segments) == 0 {
			return fmt.Errorf("collection %q: root collection path is not supported", identity.PropertyPath)
		}
		_, err = canonicalizeJSONCollectionsAtPath(root, segments, identity.IdentifierPaths, identity.PropertyPath)
		if err != nil {
			return fmt.Errorf("canonicalizing collection %q: %w", identity.PropertyPath, err)
		}
	}
	return nil
}

// collectionIdentitiesInnerFirst returns a stable depth-descending copy so
// nested collections are canonicalized before their containing collections.
func collectionIdentitiesInnerFirst(identities []CollectionIdentity) ([]CollectionIdentity, error) {
	type identityWithDepth struct {
		identity CollectionIdentity
		depth    int
	}
	ordered := make([]identityWithDepth, 0, len(identities))
	for _, identity := range identities {
		segments, err := jsonPointerSegments(identity.PropertyPath)
		if err != nil {
			return nil, fmt.Errorf("parsing collection %q: %w", identity.PropertyPath, err)
		}
		ordered = append(ordered, identityWithDepth{identity: identity, depth: len(segments)})
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].depth > ordered[right].depth
	})
	result := make([]CollectionIdentity, len(ordered))
	for index, item := range ordered {
		result[index] = item.identity
	}
	return result, nil
}

// canonicalizeJSONCollectionsAtPath recursively resolves object attributes and
// wildcard array elements, then sorts every collection reached at the leaf.
func canonicalizeJSONCollectionsAtPath(
	current interface{},
	segments []string,
	identifierPaths []string,
	location string,
) (interface{}, error) {
	if current == nil {
		return current, nil
	}
	if len(segments) == 0 {
		collection, ok := current.([]interface{})
		if !ok {
			return current, fmt.Errorf("%s expected array, got %T", location, current)
		}
		canonical, err := canonicalizeCollectionByIdentity(collection, identifierPaths)
		if err != nil {
			return current, fmt.Errorf("%s: %w", location, err)
		}
		return canonical, nil
	}

	segment := segments[0]
	if segment == "*" {
		collection, ok := current.([]interface{})
		if !ok {
			return current, fmt.Errorf("%s wildcard traverses %T", location, current)
		}
		for index, element := range collection {
			canonical, err := canonicalizeJSONCollectionsAtPath(
				element,
				segments[1:],
				identifierPaths,
				fmt.Sprintf("%s[%d]", location, index),
			)
			if err != nil {
				return current, err
			}
			collection[index] = canonical
		}
		return collection, nil
	}

	object, ok := current.(map[string]interface{})
	if !ok {
		return current, fmt.Errorf("%s segment %q traverses %T", location, segment, current)
	}
	child, found := object[segment]
	if !found || child == nil {
		return current, nil
	}
	canonical, err := canonicalizeJSONCollectionsAtPath(child, segments[1:], identifierPaths, location+"/"+segment)
	if err != nil {
		return current, err
	}
	object[segment] = canonical
	return object, nil
}

// decodeIdentityJSON preserves JSON numbers as decimal strings so canonical
// numeric comparison does not lose precision through float64 conversion.
func decodeIdentityJSON(input string) (interface{}, error) {
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()
	var result interface{}
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
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
	values, err := collectionElementIdentityScalars(element, identifierPaths)
	if err != nil {
		return "", err
	}
	return collectionIdentityKey(values)
}

// collectionElementIdentityScalars extracts schema-level scalar identity
// values in elementIdentifier order for both matching and canonical sorting.
func collectionElementIdentityScalars(element interface{}, identifierPaths []string) ([]canonicalIdentityScalar, error) {
	values := make([]canonicalIdentityScalar, 0, len(identifierPaths))
	for _, identifierPath := range identifierPaths {
		value, found, err := valueAtJSONPointer(element, identifierPath)
		if err != nil {
			return nil, fmt.Errorf("identifier path %q: %w", identifierPath, err)
		}
		if !found {
			return nil, fmt.Errorf("identifier path %q is missing", identifierPath)
		}
		if value == nil {
			return nil, fmt.Errorf("identifier path %q is null", identifierPath)
		}
		if _, unknown := value.(unknownCollectionIdentityValue); unknown {
			return nil, fmt.Errorf("identifier path %q is unknown", identifierPath)
		}
		switch value.(type) {
		case []interface{}, map[string]interface{}:
			return nil, fmt.Errorf("identifier path %q does not resolve to a scalar", identifierPath)
		}
		scalar, err := canonicalIdentityScalarFromValue(value)
		if err != nil {
			return nil, fmt.Errorf("identifier path %q: %w", identifierPath, err)
		}
		values = append(values, scalar)
	}
	return values, nil
}

// collectionIdentityKey encodes a canonical, type-preserving identity key.
func collectionIdentityKey(values []canonicalIdentityScalar) (collectionElementIdentityKey, error) {
	encoded := make([]typedIdentityScalar, 0, len(values))
	for _, value := range values {
		encoded = append(encoded, typedIdentityScalar{
			Type:  value.kind,
			Value: canonicalIdentityScalarString(value),
		})
	}
	identity, err := json.Marshal(encoded)
	if err != nil {
		return "", fmt.Errorf("marshalling identity: %w", err)
	}
	return collectionElementIdentityKey(identity), nil
}

// canonicalIdentityScalarFromValue converts supported JSON scalar values into
// the cross-language canonical types string, number, and boolean.
func canonicalIdentityScalarFromValue(value interface{}) (canonicalIdentityScalar, error) {
	switch typed := value.(type) {
	case string:
		return canonicalIdentityScalar{kind: "string", stringValue: typed}, nil
	case bool:
		return canonicalIdentityScalar{kind: "boolean", boolValue: typed}, nil
	case json.Number:
		number, ok := new(big.Rat).SetString(string(typed))
		if !ok {
			return canonicalIdentityScalar{}, fmt.Errorf("invalid JSON number %q", typed)
		}
		return canonicalIdentityScalar{kind: "number", numberValue: number}, nil
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		number := new(big.Rat).SetInt64(reflected.Int())
		return canonicalIdentityScalar{kind: "number", numberValue: number}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		integer := new(big.Int).SetUint64(reflected.Uint())
		return canonicalIdentityScalar{kind: "number", numberValue: new(big.Rat).SetInt(integer)}, nil
	case reflect.Float32, reflect.Float64:
		number := reflected.Float()
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return canonicalIdentityScalar{}, fmt.Errorf("non-finite number is not supported")
		}
		encoded := strconv.FormatFloat(number, 'g', -1, reflected.Type().Bits())
		rational, ok := new(big.Rat).SetString(encoded)
		if !ok {
			return canonicalIdentityScalar{}, fmt.Errorf("invalid number %q", encoded)
		}
		return canonicalIdentityScalar{kind: "number", numberValue: rational}, nil
	default:
		return canonicalIdentityScalar{}, fmt.Errorf("unsupported scalar type %T", value)
	}
}

// canonicalIdentityScalarString returns the stable key representation of one
// identity scalar without defining its sort order through serialization.
func canonicalIdentityScalarString(value canonicalIdentityScalar) string {
	switch value.kind {
	case "string":
		return value.stringValue
	case "number":
		return value.numberValue.RatString()
	case "boolean":
		return strconv.FormatBool(value.boolValue)
	default:
		return ""
	}
}

// compareCanonicalIdentity lexicographically compares composite identities in
// elementIdentifier order and rejects cross-element schema type mismatches.
func compareCanonicalIdentity(left, right []canonicalIdentityScalar) (int, error) {
	if len(left) != len(right) {
		return 0, fmt.Errorf("identity component count differs: %d and %d", len(left), len(right))
	}
	for index := range left {
		if left[index].kind != right[index].kind {
			return 0, fmt.Errorf("identity component %d type differs: %s and %s", index, left[index].kind, right[index].kind)
		}
		var comparison int
		switch left[index].kind {
		case "string":
			comparison = strings.Compare(left[index].stringValue, right[index].stringValue)
		case "number":
			comparison = left[index].numberValue.Cmp(right[index].numberValue)
		case "boolean":
			if left[index].boolValue != right[index].boolValue {
				if left[index].boolValue {
					comparison = 1
				} else {
					comparison = -1
				}
			}
		default:
			return 0, fmt.Errorf("unsupported identity type %q", left[index].kind)
		}
		if comparison != 0 {
			return comparison, nil
		}
	}
	return 0, nil
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
