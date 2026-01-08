package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/common/cpix"
	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/models"
)

// applyTransportToTenant executes delete/upload/deploy steps against CPI using the transport record as retry state.
//
// It mutates and persists the record while it makes progress.
func applyTransportToTenant(ctx *app.Context, repoRoot string, meta models.SyncMetadata, tenantEnv string, rec *TransportRecord, store *TransportStore) (deleted int, updated int, deployed int, retErr error) {
	if rec == nil {
		return 0, 0, 0, fmt.Errorf("transport record is nil")
	}

	profileID, source, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return 0, 0, 0, err
	}
	_, err = ctx.Stores.Profiles.Read(profileID)
	if err != nil {
		return 0, 0, 0, err
	}
	ctx.Logger.Info("resolved profile", logging.F("profile", profileID), logging.F("source", source))

	tenantKey, err := ctx.Stores.Tenants.Read(profileID, tenantEnv)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("%s tenant not found for profile %q; import it with `iflowkit tenant import --env %s --file <service-key.json>`: %w", tenantDisplay(tenantEnv), profileID, tenantEnv, err)
	}

	client := cpix.NewClient(tenantKey, ctx.Logger)
	csrf, cookies, err := client.FetchCSRFToken(context.Background())
	if err != nil {
		return 0, 0, 0, err
	}

	// 1) Delete first.
	orderedDelete := append([]artifactKey{}, rec.DeleteRemaining...)
	sort.Slice(orderedDelete, func(i, j int) bool {
		if orderedDelete[i].Kind == orderedDelete[j].Kind {
			return orderedDelete[i].ID < orderedDelete[j].ID
		}
		return orderedDelete[i].Kind < orderedDelete[j].Kind
	})

	for _, k := range orderedDelete {
		ctx.Logger.Info("deleting artifact from CPI", logging.F("kind", k.Kind), logging.F("id", k.ID), logging.F("version", "active"))
		if err := deleteArtifactInCPI(context.Background(), client, k.Kind, k.ID, csrf, cookies); err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(*rec)
			return deleted, updated, deployed, err
		}
		deleted++
		rec.DeleteRemaining = removeUpload(rec.DeleteRemaining, k)
		_, _ = store.PersistTransportRecord(*rec)
	}

	// 2) Upload by kind.
	byKind := make(map[string][]string)
	for _, k := range rec.UploadRemaining {
		byKind[k.Kind] = append(byKind[k.Kind], k.ID)
	}
	for kind := range byKind {
		sort.Strings(byKind[kind])
	}

	artsByKind := make(map[string]map[string]cpix.ArtifactInfo)
	for kind := range byKind {
		endpoint, ok := listEndpointForKind(meta.PackageID, kind)
		if !ok {
			ctx.Logger.Warn("unknown artifact kind; skipping", logging.F("kind", kind))
			continue
		}
		m, err := client.ListArtifacts(context.Background(), endpoint)
		if err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(*rec)
			return deleted, updated, deployed, err
		}
		artsByKind[kind] = m
	}

	orderedUpload := append([]artifactKey{}, rec.UploadRemaining...)
	sort.Slice(orderedUpload, func(i, j int) bool {
		if orderedUpload[i].Kind == orderedUpload[j].Kind {
			return orderedUpload[i].ID < orderedUpload[j].ID
		}
		return orderedUpload[i].Kind < orderedUpload[j].Kind
	})

	for _, k := range orderedUpload {
		arts := artsByKind[k.Kind]
		art, ok := arts[k.ID]
		if !ok {
			ctx.Logger.Warn("artifact not found in CPI list; skipping", logging.F("kind", k.Kind), logging.F("id", k.ID))
			rec.UploadRemaining = removeUpload(rec.UploadRemaining, k)
			_, _ = store.PersistTransportRecord(*rec)
			continue
		}

		artifactDir := filepath.Join(repoRoot, meta.BaseFolder, k.Kind, k.ID)
		st, err := os.Stat(artifactDir)
		if err != nil || !st.IsDir() {
			ctx.Logger.Warn("artifact directory missing; skipping", logging.F("dir", artifactDir), logging.F("kind", k.Kind), logging.F("id", k.ID))
			rec.UploadRemaining = removeUpload(rec.UploadRemaining, k)
			_, _ = store.PersistTransportRecord(*rec)
			continue
		}

		zipBytes, err := filex.ZipDirToBytes(artifactDir)
		if err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(*rec)
			return deleted, updated, deployed, err
		}

		entitySet := kindToEntitySet(k.Kind)
		if entitySet == "" {
			ctx.Logger.Warn("artifact kind is not supported for CPI updates; skipping", logging.F("kind", k.Kind), logging.F("id", k.ID))
			rec.UploadRemaining = removeUpload(rec.UploadRemaining, k)
			_, _ = store.PersistTransportRecord(*rec)
			continue
		}

		ctx.Logger.Info("uploading artifact to CPI", logging.F("kind", k.Kind), logging.F("id", k.ID))
		if err := client.UpdateArtifact(context.Background(), entitySet, art, zipBytes, csrf, cookies); err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(*rec)
			return deleted, updated, deployed, err
		}
		updated++
		rec.UploadRemaining = removeUpload(rec.UploadRemaining, k)

		// Track deploy requirement.
		switch k.Kind {
		case "iFlows", "Scripts", "ValueMappings", "MessageMappings":
			rec.DeployRemaining = mergeDeployRemaining(rec.DeployRemaining, []deployTarget{{Kind: k.Kind, ID: k.ID}})
		}
		_, _ = store.PersistTransportRecord(*rec)
	}

	// 3) Deploy.
	orderedDeploy := append([]deployTarget{}, rec.DeployRemaining...)
	sort.Slice(orderedDeploy, func(i, j int) bool {
		if orderedDeploy[i].Kind == orderedDeploy[j].Kind {
			return orderedDeploy[i].ID < orderedDeploy[j].ID
		}
		return orderedDeploy[i].Kind < orderedDeploy[j].Kind
	})

	for _, d := range orderedDeploy {
		ctx.Logger.Info("deploying artifact", logging.F("kind", d.Kind), logging.F("id", d.ID), logging.F("version", "active"))
		var derr error
		switch d.Kind {
		case "iFlows":
			derr = client.DeployIntegrationDesigntimeArtifact(context.Background(), d.ID, "active", csrf, cookies)
		case "Scripts":
			derr = client.DeployScriptCollectionDesigntimeArtifact(context.Background(), d.ID, "active", csrf, cookies)
		case "ValueMappings":
			derr = client.DeployValueMappingDesigntimeArtifact(context.Background(), d.ID, "active", csrf, cookies)
		case "MessageMappings":
			derr = client.DeployMessageMappingDesigntimeArtifact(context.Background(), d.ID, "active", csrf, cookies)
		default:
			ctx.Logger.Warn("deploy kind not supported; skipping", logging.F("kind", d.Kind), logging.F("id", d.ID))
			rec.DeployRemaining = removeDeployTarget(rec.DeployRemaining, d)
			_, _ = store.PersistTransportRecord(*rec)
			continue
		}
		if derr != nil {
			rec.TransportStatus = "pending"
			rec.Error = derr.Error()
			_, _ = store.PersistTransportRecord(*rec)
			return deleted, updated, deployed, derr
		}
		deployed++
		rec.DeployRemaining = removeDeployTarget(rec.DeployRemaining, d)
		_, _ = store.PersistTransportRecord(*rec)
		ctx.Logger.Info("artifact deployed", logging.F("kind", d.Kind), logging.F("id", d.ID), logging.F("version", "active"))
	}

	rec.TransportStatus = "completed"
	rec.Error = ""
	_, _ = store.PersistTransportRecord(*rec)
	return deleted, updated, deployed, nil
}
