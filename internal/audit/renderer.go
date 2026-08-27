package audit

import (
	"encoding/json"
	"fmt"
)

// RenderHumanReadable produces a short English string for a given event, suitable
// for display in a timeline. The actor name is not included — the caller renders
// the actor separately. Falls back to a generic string on unknown types or
// unparseable payloads.
func RenderHumanReadable(eventType string, rawPayload string) string {
	payload := json.RawMessage(rawPayload)
	if fn, ok := eventRenderers[eventType]; ok {
		if s := fn(payload); s != "" {
			return s
		}
	}
	return fmt.Sprintf("performed %s", eventType)
}

var eventRenderers = map[string]func(json.RawMessage) string{
	EventAssetCreated:                 renderAssetCreated,
	EventAssetRenamed:                 renderAssetRenamed,
	EventAssetMoved:                   renderAssetMoved,
	EventAssetTagged:                  renderAssetTagged,
	EventAssetUntagged:                renderAssetUntagged,
	EventAssetFieldSet:                renderAssetFieldSet,
	EventAssetFieldCleared:            renderAssetFieldCleared,
	EventAssetVersionUploaded:         renderAssetVersionUploaded,
	EventAssetVersionRestored:         renderAssetVersionRestored,
	EventAssetVersionDeleted:          renderAssetVersionDeleted,
	EventAssetShared:                  renderAssetShared,
	EventAssetShareRevoked:            func(_ json.RawMessage) string { return "revoked share link" },
	EventAssetDeleted:                 func(_ json.RawMessage) string { return "deleted" },
	EventAssetDownloaded:              renderAssetDownloaded,
	EventAssetVariantCreated:          renderAssetVariantCreated,
	EventAssetVariantDownloaded:       renderAssetVariantDownloaded,
	EventAssetVariantDeleted:          renderAssetVariantDeleted,
	EventAssetVariantPromoted:         renderAssetVariantPromoted,
	EventAssetThumbnailSetFromVariant: func(_ json.RawMessage) string { return "set asset thumbnail from variant" },
	EventAssetVariantRerun:            func(_ json.RawMessage) string { return "re-ran variant" },
	EventProjectCreated:               renderProjectCreated,
	EventProjectRenamed:               renderProjectRenamed,
	EventProjectFieldSet:              renderProjectFieldSet,
	EventProjectFieldCleared:          renderProjectFieldCleared,
	EventProjectDeleted:               func(_ json.RawMessage) string { return "deleted project" },
}

func renderAssetCreated(payload json.RawMessage) string {
	var p AssetCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	if p.Source == "ingress" && p.SourceID != "" {
		return fmt.Sprintf("imported from ingress source %q", p.SourceID)
	}
	if p.Source == "derived" && p.SourceID != "" {
		return fmt.Sprintf("derived from asset %q", p.SourceID)
	}
	return "uploaded"
}

func renderAssetRenamed(payload json.RawMessage) string {
	var p AssetRenamedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("renamed from %q to %q", p.Before, p.After)
}

func renderAssetMoved(payload json.RawMessage) string {
	var p AssetMovedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	if p.AfterProjectID != nil {
		return fmt.Sprintf("moved to project %q", *p.AfterProjectID)
	}
	return "moved"
}

func renderAssetTagged(payload json.RawMessage) string {
	var p AssetTaggedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("added tag %q", p.Tag)
}

func renderAssetUntagged(payload json.RawMessage) string {
	var p AssetUntaggedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("removed tag %q", p.Tag)
}

func renderAssetFieldSet(payload json.RawMessage) string {
	var p AssetFieldSetPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("set %s to %v", p.FieldName, p.After)
}

func renderAssetFieldCleared(payload json.RawMessage) string {
	var p AssetFieldClearedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("cleared %s", p.FieldName)
}

func renderAssetVersionUploaded(payload json.RawMessage) string {
	var p AssetVersionUploadedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	if p.Comment != "" {
		return fmt.Sprintf("uploaded version %d — %q", p.VersionNum, p.Comment)
	}
	return fmt.Sprintf("uploaded version %d", p.VersionNum)
}

func renderAssetVersionRestored(payload json.RawMessage) string {
	var p AssetVersionRestoredPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("restored version %d (rolled back from v%d)", p.ToVersionNum, p.FromVersionNum)
}

func renderAssetVersionDeleted(payload json.RawMessage) string {
	var p AssetVersionDeletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("deleted version %d", p.VersionNum)
}

func renderAssetShared(payload json.RawMessage) string {
	var p AssetSharedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	if p.ExpiresAt != nil {
		return fmt.Sprintf("created share link (expires %s)", *p.ExpiresAt)
	}
	return "created share link"
}

func renderAssetDownloaded(payload json.RawMessage) string {
	var p AssetDownloadedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	if p.Via == "share" {
		return "downloaded via share link"
	}
	return "downloaded"
}

func renderAssetVariantCreated(payload json.RawMessage) string {
	var p AssetVariantCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("requested %s variant", p.Type)
}

func renderAssetVariantDownloaded(payload json.RawMessage) string {
	var p AssetVariantDownloadedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("downloaded %s variant", p.Type)
}

func renderAssetVariantDeleted(payload json.RawMessage) string {
	var p AssetVariantDeletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("deleted %s variant", p.Type)
}

func renderAssetVariantPromoted(payload json.RawMessage) string {
	var p AssetVariantPromotedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("promoted variant to asset %s", p.NewAssetID)
}

func renderProjectCreated(payload json.RawMessage) string {
	var p ProjectCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("created project %q", p.Name)
}

func renderProjectRenamed(payload json.RawMessage) string {
	var p ProjectRenamedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("renamed project from %q to %q", p.Before, p.After)
}

func renderProjectFieldSet(payload json.RawMessage) string {
	var p ProjectFieldSetPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("set %s to %v", p.FieldName, p.After)
}

func renderProjectFieldCleared(payload json.RawMessage) string {
	var p ProjectFieldClearedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return fmt.Sprintf("cleared %s", p.FieldName)
}
