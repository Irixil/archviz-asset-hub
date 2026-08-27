package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"damask/server/internal/assetio"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"

	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdftypes "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type stackMergePayload struct {
	WorkspaceID    string   `json:"workspace_id"`
	CreatedBy      string   `json:"created_by"`
	AssetIDs       []string `json:"asset_ids"`
	OutputType     string   `json:"output_type"`
	OutputFilename string   `json:"output_filename"`
	GifFrameMs     int      `json:"gif_frame_ms"`
}

func (s *JobServer) jobStackMerge(ctx context.Context, job dbgen.Job) error {
	var p stackMergePayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "damask-stack-merge-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	entries := s.collectStackEntries(ctx, p.AssetIDs, job.WorkspaceID)
	if len(entries) == 0 {
		return errors.New("no processable assets in stack")
	}

	localPaths := s.downloadStackEntries(ctx, tmpDir, entries)
	if len(localPaths) == 0 {
		return errors.New("no assets could be downloaded")
	}

	var outPath, outExt string
	switch p.OutputType {
	case "gif":
		outPath = filepath.Join(tmpDir, "output.gif")
		outExt = ".gif"
		if e := buildGIF(localPaths, outPath, p.GifFrameMs); e != nil {
			return fmt.Errorf("build gif: %w", e)
		}
	case "pdf":
		outPath = filepath.Join(tmpDir, "output.pdf")
		outExt = ".pdf"
		if e := buildPDF(localPaths, outPath); e != nil {
			return fmt.Errorf("build pdf: %w", e)
		}
	default:
		return fmt.Errorf("unsupported output type: %s", p.OutputType)
	}

	filename := p.OutputFilename
	if filename == "" {
		filename = "stack-merge"
	}

	asset, err := s.ingester.IngestFile(ctx, p.WorkspaceID, outPath, assetio.IngestFileOpts{
		UserID:       p.CreatedBy,
		OriginalName: filename + outExt,
	})
	if err != nil {
		return fmt.Errorf("create asset: %w", err)
	}

	resultBytes, _ := json.Marshal(map[string]string{"asset_id": asset.ID})
	resultStr := string(resultBytes)
	if e := s.queries.CompleteJobWithResult(ctx, dbgen.CompleteJobWithResultParams{
		Result: &resultStr,
		ID:     job.ID,
	}); e != nil {
		slog.ErrorContext(ctx, "stack_merge: could not persist result", "err", e)
	}

	s.hub.Publish(ctx, p.WorkspaceID, events.StackMergeDone(asset.ID, job.ID))

	return nil
}

type stackMergeEntry struct{ storageKey, ext string }

// collectStackEntries resolves each requested asset ID to its current
// version's storage key, skipping assets with no current version or that
// don't belong to the merging workspace.
func (s *JobServer) collectStackEntries(ctx context.Context, assetIDs []string, workspaceID string) []stackMergeEntry {
	var entries []stackMergeEntry
	for _, assetID := range assetIDs {
		ver, err := s.queries.GetCurrentVersion(ctx, assetID)
		if err != nil {
			slog.WarnContext(ctx, "stack_merge: skip asset (no current version)", "asset_id", assetID)
			continue
		}
		if ver.WorkspaceID != workspaceID {
			slog.WarnContext(ctx, "stack_merge: skip asset (not in workspace)",
				"asset_id", assetID, "workspace_id", workspaceID)
			continue
		}
		ext := filepath.Ext(ver.StorageKey)
		if ext == "" {
			ext = ".bin"
		}
		entries = append(entries, stackMergeEntry{storageKey: ver.StorageKey, ext: ext})
	}
	return entries
}

// downloadStackEntries fetches each entry's file into tmpDir, skipping any
// that fail to read or copy; the returned paths are in entry order.
func (s *JobServer) downloadStackEntries(ctx context.Context, tmpDir string, entries []stackMergeEntry) []string {
	var localPaths []string
	for i, ent := range entries {
		rc, getErr := s.storage.Get(ctx, ent.storageKey)
		if getErr != nil {
			slog.WarnContext(ctx, "stack_merge: skip asset (storage error)", "key", ent.storageKey, "err", getErr)
			continue
		}
		path := filepath.Join(tmpDir, fmt.Sprintf("%04d%s", i, ent.ext))
		f, createErr := os.Create(path)
		if createErr != nil {
			_ = rc.Close()
			continue
		}
		if _, copyErr := io.Copy(f, rc); copyErr != nil {
			_ = f.Close()
			_ = rc.Close()
			slog.WarnContext(ctx, "stack_merge: copy error", "key", ent.storageKey, "err", copyErr)
			continue
		}
		_ = f.Close()
		_ = rc.Close()
		localPaths = append(localPaths, path)
	}
	return localPaths
}

func buildGIF(paths []string, outPath string, frameMs int) error {
	delay := frameMs / 10 //nolint:mnd // GIF delay is in units of 10ms
	if delay <= 0 {
		delay = 50
	}

	outGIF := &gif.GIF{}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		_ = f.Close()
		if err != nil {
			continue
		}
		bounds := img.Bounds()
		palettedImg := image.NewPaletted(bounds, palette.Plan9)
		draw.FloydSteinberg.Draw(palettedImg, bounds, img, bounds.Min)
		outGIF.Image = append(outGIF.Image, palettedImg)
		outGIF.Delay = append(outGIF.Delay, delay)
	}
	if len(outGIF.Image) == 0 {
		return errors.New("no valid images to encode as GIF")
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	return gif.EncodeAll(out, outGIF)
}

func buildPDF(paths []string, outPath string) error {
	imageExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".tif":  true,
		".tiff": true,
		".webp": true,
	}
	var imgPaths []string
	for _, p := range paths {
		if imageExts[filepath.Ext(p)] {
			imgPaths = append(imgPaths, p)
		}
	}
	if len(imgPaths) == 0 {
		return buildRawTextPDFFile(paths, outPath)
	}

	imp, err := pdfcpuapi.Import("f:A4, pos:full, sc:1.0", pdftypes.POINTS)
	if err != nil {
		return fmt.Errorf("pdfcpu import config: %w", err)
	}
	return pdfcpuapi.ImportImagesFile(imgPaths, outPath, imp, nil)
}

func buildRawTextPDFFile(paths []string, outPath string) error {
	var lines []string
	for _, p := range paths {
		lines = append(lines, filepath.Base(p))
	}
	pdf := makeRawTextPDF(lines)
	return os.WriteFile(outPath, pdf, 0o600)
}

func makeRawTextPDF(lines []string) []byte {
	var content string
	y := 750
	var contentSb205 strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&contentSb205, "BT /F1 12 Tf 50 %d Td (%s) Tj ET\n", y, sanitizePDFString(l))
		y -= 20
	}
	content += contentSb205.String()
	streamLen := len(content)

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	o1 := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	o2 := b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	o3 := b.Len()
	b.WriteString(
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n",
	)
	o4 := b.Len()
	fmt.Fprintf(&b, "4 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n", streamLen, content)
	o5 := b.Len()
	b.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")
	xref := b.Len()
	fmt.Fprintf(
		&b,
		"xref\n0 6\n0000000000 65535 f \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n",
		o1,
		o2,
		o3,
		o4,
		o5,
	)
	fmt.Fprintf(&b, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return b.Bytes()
}

func sanitizePDFString(s string) string {
	var out []byte
	for i := range len(s) {
		c := s[i]
		if c == '(' || c == ')' || c == '\\' {
			out = append(out, '\\')
		}
		if c >= 0x20 && c < 0x7f {
			out = append(out, c)
		}
	}
	return string(out)
}
