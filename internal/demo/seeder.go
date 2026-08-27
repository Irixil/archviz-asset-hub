//go:build demo

// Package demo implements the demo workspace seeder, wiper, and reset scheduler.
// All demo functionality is gated behind DEMO_MODE=true in config.
package demo

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"damask/server/internal/config"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// bcryptHashOfDemo is a pre-computed bcrypt hash of the password "demo"
// at cost 10. Never bcrypt at runtime during seed — it's slow.
const bcryptHashOfDemo = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Seeder creates and populates the demo workspace.
type Seeder struct {
	db          *sql.DB
	storage     storage.Storage
	trf         transform.Transformer
	tmb         transform.Thumbnailer
	cfg         config.DemoConfig
	lastResetAt time.Time // set after each successful reset; zero on first boot
}

// New returns a Seeder ready to use.
func New(db *sql.DB, stor storage.Storage, cfg config.DemoConfig, trf transform.Transformer, tmb transform.Thumbnailer) *Seeder {
	return &Seeder{db: db, storage: stor, cfg: cfg, trf: trf, tmb: tmb}
}

// ids holds the stable IDs created during seeding so later steps can reference them.
type ids struct {
	workspaceID string
	userID      string
	aliceID     string
	marcID      string

	// projects
	brandProjectID   string
	summerProjectID  string
	websiteProjectID string
	archiveProjectID string

	// folders (brand)
	logosFolder  string
	colorsFolder string

	// folders (summer)
	photoFolder  string
	videoFolder  string
	socialFolder string

	// folders (website)
	wireframesFolder string
	uiCompFolder     string
	exportsFolder    string

	// folders (archive)
	printReadyFolder string

	// field definitions (asset scope)
	fieldClient       string
	fieldStatus       string
	fieldUsageRights  string
	fieldPhotographer string
	fieldLicensed     string

	// field definitions (project scope)
	pfieldClient string
	pfieldBudget string
	pfieldPhase  string

	// key assets (for versioning and events)
	assetHomepageV2  string
	assetLogoPrimary string
	assetBeachHero   string
	assetStudioHero  string

	// share
	shareID string

	// all asset ids (for event seeding)
	allAssets []assetMeta
}

type assetMeta struct {
	id        string
	name      string
	projectID string
}

// SeedIfEmpty seeds the demo workspace only if it has no projects yet.
// Used on startup to handle first boot and crash recovery.
func (s *Seeder) SeedIfEmpty(ctx context.Context) error {
	var workspaceID string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`).Scan(&workspaceID)
	if err != nil {
		return nil // workspace doesn't exist yet
	}

	var count int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE workspace_id = ?`, workspaceID).Scan(&count)
	if count > 0 {
		return nil // already seeded
	}

	return s.Seed(ctx)
}

// Seed creates the demo workspace content from scratch.
// Call this only after Wipe() or on first boot.
// The workspace and user rows must already exist (created by EnsureWorkspace).
func (s *Seeder) Seed(ctx context.Context) error {
	slog.InfoContext(ctx, "demo: seed started")
	start := time.Now()

	var d ids

	// Load the stable workspace and user IDs
	row := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`)
	if err := row.Scan(&d.workspaceID); err != nil {
		return fmt.Errorf("demo: find demo workspace: %w", err)
	}
	row = s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ? LIMIT 1`, s.cfg.UserEmail)
	if err := row.Scan(&d.userID); err != nil {
		return fmt.Errorf("demo: find demo user: %w", err)
	}

	// Create ghost users for the activity log (they may already exist from a previous seed)
	base := strings.TrimPrefix(s.cfg.UserEmail, "demo") // e.g. "@example.com"
	aliceID, err := s.ensureGhostUser(ctx, "alice"+base, "Alice")
	if err != nil {
		return fmt.Errorf("demo: ghost user alice: %w", err)
	}
	d.aliceID = aliceID

	marcID, err := s.ensureGhostUser(ctx, "marc"+base, "Marc")
	if err != nil {
		return fmt.Errorf("demo: ghost user marc: %w", err)
	}
	d.marcID = marcID

	// Seed all content inside a transaction where possible
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("demo: begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := s.seedFieldDefinitions(ctx, tx, &d); err != nil {
		return err
	}
	if err := s.seedProjects(ctx, tx, &d); err != nil {
		return err
	}
	if err := s.seedFolders(ctx, tx, &d); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("demo: commit structure tx: %w", err)
	}

	// Asset creation writes to storage outside the transaction
	if err := s.seedAssets(ctx, &d); err != nil {
		return err
	}

	// Second transaction for everything that references assets
	tx2, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("demo: begin tx2: %w", err)
	}
	defer tx2.Rollback()

	if err := s.seedTags(ctx, tx2, &d); err != nil {
		return err
	}
	if err := s.seedFieldValues(ctx, tx2, &d); err != nil {
		return err
	}
	if err := s.seedShare(ctx, tx2, &d); err != nil {
		return err
	}
	if err := s.seedIngressSource(ctx, tx2, &d); err != nil {
		return err
	}

	if err := tx2.Commit(); err != nil {
		return fmt.Errorf("demo: commit data tx: %w", err)
	}

	// Events are best-effort; written outside any transaction
	if err := s.seedEvents(ctx, &d); err != nil {
		slog.WarnContext(ctx, "demo: seed events (non-fatal)", "error", err)
	}

	slog.InfoContext(ctx, "demo: seed complete", "assets_created", len(d.allAssets), "duration_ms", time.Since(start).Milliseconds())
	return nil
}

// EnsureWorkspace creates the workspace and user rows if they don't exist.
// These rows are kept stable across resets (not wiped), so this is idempotent.
func (s *Seeder) EnsureWorkspace(ctx context.Context) error {
	// Check if already exists
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`).Scan(&existing)
	if err == nil {
		return nil // already exists
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("demo: check workspace: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("demo: begin workspace tx: %w", err)
	}
	defer tx.Rollback()

	workspaceID := "demo_ws_" + uuid.NewString()
	userID := "demo_usr_" + uuid.NewString()
	ingestToken := uuid.NewString()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO workspaces (id, name, ingest_token, is_demo, created_at, updated_at)
		VALUES (?, ?, ?, 1, datetime('now'), datetime('now'))
	`, workspaceID, s.cfg.WorkspaceName, ingestToken)
	if err != nil {
		return fmt.Errorf("demo: create workspace: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES (?, ?, ?, 'Demo User', datetime('now'), datetime('now'))
	`, userID, s.cfg.UserEmail, bcryptHashOfDemo)
	if err != nil {
		return fmt.Errorf("demo: create user: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role, created_at)
		VALUES (?, ?, 'editor', datetime('now'))
	`, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("demo: create member: %w", err)
	}

	return tx.Commit()
}

// GetWorkspaceID returns the demo workspace ID, or ("", false) if not found.
func (s *Seeder) GetWorkspaceID(ctx context.Context) (string, bool) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`).Scan(&id)
	if err != nil {
		return "", false
	}
	return id, true
}

// GetDemoUser returns the (userID, workspaceID) for the demo session, or an error
// if the demo workspace does not exist (mid-reset).
func (s *Seeder) GetDemoUser(ctx context.Context) (userID, workspaceID string, err error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, w.id
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		JOIN users u ON u.id = wm.user_id
		WHERE w.is_demo = 1 AND u.email = ?
		LIMIT 1
	`, s.cfg.UserEmail)
	err = row.Scan(&userID, &workspaceID)
	return
}

// VerifyDemoPassword checks that the provided password matches the demo user.
// Returns false (not an error) if the password is wrong.
func VerifyDemoPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(bcryptHashOfDemo), []byte(password))
	return err == nil
}

// ResetInterval returns the configured reset interval.
func (s *Seeder) ResetInterval() time.Duration {
	return time.Duration(s.cfg.ResetIntervalHours) * time.Hour
}

// LastResetAt returns the time of the last successful reset (zero if none yet).
func (s *Seeder) LastResetAt() time.Time {
	return s.lastResetAt
}

// NextResetAt returns the estimated time of the next reset.
// Returns zero if the reset interval is not configured.
func (s *Seeder) NextResetAt() time.Time {
	if s.lastResetAt.IsZero() || s.cfg.ResetIntervalHours == 0 {
		return time.Time{}
	}
	return s.lastResetAt.Add(s.ResetInterval())
}

// GetUsage returns the current asset count and total storage bytes used by the demo workspace.
func (s *Seeder) GetUsage(ctx context.Context, workspaceID string) (assetCount, storageUsed int64, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT a.id),
		       COALESCE(SUM(av.size), 0)
		FROM assets a
		LEFT JOIN asset_versions av ON av.id = a.current_version_id
		WHERE a.workspace_id = ?
	`, workspaceID).Scan(&assetCount, &storageUsed)
	return
}

// --- field definitions ---

func (s *Seeder) seedFieldDefinitions(ctx context.Context, tx *sql.Tx, d *ids) error {
	type fieldDef struct {
		id       string
		scope    string
		name     string
		key      string
		ftype    string
		options  *string
		position int
	}

	opts := func(o string) *string { return &o }

	assetFields := []fieldDef{
		{newID("fd"), "asset", "Client", "client", "select", opts(`["Sportswear Co","Internal","Archived"]`), 0},
		{newID("fd"), "asset", "Status", "status", "select", opts(`["Draft","In Review","Approved","Rejected"]`), 1},
		{newID("fd"), "asset", "Usage Rights Expiry", "usage_rights", "date", nil, 2},
		{newID("fd"), "asset", "Photographer", "photographer", "text", nil, 3},
		{newID("fd"), "asset", "Licensed", "licensed", "boolean", nil, 4},
	}
	projFields := []fieldDef{
		{newID("fd"), "project", "Client", "client", "text", nil, 0},
		{newID("fd"), "project", "Budget (€)", "budget", "number", nil, 1},
		{newID("fd"), "project", "Phase", "phase", "select", opts(`["Discovery","Production","Delivery","Archived"]`), 2},
	}

	insertFD := func(f fieldDef) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO field_definitions
			  (id, workspace_id, created_by, scope, name, key, field_type, options, required, position, inherit_from_project, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, 0, datetime('now'), datetime('now'))
		`, f.id, d.workspaceID, d.userID, f.scope, f.name, f.key, f.ftype, f.options, f.position)
		return err
	}

	for i, f := range assetFields {
		if err := insertFD(f); err != nil {
			return fmt.Errorf("demo: asset field def %d: %w", i, err)
		}
		switch f.key {
		case "client":
			d.fieldClient = f.id
		case "status":
			d.fieldStatus = f.id
		case "usage_rights":
			d.fieldUsageRights = f.id
		case "photographer":
			d.fieldPhotographer = f.id
		case "licensed":
			d.fieldLicensed = f.id
		}
	}
	for i, f := range projFields {
		if err := insertFD(f); err != nil {
			return fmt.Errorf("demo: project field def %d: %w", i, err)
		}
		switch f.key {
		case "client":
			d.pfieldClient = f.id
		case "budget":
			d.pfieldBudget = f.id
		case "phase":
			d.pfieldPhase = f.id
		}
	}
	return nil
}

// --- projects ---

func (s *Seeder) seedProjects(ctx context.Context, tx *sql.Tx, d *ids) error {
	type proj struct {
		id    string
		name  string
		desc  string
		color string
	}
	projects := []proj{
		{newID("proj"), "Brand Identity", "Sportswear client brand system", "#6366f1"},
		{newID("proj"), "Summer Campaign", "Photography and video for the summer launch", "#f59e0b"},
		{newID("proj"), "Website Redesign", "Full redesign of the marketing site", "#10b981"},
		{newID("proj"), "Q2 Campaign Archive", "Archived Q2 assets", "#6b7280"},
	}

	for i, p := range projects {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO projects (id, workspace_id, name, description, color, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, p.id, d.workspaceID, p.name, p.desc, p.color)
		if err != nil {
			return fmt.Errorf("demo: project %d: %w", i, err)
		}
	}

	d.brandProjectID = projects[0].id
	d.summerProjectID = projects[1].id
	d.websiteProjectID = projects[2].id
	d.archiveProjectID = projects[3].id
	return nil
}

// --- folders ---

func (s *Seeder) seedFolders(ctx context.Context, tx *sql.Tx, d *ids) error {
	type folder struct {
		id        string
		projectID string
		name      string
	}
	folders := []folder{
		// brand
		{newID("fld"), d.brandProjectID, "Logos"},
		{newID("fld"), d.brandProjectID, "Colors & Typography"},
		// summer
		{newID("fld"), d.summerProjectID, "Photography"},
		{newID("fld"), d.summerProjectID, "Video"},
		{newID("fld"), d.summerProjectID, "Social"},
		// website
		{newID("fld"), d.websiteProjectID, "Wireframes"},
		{newID("fld"), d.websiteProjectID, "UI Components"},
		{newID("fld"), d.websiteProjectID, "Exports"},
		// archive
		{newID("fld"), d.archiveProjectID, "Print Ready"},
	}

	for i, f := range folders {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO folders (id, workspace_id, project_id, name, position, created_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'))
		`, f.id, d.workspaceID, f.projectID, f.name, i)
		if err != nil {
			return fmt.Errorf("demo: folder %q: %w", f.name, err)
		}
	}

	d.logosFolder = folders[0].id
	d.colorsFolder = folders[1].id
	d.photoFolder = folders[2].id
	d.videoFolder = folders[3].id
	d.socialFolder = folders[4].id
	d.wireframesFolder = folders[5].id
	d.uiCompFolder = folders[6].id
	d.exportsFolder = folders[7].id
	d.printReadyFolder = folders[8].id
	return nil
}

// --- assets ---

type assetSpec struct {
	name      string
	projectID string
	folderID  string
	mime      string
	w, h      int
	// used as a unique label in generated images
	label string
	// bg colour hue for generated images (used to vary versions visually)
	bgColor color.RGBA
	// mark as needing version history
	makeVersions int
}

func (s *Seeder) seedAssets(ctx context.Context, d *ids) error {
	rng := rand.New(rand.NewSource(42))

	specs := s.buildAssetSpecs(d)

	for i := range specs {
		sp := &specs[i]
		assetID := newID("ast")
		ext := transform.MimeToExt(sp.mime)

		data, width, height, err := s.generateFile(sp, 0, rng)
		if err != nil {
			return fmt.Errorf("demo: generate %s: %w", sp.name, err)
		}

		storageKey := fmt.Sprintf("demo/%s/%s/%s%s", d.workspaceID, assetID, assetID, ext)
		if err := s.storage.Put(ctx, storageKey, bytes.NewReader(data)); err != nil {
			return fmt.Errorf("demo: store %s: %w", sp.name, err)
		}

		contentHash := md5hex(data)

		var widthPtr, heightPtr *int64
		if width > 0 {
			w64 := int64(width)
			widthPtr = &w64
		}
		if height > 0 {
			h64 := int64(height)
			heightPtr = &h64
		}

		// Backdate assets over 14 days for realism
		createdAt := backdateRand(rng, 14)

		_, err = s.db.ExecContext(ctx, `
			INSERT INTO assets
			  (id, workspace_id, project_id, folder_id, original_filename, storage_key,
			   mime_type, size, width, height, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, assetID, d.workspaceID, sp.projectID, nullStr(sp.folderID),
			sp.name, storageKey, sp.mime, len(data),
			widthPtr, heightPtr,
			createdAt.Format("2006-01-02T15:04:05Z"),
			createdAt.Format("2006-01-02T15:04:05Z"))
		if err != nil {
			return fmt.Errorf("demo: insert asset %s: %w", sp.name, err)
		}

		// Create initial version row
		versionID := newID("ver")
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO asset_versions
			  (id, asset_id, workspace_id, version_num, storage_key, content_hash,
			   mime_type, size, width, height, created_by, created_at, is_current)
			VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		`, versionID, assetID, d.workspaceID,
			storageKey, contentHash,
			sp.mime, len(data), widthPtr, heightPtr,
			d.userID, createdAt.Format("2006-01-02T15:04:05Z"))
		if err != nil {
			return fmt.Errorf("demo: insert version for %s: %w", sp.name, err)
		}

		// Link current_version_id on the asset
		_, err = s.db.ExecContext(ctx, `UPDATE assets SET current_version_id = ? WHERE id = ?`, versionID, assetID)
		if err != nil {
			return fmt.Errorf("demo: link version for %s: %w", sp.name, err)
		}

		// Generate thumbnail synchronously
		thumbData, thumbExt, tErr := s.tmb.GenerateThumbnailData(ctx, s.storage, sp.mime, storageKey)
		if tErr == nil && thumbData != nil {
			thumbKey := fmt.Sprintf("demo/%s/%s/versions/%s/thumb%s", d.workspaceID, assetID, versionID, thumbExt)
			if putErr := s.storage.Put(ctx, thumbKey, bytes.NewReader(thumbData)); putErr == nil {
				s.db.ExecContext(ctx, `UPDATE asset_versions SET thumbnail_key = ? WHERE id = ?`, thumbKey, versionID)
				s.db.ExecContext(ctx, `UPDATE assets SET thumbnail_key = ? WHERE id = ?`, thumbKey, assetID)
			} else {
				slog.WarnContext(ctx, "demo: store thumbnail failed", "name", sp.name, "error", putErr)
			}
		} else {
			slog.WarnContext(ctx, "demo: thumbnail generation failed", "name", sp.name, "error", tErr)
		}

		d.allAssets = append(d.allAssets, assetMeta{id: assetID, name: sp.name, projectID: sp.projectID})

		// Capture key asset IDs by name
		switch sp.name {
		case "homepage-v2.png":
			d.assetHomepageV2 = assetID
		case "logo-primary-light.png":
			d.assetLogoPrimary = assetID
		case "hero-shot-beach.jpg":
			d.assetBeachHero = assetID
		case "hero-shot-studio.jpg":
			d.assetStudioHero = assetID
		}

		// Create additional versions where requested
		if sp.makeVersions > 1 {
			for v := 2; v <= sp.makeVersions; v++ {
				if err := s.addVersion(ctx, d, assetID, sp, v, rng); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (s *Seeder) addVersion(ctx context.Context, d *ids, assetID string, sp *assetSpec, versionNum int, rng *rand.Rand) error {
	data, width, height, err := s.generateFile(sp, versionNum-1, rng)
	if err != nil {
		return fmt.Errorf("demo: generate version %d of %s: %w", versionNum, sp.name, err)
	}

	storageKey := fmt.Sprintf("demo/%s/%s_v%d", d.workspaceID, assetID, versionNum)
	if err := s.storage.Put(ctx, storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("demo: store version %d of %s: %w", versionNum, sp.name, err)
	}

	contentHash := md5hex(data)
	versionID := newID("ver")

	comments := versionComments(sp.name, versionNum)
	var commentPtr *string
	if comments != "" {
		commentPtr = &comments
	}

	var widthPtr, heightPtr *int64
	if width > 0 {
		w64 := int64(width)
		widthPtr = &w64
	}
	if height > 0 {
		h64 := int64(height)
		heightPtr = &h64
	}

	createdAt := backdateRand(rng, 10)

	// Mark previous current versions as non-current
	_, err = s.db.ExecContext(ctx, `UPDATE asset_versions SET is_current = 0 WHERE asset_id = ?`, assetID)
	if err != nil {
		return fmt.Errorf("demo: unset current versions: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO asset_versions
		  (id, asset_id, workspace_id, version_num, storage_key, content_hash,
		   mime_type, size, width, height, comment, created_by, created_at, is_current)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	`, versionID, assetID, d.workspaceID,
		versionNum, storageKey, contentHash,
		sp.mime, len(data), widthPtr, heightPtr,
		commentPtr, d.userID, createdAt.Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return fmt.Errorf("demo: insert version %d of %s: %w", versionNum, sp.name, err)
	}

	// Update asset's current_version_id
	_, err = s.db.ExecContext(ctx, `UPDATE assets SET current_version_id = ? WHERE id = ?`, versionID, assetID)
	if err != nil {
		return err
	}

	// Generate thumbnail synchronously
	thumbData, thumbExt, tErr := s.tmb.GenerateThumbnailData(ctx, s.storage, sp.mime, storageKey)
	if tErr == nil && thumbData != nil {
		thumbKey := fmt.Sprintf("demo/%s/%s/versions/%s/thumb%s", d.workspaceID, assetID, versionID, thumbExt)
		if putErr := s.storage.Put(ctx, thumbKey, bytes.NewReader(thumbData)); putErr == nil {
			s.db.ExecContext(ctx, `UPDATE asset_versions SET thumbnail_key = ? WHERE id = ?`, thumbKey, versionID)
			s.db.ExecContext(ctx, `UPDATE assets SET thumbnail_key = ? WHERE id = ?`, thumbKey, assetID)
		}
	}

	return nil
}


func versionComments(filename string, versionNum int) string {
	comments := map[string][]string{
		"homepage-v2.png": {
			"",
			"Initial wireframe",
			"Revised after client feedback — moved nav to top",
		},
		"logo-primary-light.png": {
			"",
			"First draft",
			"Rounded corners per feedback",
			"Final — approved by client",
		},
		"hero-shot-beach.jpg": {
			"",
			"Raw edit",
			"Colour graded final",
		},
	}
	if c, ok := comments[filename]; ok && versionNum < len(c) {
		return c[versionNum]
	}
	return ""
}

// --- tags ---

func (s *Seeder) seedTags(ctx context.Context, tx *sql.Tx, d *ids) error {
	tagNames := []string{"approved", "hero", "social", "print", "web", "draft", "archive", "photography", "video", "brand"}
	tagIDs := map[string]string{}

	for _, name := range tagNames {
		id := newID("tag")
		_, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO tags (id, workspace_id, name) VALUES (?, ?, ?)
		`, id, d.workspaceID, name)
		if err != nil {
			return fmt.Errorf("demo: tag %q: %w", name, err)
		}
		// Fetch actual ID (may have been inserted by a concurrent seed)
		row := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE workspace_id = ? AND name = ?`, d.workspaceID, name)
		var actualID string
		if err := row.Scan(&actualID); err != nil {
			return fmt.Errorf("demo: fetch tag id %q: %w", name, err)
		}
		tagIDs[name] = actualID
	}

	addTag := func(assetID, tagName string) error {
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) VALUES (?, ?)`,
			assetID, tagIDs[tagName])
		return err
	}

	for _, am := range d.allAssets {
		switch am.projectID {
		case d.brandProjectID:
			if err := addTag(am.id, "brand"); err != nil {
				return err
			}
			if err := addTag(am.id, "approved"); err != nil {
				return err
			}
		case d.summerProjectID:
			if strings.HasSuffix(am.name, ".mp4") {
				if err := addTag(am.id, "video"); err != nil {
					return err
				}
			}
			if strings.HasPrefix(am.name, "instagram") || strings.HasPrefix(am.name, "twitter") || strings.HasPrefix(am.name, "facebook") {
				if err := addTag(am.id, "social"); err != nil {
					return err
				}
			}
			if strings.HasPrefix(am.name, "hero") {
				if err := addTag(am.id, "hero"); err != nil {
					return err
				}
				if err := addTag(am.id, "approved"); err != nil {
					return err
				}
			}
			if strings.HasPrefix(am.name, "lifestyle") {
				if err := addTag(am.id, "photography"); err != nil {
					return err
				}
				if err := addTag(am.id, "approved"); err != nil {
					return err
				}
			}
		case d.archiveProjectID:
			if err := addTag(am.id, "archive"); err != nil {
				return err
			}
			if err := addTag(am.id, "approved"); err != nil {
				return err
			}
		}
	}

	return nil
}

// --- field values ---

func (s *Seeder) seedFieldValues(ctx context.Context, tx *sql.Tx, d *ids) error {
	strVal := func(v string) *string { return &v }
	boolVal := func(v int64) *int64 { return &v }

	upsert := func(assetID, fieldID string, text *string, num *float64, date *string, boolean *int64) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO asset_field_values (id, asset_id, field_id, value_text, value_number, value_date, value_boolean, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(asset_id, field_id) DO UPDATE SET
			  value_text    = excluded.value_text,
			  value_number  = excluded.value_number,
			  value_date    = excluded.value_date,
			  value_boolean = excluded.value_boolean,
			  updated_at    = datetime('now')
		`, newID("afv"), assetID, fieldID, text, num, date, boolean, d.userID)
		return err
	}

	sixMonths := time.Now().AddDate(0, 6, 0).Format("2006-01-02")
	pastDate := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")

	for _, am := range d.allAssets {
		switch am.projectID {
		case d.brandProjectID:
			if err := upsert(am.id, d.fieldClient, strVal("Sportswear Co"), nil, nil, nil); err != nil {
				return err
			}
			if err := upsert(am.id, d.fieldStatus, strVal("Approved"), nil, nil, nil); err != nil {
				return err
			}
			if err := upsert(am.id, d.fieldLicensed, nil, nil, nil, boolVal(1)); err != nil {
				return err
			}
		case d.summerProjectID:
			if strings.HasPrefix(am.name, "hero") || strings.HasPrefix(am.name, "lifestyle") {
				if err := upsert(am.id, d.fieldPhotographer, strVal("Sarah M."), nil, nil, nil); err != nil {
					return err
				}
				if err := upsert(am.id, d.fieldUsageRights, nil, nil, &sixMonths, nil); err != nil {
					return err
				}
				if err := upsert(am.id, d.fieldStatus, strVal("Approved"), nil, nil, nil); err != nil {
					return err
				}
			}
		case d.websiteProjectID:
			if am.folderIDForLookup(d.wireframesFolder) {
				if err := upsert(am.id, d.fieldStatus, strVal("In Review"), nil, nil, nil); err != nil {
					return err
				}
			}
		case d.archiveProjectID:
			if err := upsert(am.id, d.fieldStatus, strVal("Approved"), nil, nil, nil); err != nil {
				return err
			}
			if err := upsert(am.id, d.fieldUsageRights, nil, nil, &pastDate, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// folderIDForLookup is a helper on assetMeta to check if an asset is in a given folder.
// Since assetMeta doesn't store folderID directly, we check by name heuristic.
func (am assetMeta) folderIDForLookup(folderID string) bool {
	// Wireframes: homepage and product-page
	return strings.HasPrefix(am.name, "homepage") || am.name == "product-page.png"
}

// --- share ---

func (s *Seeder) seedShare(ctx context.Context, tx *sql.Tx, d *ids) error {
	shareID := newID("sh")
	d.shareID = shareID
	expires := time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02T15:04:05Z")

	_, err := tx.ExecContext(ctx, `
		INSERT INTO shares (id, workspace_id, created_by, label, target_type, target_id,
		                    expires_at, allow_comments, allow_download, created_at)
		VALUES (?, ?, ?, 'Summer Campaign — Client Delivery', 'project', ?,
		        ?, 1, 1, datetime('now'))
	`, shareID, d.workspaceID, d.userID, d.summerProjectID, expires)
	if err != nil {
		return fmt.Errorf("demo: create share: %w", err)
	}

	// Find a hero asset to attach comments to
	heroAssetID := d.assetBeachHero
	if heroAssetID == "" {
		heroAssetID = d.assetStudioHero
	}
	if heroAssetID == "" && len(d.allAssets) > 0 {
		heroAssetID = d.allAssets[0].id
	}

	comments := []struct{ name, body string }{
		{"John (Sportswear Co)", "Love the beach shot! Can we get a version without the logo?"},
		{"John (Sportswear Co)", "The studio shot is perfect, approved."},
	}
	for _, c := range comments {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO share_comments (id, share_id, asset_id, author_name, body, created_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'))
		`, newID("sc"), shareID, heroAssetID, c.name, c.body)
		if err != nil {
			return fmt.Errorf("demo: share comment: %w", err)
		}
	}

	return nil
}

// --- ingress source ---

func (s *Seeder) seedIngressSource(ctx context.Context, tx *sql.Tx, d *ids) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ingress_sources
		  (id, workspace_id, created_by, type, label, config, public_token,
		   enabled, poll_interval_min, created_at, updated_at)
		VALUES (?, ?, ?, 'email_api', 'Inbound Email (Demo)', '', ?,
		        0, 60, datetime('now'), datetime('now'))
	`, newID("is"), d.workspaceID, d.userID, uuid.NewString())
	return err
}

// --- events ---

func (s *Seeder) seedEvents(ctx context.Context, d *ids) error {
	if len(d.allAssets) == 0 {
		return nil
	}

	rng := rand.New(rand.NewSource(99))
	actors := []struct {
		id   string
		kind string
	}{
		{d.userID, "user"},
		{d.aliceID, "user"},
		{d.marcID, "user"},
	}

	eventCount := 0
	insert := func(workspaceID, assetID, userID, actorType, eventType, payload string, t time.Time) error {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO asset_events (id, workspace_id, asset_id, user_id, actor_type, event_type, payload, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, newID("ev"), workspaceID, assetID, userID, actorType, eventType, payload, t.UTC().Format("2006-01-02T15:04:05Z"))
		if err == nil {
			eventCount++
		}
		return err
	}

	// asset_created events — one per asset, spread over 14 days
	for _, am := range d.allAssets {
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 14)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_created",
			`{"source":"upload"}`, t); err != nil {
			return err
		}
	}

	// asset_tagged events
	for i := 0; i < 12; i++ {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 12)
		tags := []string{"approved", "hero", "social", "brand", "photography"}
		tag := tags[rng.Intn(len(tags))]
		payload := fmt.Sprintf(`{"tag":%q}`, tag)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_tagged", payload, t); err != nil {
			return err
		}
	}

	// asset_renamed events
	for i := 0; i < 5; i++ {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 10)
		payload := fmt.Sprintf(`{"old_name":%q,"new_name":%q}`, am.name, am.name)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_renamed", payload, t); err != nil {
			return err
		}
	}

	// asset_version_uploaded events for versioned assets
	for _, assetID := range []string{d.assetHomepageV2, d.assetLogoPrimary, d.assetBeachHero} {
		if assetID == "" {
			continue
		}
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 7)
		if err := insert(d.workspaceID, assetID, actor.id, actor.kind, "asset_version_uploaded",
			`{"version_num":2}`, t); err != nil {
			return err
		}
	}

	// asset_field_set events
	for i := 0; i < 10; i++ {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 8)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_field_set",
			`{"field":"status","value":"Approved"}`, t); err != nil {
			return err
		}
	}

	// asset_moved events
	for i := 0; i < 4; i++ {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 9)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_moved",
			`{"from_folder":null,"to_folder":"Logos"}`, t); err != nil {
			return err
		}
	}

	// asset_shared event
	if len(d.allAssets) > 0 {
		am := d.allAssets[0]
		t := businessTime(rng, 5)
		if err := insert(d.workspaceID, am.id, d.userID, "user", "asset_shared",
			fmt.Sprintf(`{"share_id":%q}`, d.shareID), t); err != nil {
			return err
		}
	}

	// asset_deleted + asset_restored pairs
	for i := 0; i < 4; i++ {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t1 := businessTime(rng, 6)
		t2 := t1.Add(2 * time.Hour)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_deleted", `{}`, t1); err != nil {
			return err
		}
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_restored", `{}`, t2); err != nil {
			return err
		}
	}

	slog.InfoContext(ctx, "demo: seed complete events", "events_created", eventCount)
	return nil
}

// --- helpers ---

func (s *Seeder) ensureGhostUser(ctx context.Context, email, name string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ? LIMIT 1`, email).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("demo: check ghost user %s: %w", email, err)
	}

	id = newID("ghost")
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES (?, ?, '', ?, datetime('now'), datetime('now'))
	`, id, email, name)
	if err != nil {
		return "", fmt.Errorf("demo: insert ghost user %s: %w", email, err)
	}
	return id, nil
}

func newID(prefix string) string {
	return prefix + "_" + uuid.NewString()
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func md5hex(data []byte) string {
	h := md5.Sum(data)
	return hex.EncodeToString(h[:])
}

// backdateRand returns a random time within the last n days.
func backdateRand(rng *rand.Rand, days int) time.Time {
	offset := time.Duration(rng.Intn(days*24)) * time.Hour
	return time.Now().Add(-offset)
}

// businessTime returns a random weekday business-hours timestamp within the last n days.
func businessTime(rng *rand.Rand, days int) time.Time {
	for {
		t := backdateRand(rng, days)
		if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
			continue
		}
		hour := 8 + rng.Intn(10) // 8am–5pm
		return time.Date(t.Year(), t.Month(), t.Day(), hour, rng.Intn(60), 0, 0, time.UTC)
	}
}

// --- file generation ---

func (s *Seeder) buildAssetSpecs(d *ids) []assetSpec {
	return []assetSpec{
		// Brand — Logos
		{name: "logo-primary-light.png", projectID: d.brandProjectID, folderID: d.logosFolder, mime: "image/png", w: 512, h: 512, bgColor: color.RGBA{R: 255, G: 255, B: 255, A: 255}, makeVersions: 3},
		{name: "logo-primary-dark.png", projectID: d.brandProjectID, folderID: d.logosFolder, mime: "image/png", w: 512, h: 512, bgColor: color.RGBA{R: 30, G: 30, B: 30, A: 255}},
		{name: "logo-mark-only.svg", projectID: d.brandProjectID, folderID: d.logosFolder, mime: "image/svg+xml"},
		{name: "logo-wordmark.png", projectID: d.brandProjectID, folderID: d.logosFolder, mime: "image/png", w: 800, h: 200, bgColor: color.RGBA{R: 240, G: 240, B: 255, A: 255}},
		// Brand — Colors & Typography
		{name: "brand-guidelines-v3.pdf", projectID: d.brandProjectID, folderID: d.colorsFolder, mime: "application/pdf"},
		{name: "font-specimen.png", projectID: d.brandProjectID, folderID: d.colorsFolder, mime: "image/png", w: 1200, h: 800, bgColor: color.RGBA{R: 250, G: 248, B: 240, A: 255}},
		// Brand — root
		{name: "brand-overview-deck.pdf", projectID: d.brandProjectID, mime: "application/pdf"},
		{name: "mood-board-final.jpg", projectID: d.brandProjectID, mime: "image/jpeg", w: 1920, h: 1080, bgColor: color.RGBA{R: 200, G: 180, B: 160, A: 255}},

		// Summer — Photography
		{name: "hero-shot-beach.jpg", projectID: d.summerProjectID, folderID: d.photoFolder, mime: "image/jpeg", w: 1920, h: 1080, bgColor: color.RGBA{R: 135, G: 206, B: 235, A: 255}, makeVersions: 2},
		{name: "hero-shot-studio.jpg", projectID: d.summerProjectID, folderID: d.photoFolder, mime: "image/jpeg", w: 1920, h: 1080, bgColor: color.RGBA{R: 240, G: 240, B: 240, A: 255}},
		{name: "lifestyle-01.jpg", projectID: d.summerProjectID, folderID: d.photoFolder, mime: "image/jpeg", w: 1200, h: 800, bgColor: color.RGBA{R: 255, G: 220, B: 180, A: 255}},
		{name: "lifestyle-02.jpg", projectID: d.summerProjectID, folderID: d.photoFolder, mime: "image/jpeg", w: 1200, h: 800, bgColor: color.RGBA{R: 180, G: 220, B: 200, A: 255}},
		{name: "lifestyle-03.jpg", projectID: d.summerProjectID, folderID: d.photoFolder, mime: "image/jpeg", w: 1200, h: 800, bgColor: color.RGBA{R: 255, G: 200, B: 210, A: 255}},
		// Summer — Video
		{name: "teaser-15s.mp4", projectID: d.summerProjectID, folderID: d.videoFolder, mime: "video/mp4"},
		{name: "behind-the-scenes.mp4", projectID: d.summerProjectID, folderID: d.videoFolder, mime: "video/mp4"},
		// Summer — Social
		{name: "instagram-square.jpg", projectID: d.summerProjectID, folderID: d.socialFolder, mime: "image/jpeg", w: 1080, h: 1080, bgColor: color.RGBA{R: 255, G: 180, B: 100, A: 255}},
		{name: "instagram-story.jpg", projectID: d.summerProjectID, folderID: d.socialFolder, mime: "image/jpeg", w: 1080, h: 1920, bgColor: color.RGBA{R: 100, G: 180, B: 255, A: 255}},
		{name: "twitter-banner.jpg", projectID: d.summerProjectID, folderID: d.socialFolder, mime: "image/jpeg", w: 1500, h: 500, bgColor: color.RGBA{R: 29, G: 161, B: 242, A: 255}},
		{name: "facebook-cover.jpg", projectID: d.summerProjectID, folderID: d.socialFolder, mime: "image/jpeg", w: 820, h: 312, bgColor: color.RGBA{R: 66, G: 103, B: 178, A: 255}},

		// Website — Wireframes
		{name: "homepage-v1.png", projectID: d.websiteProjectID, folderID: d.wireframesFolder, mime: "image/png", w: 1440, h: 900, bgColor: color.RGBA{R: 240, G: 240, B: 240, A: 255}},
		{name: "homepage-v2.png", projectID: d.websiteProjectID, folderID: d.wireframesFolder, mime: "image/png", w: 1440, h: 900, bgColor: color.RGBA{R: 230, G: 230, B: 250, A: 255}, makeVersions: 2},
		{name: "product-page.png", projectID: d.websiteProjectID, folderID: d.wireframesFolder, mime: "image/png", w: 1440, h: 900, bgColor: color.RGBA{R: 240, G: 248, B: 255, A: 255}},
		// Website — UI Components
		{name: "button-states.png", projectID: d.websiteProjectID, folderID: d.uiCompFolder, mime: "image/png", w: 800, h: 400, bgColor: color.RGBA{R: 250, G: 250, B: 250, A: 255}},
		{name: "form-elements.png", projectID: d.websiteProjectID, folderID: d.uiCompFolder, mime: "image/png", w: 800, h: 600, bgColor: color.RGBA{R: 245, G: 245, B: 245, A: 255}},
		{name: "navigation-states.png", projectID: d.websiteProjectID, folderID: d.uiCompFolder, mime: "image/png", w: 1200, h: 300, bgColor: color.RGBA{R: 30, G: 30, B: 30, A: 255}},
		// Website — Exports
		{name: "hero-banner-2x.png", projectID: d.websiteProjectID, folderID: d.exportsFolder, mime: "image/png", w: 2880, h: 1200, bgColor: color.RGBA{R: 99, G: 102, B: 241, A: 255}},
		{name: "hero-banner-1x.png", projectID: d.websiteProjectID, folderID: d.exportsFolder, mime: "image/png", w: 1440, h: 600, bgColor: color.RGBA{R: 99, G: 102, B: 241, A: 255}},
		{name: "favicon-set.zip", projectID: d.websiteProjectID, folderID: d.exportsFolder, mime: "application/zip"},

		// Archive
		{name: "final-assets.zip", projectID: d.archiveProjectID, mime: "application/zip"},
		{name: "campaign-report.pdf", projectID: d.archiveProjectID, mime: "application/pdf"},
		{name: "poster-a1.pdf", projectID: d.archiveProjectID, folderID: d.printReadyFolder, mime: "application/pdf"},
		{name: "flyer-dl.pdf", projectID: d.archiveProjectID, folderID: d.printReadyFolder, mime: "application/pdf"},
	}
}

// generateFile produces a valid file for the given spec.
// versionOffset shifts the background colour slightly so versions look different.
func (s *Seeder) generateFile(sp *assetSpec, versionOffset int, rng *rand.Rand) (data []byte, width, height int, err error) {
	switch sp.mime {
	case "image/png":
		bg := shiftColor(sp.bgColor, versionOffset)
		data = generatePNG(sp.w, sp.h, bg, sp.name)
		return data, sp.w, sp.h, nil

	case "image/jpeg":
		bg := shiftColor(sp.bgColor, versionOffset)
		data, err = generateJPEG(sp.w, sp.h, bg, sp.name)
		return data, sp.w, sp.h, err

	case "image/svg+xml":
		data = generateSVG(sp.name)
		return data, 0, 0, nil

	case "application/pdf":
		data = generatePDF(sp.name)
		return data, 0, 0, nil

	case "video/mp4":
		data = minimalMP4()
		return data, 0, 0, nil

	case "application/zip":
		data, err = generateZip(sp.name)
		return data, 0, 0, err

	default:
		data = []byte("placeholder: " + sp.name)
		return data, 0, 0, nil
	}
}

func shiftColor(c color.RGBA, offset int) color.RGBA {
	shift := uint8(offset * 30)
	return color.RGBA{
		R: c.R + shift,
		G: c.G,
		B: c.B + shift/2,
		A: c.A,
	}
}

func generatePNG(w, h int, bg color.RGBA, label string) []byte {
	if w == 0 {
		w = 256
	}
	if h == 0 {
		h = 256
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	// Draw a simple label as a darker rectangle in the centre
	drawLabel(img, label, bg)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func generateJPEG(w, h int, bg color.RGBA, label string) ([]byte, error) {
	if w == 0 {
		w = 256
	}
	if h == 0 {
		h = 256
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	drawLabel(img, label, bg)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawLabel draws a contrasting rectangle in the centre of the image as a
// visual label so thumbnails look distinct in the grid.
func drawLabel(img *image.RGBA, label string, bg color.RGBA) {
	bounds := img.Bounds()
	w := bounds.Max.X
	h := bounds.Max.Y

	// Contrasting colour
	fg := color.RGBA{R: 255 - bg.R, G: 255 - bg.G, B: 255 - bg.B, A: 255}

	// Draw a centred horizontal bar (simulates a text label)
	barH := h / 8
	if barH < 4 {
		barH = 4
	}
	barW := w * 3 / 4
	startX := (w - barW) / 2
	startY := (h - barH) / 2
	for y := startY; y < startY+barH; y++ {
		for x := startX; x < startX+barW; x++ {
			img.SetRGBA(x, y, fg)
		}
	}
	_ = label // label text rendering would need font package; the bar is enough
}

func generateSVG(label string) []byte {
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <rect width="100" height="100" fill="#6366f1"/>
  <circle cx="50" cy="50" r="30" fill="#fff" opacity="0.8"/>
  <text x="50" y="54" font-family="sans-serif" font-size="8" text-anchor="middle" fill="#6366f1">%s</text>
</svg>`, label)
	return []byte(svg)
}

func generatePDF(label string) []byte {
	// Minimal valid single-page PDF
	body := fmt.Sprintf(`1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Contents 4 0 R/Resources<</Font<</F1<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>>>>>>>endobj
4 0 obj<</Length 44>>
stream
BT /F1 18 Tf 72 720 Td (%s) Tj ET
endstream
endobj`, label)

	xrefOffset := 9 + len(`%PDF-1.4
`)
	pdf := fmt.Sprintf(`%%PDF-1.4
%s
xref
0 5
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000274 00000 n
trailer<</Size 5/Root 1 0 R>>
startxref
%d
%%%%EOF`, body, xrefOffset)
	return []byte(pdf)
}

// minimalMP4 returns a valid minimal MP4 container (~200 bytes).
// This is a ftyp + mdat with no video data — enough for MIME detection.
func minimalMP4() []byte {
	// ftyp box: size(4) + 'ftyp'(4) + 'mp42'(4) + version(4) + 'mp42'(4) + 'isom'(4) = 24 bytes
	ftyp := []byte{
		0, 0, 0, 24, // size = 24
		'f', 't', 'y', 'p',
		'm', 'p', '4', '2',
		0, 0, 0, 0,
		'm', 'p', '4', '2',
		'i', 's', 'o', 'm',
	}
	// mdat box: size(4) + 'mdat'(4) = 8 bytes, empty
	mdat := []byte{
		0, 0, 0, 8,
		'm', 'd', 'a', 't',
	}
	return append(ftyp, mdat...)
}

func generateZip(label string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("readme.txt")
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(w, "Demo archive: %s\n", label)
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
