package api

import "fmt"

func assetThumbURL(assetID string) string {
	return fmt.Sprintf("/api/v1/assets/%s/thumb", assetID)
}

func assetFileURL(assetID string) string { //nolint:unused // readability
	return fmt.Sprintf("/api/v1/assets/%s/file", assetID)
}

func versionThumbURL(assetID, versionID string) string {
	return fmt.Sprintf("/api/v1/assets/%s/versions/%s/thumb", assetID, versionID)
}

func versionFileURL(assetID, versionID string) string { //nolint:unused // readability
	return fmt.Sprintf("/api/v1/assets/%s/versions/%s/file", assetID, versionID)
}

func variantThumbURL(assetID, variantID string) string {
	return fmt.Sprintf("/api/v1/assets/%s/variants/%s/thumb", assetID, variantID)
}

func variantFileURL(assetID, variantID string) string {
	return fmt.Sprintf("/api/v1/assets/%s/variants/%s/file", assetID, variantID)
}

func textTrackDownloadURL(assetID, trackID string) string {
	return fmt.Sprintf("/api/v1/assets/%s/text-tracks/%s/download", assetID, trackID)
}

func sharedAssetThumbURL(shareID, assetID string) string {
	return fmt.Sprintf("/shared/%s/assets/%s/thumb", shareID, assetID)
}

func sharedVariantThumbURL(shareID, assetID, variantID string) string {
	return fmt.Sprintf("/shared/%s/assets/%s/variants/%s/thumb", shareID, assetID, variantID)
}
