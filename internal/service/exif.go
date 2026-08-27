package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"damask/server/internal/apperr"
	"damask/server/internal/media/contentmeta"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
)

type exifFieldDef struct {
	key       string
	name      string
	fieldType string
	gpsOnly   bool
}

const (
	exifSource            = "exif"
	customFieldTypeNumber = "number"
	customFieldTypeText   = "text"
	customFieldTypeDate   = "date"
	startExifPosition     = 1000
	exifKeyMake           = "_exif_make"
)

var exifFields = []exifFieldDef{
	{exifKeyMake, "Camera maker", customFieldTypeText, false},
	{"_exif_model", "Camera model", customFieldTypeText, false},
	{"_exif_lens", "Lens", customFieldTypeText, false},
	{"_exif_software", "Software", customFieldTypeText, false},
	{"_exif_exposure_time", "Shutter speed", customFieldTypeText, false},
	{"_exif_f_number", "Aperture", customFieldTypeNumber, false},
	{"_exif_iso", "ISO", customFieldTypeNumber, false},
	{"_exif_focal_length", "Focal length (mm)", customFieldTypeNumber, false},
	{"_exif_focal_length_35", "Focal length 35mm equiv.", customFieldTypeNumber, false},
	{"_exif_flash", "Flash", customFieldTypeText, false},
	{"_exif_white_balance", "White balance", customFieldTypeText, false},
	{"_exif_taken_at", "Date taken", customFieldTypeDate, false},
	{"_exif_gps_lat", "GPS latitude", customFieldTypeNumber, true},
	{"_exif_gps_lng", "GPS longitude", customFieldTypeNumber, true},
}

// ExifService extracts EXIF metadata from image assets and stores it as field values.
type ExifService struct {
	workspaces repository.WorkspaceRepository
	assets     repository.AssetRepository
	fields     repository.FieldRepository
	assetField repository.AssetFieldRepository
	storage    storage.Storage
}

func NewExifService(
	workspaces repository.WorkspaceRepository,
	assets repository.AssetRepository,
	fields repository.FieldRepository,
	assetField repository.AssetFieldRepository,
	stor storage.Storage,
) *ExifService {
	return &ExifService{workspaces: workspaces, assets: assets, fields: fields, assetField: assetField, storage: stor}
}

func (s *ExifService) ExtractForAsset(ctx context.Context, workspaceID, assetID, userID string) error {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load workspace: %w", err)
	}
	if !ws.ExifKeep {
		return nil
	}

	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("load asset: %w", err)
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return nil
	}

	keepGPS := ws.ExifKeepGps

	// Tombstone check before ensureFields to avoid N inserts on already-processed assets.
	if processed, tombErr := s.isExifTombstoned(ctx, workspaceID, assetID); tombErr != nil {
		return tombErr
	} else if processed {
		return nil
	}

	fieldIDs, err := s.ensureFields(ctx, workspaceID, userID, keepGPS)
	if err != nil {
		return fmt.Errorf("ensure exif fields: %w", err)
	}

	makeFieldID, ok := fieldIDs[exifKeyMake]
	if !ok {
		return errors.New("_exif_make field not found after ensureFields")
	}

	r, err := s.storage.Get(ctx, asset.StorageKey)
	if err != nil {
		return fmt.Errorf("open asset: %w", err)
	}
	defer r.Close()

	result, err := contentmeta.ExtractImageEXIF(ctx, r, keepGPS)
	if err != nil {
		slog.WarnContext(ctx, "exif: extract error — writing tombstone", "asset_id", assetID, "error", err)
	}

	if result == nil {
		empty := ""
		if uErr := s.assetField.UpsertValue(ctx, assetID, repository.SetFieldValueParams{
			FieldID:   makeFieldID,
			ValueText: &empty,
			CreatedBy: userID,
		}); uErr != nil {
			return fmt.Errorf("write tombstone: %w", uErr)
		}
		slog.DebugContext(ctx, "exif: no data — tombstone written", "asset_id", assetID)
		return nil
	}

	if err = s.upsertExifFields(ctx, assetID, userID, fieldIDs, result, keepGPS); err != nil {
		return err
	}

	slog.DebugContext(ctx, "exif: extracted",
		"asset_id", assetID,
		"make", ptrStr(result.Make),
		"model", ptrStr(result.Model),
		"gps", result.GPS != nil,
	)
	return nil
}

func (s *ExifService) isExifTombstoned(ctx context.Context, workspaceID, assetID string) (bool, error) {
	makeField, err := s.fields.GetByKey(ctx, workspaceID, exifKeyMake)
	if err != nil {
		return false, nil //nolint:nilerr // field doesn't exist yet
	}
	values, tombErr := s.assetField.GetValues(ctx, assetID)
	if tombErr != nil {
		return false, fmt.Errorf("check tombstone: %w", tombErr)
	}
	for _, v := range values {
		if v.FieldID == makeField.ID {
			return true, nil
		}
	}
	return false, nil
}

func (s *ExifService) ensureFields(
	ctx context.Context,
	workspaceID, _ string,
	keepGPS bool,
) (map[string]string, error) {
	for i, fd := range exifFields {
		if fd.gpsOnly && !keepGPS {
			continue
		}
		if err := s.fields.EnsureSystemField(ctx, repository.EnsureSystemFieldParams{
			ID:          uuid.NewString(),
			WorkspaceID: workspaceID,
			Source:      exifSource,
			Name:        fd.name,
			Key:         fd.key,
			FieldType:   fd.fieldType,
			Position:    int64(startExifPosition + i),
		}); err != nil {
			return nil, fmt.Errorf("ensure exif field %s: %w", fd.key, err)
		}
	}

	fields, err := s.fields.ListBySource(ctx, workspaceID, exifSource)
	if err != nil {
		return nil, fmt.Errorf("load exif fields: %w", err)
	}

	fieldIDs := make(map[string]string, len(fields))
	for _, field := range fields {
		fieldIDs[field.Key] = field.ID
	}
	return fieldIDs, nil
}

func (s *ExifService) upsertExifFields(
	ctx context.Context,
	assetID, userID string,
	fieldIDs map[string]string,
	result *contentmeta.ImageEXIF,
	keepGPS bool,
) error {
	type textField struct {
		key string
		val *string
	}
	type numField struct {
		key string
		val *float64
	}

	for _, f := range []textField{
		{exifKeyMake, result.Make},
		{"_exif_model", result.Model},
		{"_exif_lens", result.LensModel},
		{"_exif_software", result.Software},
		{"_exif_exposure_time", result.ExposureTime},
		{"_exif_flash", result.Flash},
		{"_exif_white_balance", result.WhiteBalance},
	} {
		fid, ok := fieldIDs[f.key]
		if !ok {
			continue
		}
		if uErr := s.assetField.UpsertValue(ctx, assetID, repository.SetFieldValueParams{
			FieldID:   fid,
			ValueText: f.val,
			CreatedBy: userID,
		}); uErr != nil {
			return fmt.Errorf("upsert %s: %w", f.key, uErr)
		}
	}

	var isoF *float64
	if result.ISO != nil {
		v := float64(*result.ISO)
		isoF = &v
	}
	nums := []numField{
		{"_exif_f_number", result.FNumber},
		{"_exif_iso", isoF},
		{"_exif_focal_length", result.FocalLength},
		{"_exif_focal_length_35", result.FocalLength35},
	}
	if keepGPS && result.GPS != nil {
		lat, lng := result.GPS.Lat, result.GPS.Lng
		nums = append(nums, numField{"_exif_gps_lat", &lat}, numField{"_exif_gps_lng", &lng})
	}
	for _, f := range nums {
		fid, ok := fieldIDs[f.key]
		if !ok {
			continue
		}
		if uErr := s.assetField.UpsertValue(ctx, assetID, repository.SetFieldValueParams{
			FieldID:     fid,
			ValueNumber: f.val,
			CreatedBy:   userID,
		}); uErr != nil {
			return fmt.Errorf("upsert %s: %w", f.key, uErr)
		}
	}

	if result.TakenAt != nil {
		if fid, ok := fieldIDs["_exif_taken_at"]; ok {
			v := result.TakenAt.Format("2006-01-02")
			if uErr := s.assetField.UpsertValue(ctx, assetID, repository.SetFieldValueParams{
				FieldID:   fid,
				ValueDate: &v,
				CreatedBy: userID,
			}); uErr != nil {
				return fmt.Errorf("upsert _exif_taken_at: %w", uErr)
			}
		}
	}
	return nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
