package generic

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// restoreIdentityCollectionWriteOnlyValues restores write-only element fields
// by collection identity after Read has translated and aligned remote state.
func (r *genericResource) restoreIdentityCollectionWriteOnlyValues(prior, remote tftypes.Value) (tftypes.Value, error) {
	result := remote
	for _, identity := range r.collectionIdentities {
		attributeNames, err := r.collectionAttributeNames(identity.PropertyPath)
		if err != nil {
			return remote, err
		}
		if len(attributeNames) != 1 {
			continue
		}
		writeOnlyNames, err := r.identityCollectionWriteOnlyAttributeNames(identity.PropertyPath)
		if err != nil {
			return remote, err
		}
		if len(writeOnlyNames) == 0 {
			continue
		}

		attributePath := terraformAttributePath(attributeNames)
		priorCollection, err := terraformValueAtPath(prior, attributePath)
		if err != nil {
			return remote, fmt.Errorf("reading prior collection %q: %w", identity.PropertyPath, err)
		}
		remoteCollection, err := terraformValueAtPath(result, attributePath)
		if err != nil {
			return remote, fmt.Errorf("reading remote collection %q: %w", identity.PropertyPath, err)
		}
		identifierNames, err := r.identifierAttributeNames(identity.IdentifierPaths)
		if err != nil {
			return remote, err
		}
		restored, err := restoreTerraformCollectionWriteOnlyValues(priorCollection, remoteCollection, identifierNames, writeOnlyNames)
		if err != nil {
			return remote, fmt.Errorf("restoring write-only values for collection %q: %w", identity.PropertyPath, err)
		}
		result, err = replaceTerraformValueAtPath(result, attributePath, restored)
		if err != nil {
			return remote, fmt.Errorf("setting remote collection %q: %w", identity.PropertyPath, err)
		}
	}
	return result, nil
}

// writeOnlyPathBelongsToIdentityCollection reports whether a write-only
// property path points inside an identity-bearing collection element.
func (r *genericResource) writeOnlyPathBelongsToIdentityCollection(writeOnlyPath string) bool {
	for _, identity := range r.collectionIdentities {
		if _, ok := writeOnlyRelativePropertyPath(identity.PropertyPath, writeOnlyPath); ok {
			return true
		}
	}
	return false
}

// identityCollectionWriteOnlyAttributeNames returns element-relative Terraform
// attribute paths for write-only fields inside one identity collection.
func (r *genericResource) identityCollectionWriteOnlyAttributeNames(collectionPath string) ([][]string, error) {
	result := make([][]string, 0)
	for _, writeOnlyPath := range r.writeOnlyPropertyPaths {
		relativePath, ok := writeOnlyRelativePropertyPath(collectionPath, writeOnlyPath)
		if !ok {
			continue
		}
		names, err := r.identifierAttributeNames([]string{relativePath})
		if err != nil {
			return nil, err
		}
		result = append(result, names[0])
	}
	return result, nil
}

// writeOnlyRelativePropertyPath converts a Cloud Control write-only property
// path into an element-relative path when it is under the given collection.
func writeOnlyRelativePropertyPath(collectionPath string, writeOnlyPath string) (string, bool) {
	normalized := strings.TrimPrefix(writeOnlyPath, "/properties")
	prefix := collectionPath + "/*/"
	if !strings.HasPrefix(normalized, prefix) {
		return "", false
	}
	relative := strings.TrimPrefix(normalized, prefix)
	if relative == "" {
		return "", false
	}
	return "/" + relative, true
}

// restoreTerraformCollectionWriteOnlyValues copies write-only values from
// matched prior elements into remote elements without copying by array index,
// then restores canonical order because identity fields are unchanged.
func restoreTerraformCollectionWriteOnlyValues(prior, remote tftypes.Value, identifiers [][]string, writeOnlyPaths [][]string) (tftypes.Value, error) {
	if prior.IsNull() || !prior.IsKnown() || remote.IsNull() || !remote.IsKnown() {
		return remote, nil
	}
	priorElements, err := terraformCollectionElements(prior)
	if err != nil {
		return remote, err
	}
	remoteElements, err := terraformCollectionElements(remote)
	if err != nil {
		return remote, err
	}
	priorByIdentity, err := indexTerraformElements(priorElements, identifiers)
	if err != nil {
		return remote, err
	}
	if _, err := indexTerraformElements(remoteElements, identifiers); err != nil {
		return remote, err
	}

	restored := make([]tftypes.Value, 0, len(remoteElements))
	for _, remoteElement := range remoteElements {
		key, err := terraformElementIdentity(remoteElement, identifiers)
		if err != nil {
			return remote, err
		}
		priorElement, ok := priorByIdentity[key]
		if !ok {
			restored = append(restored, remoteElement)
			continue
		}
		element := remoteElement
		for _, writeOnlyPath := range writeOnlyPaths {
			value, err := terraformValueAtPath(priorElement, terraformAttributePath(writeOnlyPath))
			if err != nil {
				return remote, err
			}
			element, err = replaceTerraformValueAtPath(element, terraformAttributePath(writeOnlyPath), value)
			if err != nil {
				return remote, err
			}
		}
		restored = append(restored, element)
	}
	return canonicalizeTerraformCollectionState(tftypes.NewValue(remote.Type(), restored), identifiers)
}
