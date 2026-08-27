package audit

// IsBrowserPrefetch reports whether a Sec-Fetch-Dest value represents a
// passive browser fetch (image/video/document/iframe) that should not be
// recorded as an intentional download in the audit log.
func IsBrowserPrefetch(secFetchDest string) bool {
	switch secFetchDest {
	case "image", "video", "document", "iframe":
		return true
	}
	return false
}
