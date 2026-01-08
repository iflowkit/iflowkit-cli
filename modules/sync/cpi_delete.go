package sync

import (
	"context"

	"github.com/iflowkit/iflowkit-cli/internal/common/cpix"
)

// deleteArtifactInCPI deletes the artifact by kind using CPI OData delete endpoints.
// Version is always 'active' for versioned entity keys.
func deleteArtifactInCPI(ctx context.Context, client *cpix.Client, kind, id, csrfToken, cookieHeader string) error {
	switch kind {
	case "iFlows":
		// Requested endpoint (some tenants expose IntegrationAdapterDesigntimeArtifacts).
		err := client.DeleteArtifact(ctx, "IntegrationAdapterDesigntimeArtifacts", id, "", csrfToken, cookieHeader)
		if err == nil {
			return nil
		}
		// Fallback to the standard IntegrationDesigntimeArtifacts (Version='active').
		if cpix.IsNotFound(err) || cpix.IsBadRequest(err) {
			return client.DeleteArtifact(ctx, "IntegrationDesigntimeArtifacts", id, "active", csrfToken, cookieHeader)
		}
		return err
	case "ValueMappings":
		return client.DeleteArtifact(ctx, "ValueMappingDesigntimeArtifacts", id, "active", csrfToken, cookieHeader)
	case "MessageMappings":
		return client.DeleteArtifact(ctx, "MessageMappingDesigntimeArtifacts", id, "active", csrfToken, cookieHeader)
	case "Scripts":
		return client.DeleteArtifact(ctx, "ScriptCollectionDesigntimeArtifacts", id, "active", csrfToken, cookieHeader)
	default:
		// Not supported (e.g. CustomTags).
		return nil
	}
}
