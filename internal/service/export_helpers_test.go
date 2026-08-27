package service

// White-box exports for unit testing unexported helpers.
// This file is compiled only during `go test`.

var (
	CSVEscape            = csvEscape
	PaginateAuditEvents  = paginateAuditEvents
	ClampLimit           = clampLimit
	MakeTypesFilter      = makeTypesFilter
	NilIfEmpty           = nilIfEmpty
	RemoveAuthMethodStr  = removeAuthMethodStr
	HasOnlyMethodStr     = hasOnlyMethodStr
	MethodName           = methodName
	ExtractJobType       = extractJobType
	ReadyTextContent     = readyTextContent
	StringValue          = stringValue
	StringParam          = stringParam
	IsShareExpiredDomain = isShareExpiredDomain
	ToPublicAssetDTO     = toPublicAssetDTO
	ToCommentDTO         = toCommentDTO
	StampWorkspaceID     = stampWorkspaceID
	ExportConfigToDTO    = exportConfigToDTO
	ExportRunToDTO       = exportRunToDTO
)
