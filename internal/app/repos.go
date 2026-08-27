package app

import (
	"damask/server/internal/db"
	"damask/server/internal/repository"
	reposqlc "damask/server/internal/repository/sqlc"
)

// repos groups every repository constructed from database. Splitting this
// out of Build keeps Build itself focused on wiring services together.
type repos struct {
	workspace       repository.WorkspaceRepository
	asset           repository.AssetRepository
	tag             repository.TagRepository
	field           repository.FieldRepository
	user            repository.UserRepository
	version         repository.VersionRepository
	variant         repository.VariantRepository
	assetField      repository.AssetFieldRepository
	workflow        repository.WorkflowRepository
	workflowRun     repository.WorkflowRunRepository
	workflowWebhook repository.WorkflowWebhookRepository
	project         repository.ProjectRepository
	folder          repository.FolderRepository
	collection      repository.CollectionRepository
	share           repository.ShareRepository
	projectField    repository.ProjectFieldRepository
	embedToken      repository.EmbedTokenRepository
	ingress         repository.IngressRepository
	textTrack       repository.TextTrackRepository
	auditLog        repository.AuditLogRepository
	autoTagSug      repository.AutoTagSuggestionRepository
	storageStats    repository.StorageStatsRepository
}

func buildRepos(database *db.DB) repos {
	return repos{
		workspace:       reposqlc.NewWorkspaceRepo(database),
		asset:           reposqlc.NewAssetRepo(database),
		tag:             reposqlc.NewTagRepo(database),
		field:           reposqlc.NewFieldRepo(database),
		user:            reposqlc.NewUserRepo(database),
		version:         reposqlc.NewVersionRepo(database),
		variant:         reposqlc.NewVariantRepo(database),
		assetField:      reposqlc.NewAssetFieldRepo(database),
		workflow:        reposqlc.NewWorkflowRepo(database),
		workflowRun:     reposqlc.NewWorkflowRunRepo(database),
		workflowWebhook: reposqlc.NewWorkflowWebhookRepo(database),
		project:         reposqlc.NewProjectRepo(database),
		folder:          reposqlc.NewFolderRepo(database),
		collection:      reposqlc.NewCollectionRepo(database),
		share:           reposqlc.NewShareRepo(database),
		projectField:    reposqlc.NewProjectFieldRepo(database),
		embedToken:      reposqlc.NewEmbedTokenRepo(database),
		ingress:         reposqlc.NewIngressRepo(database),
		textTrack:       reposqlc.NewTextTrackRepo(database),
		auditLog:        reposqlc.NewAuditLogRepo(database),
		autoTagSug:      reposqlc.NewAutoTagSuggestionRepo(database),
		storageStats:    reposqlc.NewStorageStatsRepo(database),
	}
}
