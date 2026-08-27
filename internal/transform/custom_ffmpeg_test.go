package transform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"damask/server/internal/config"
)

const validCustomCmd = "ffmpeg -i {input} -vf scale=320:-2 -c:v libx264 {output}"

// ---- StripLeadingDescription — no ffmpeg needed ----

func TestStripLeadingDescription_NoHashLine(t *testing.T) {
	desc, content := StripLeadingDescription(validCustomCmd)
	if desc != "" {
		t.Fatalf("expected no description, got %q", desc)
	}
	if content != validCustomCmd {
		t.Fatalf("expected content unchanged, got %q", content)
	}
}

func TestStripLeadingDescription_HashLineStripped(t *testing.T) {
	desc, content := StripLeadingDescription("# downscale for web\n" + validCustomCmd)
	if desc != "downscale for web" {
		t.Fatalf("expected description %q, got %q", "downscale for web", desc)
	}
	if content != validCustomCmd {
		t.Fatalf("expected content %q, got %q", validCustomCmd, content)
	}
}

func TestStripLeadingDescription_HashLineOnly(t *testing.T) {
	desc, content := StripLeadingDescription("# just a label")
	if desc != "just a label" {
		t.Fatalf("expected description %q, got %q", "just a label", desc)
	}
	if content != "" {
		t.Fatalf("expected empty content, got %q", content)
	}
}

func TestStripLeadingDescription_HashMidStringNotStripped(t *testing.T) {
	raw := "ffmpeg -i {input} # not a leading line\n -c copy {output}"
	desc, content := StripLeadingDescription(raw)
	if desc != "" {
		t.Fatalf("expected no description, got %q", desc)
	}
	if content != strings.TrimSpace(raw) {
		t.Fatalf("expected content unchanged (trimmed), got %q", content)
	}
}

// ---- ValidateCustomCommand — no ffmpeg needed ----

func TestValidateCustomCommand_HappyPath(t *testing.T) {
	if err := ValidateCustomCommand(validCustomCmd); err != nil {
		t.Fatalf("expected valid command, got error: %v", err)
	}
}

func TestValidateCustomCommand_DescriptionLineIgnored(t *testing.T) {
	if err := ValidateCustomCommand("# downscale for web\n" + validCustomCmd); err != nil {
		t.Fatalf("expected valid command with description line, got error: %v", err)
	}
}

func TestValidateCustomCommand_DescriptionLineOnlyMissingTokens(t *testing.T) {
	err := ValidateCustomCommand("# just a label")
	if !errors.Is(err, ErrCustomFFmpegNoInputToken) {
		t.Fatalf("expected ErrCustomFFmpegNoInputToken for description-only command, got %v", err)
	}
}

func TestValidateCustomCommand_MissingInputToken(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i src.mp4 -c copy {output}")
	if !errors.Is(err, ErrCustomFFmpegNoInputToken) {
		t.Fatalf("expected ErrCustomFFmpegNoInputToken, got %v", err)
	}
}

func TestValidateCustomCommand_MissingOutputToken(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -c copy out.mp4")
	if !errors.Is(err, ErrCustomFFmpegNoOutputToken) {
		t.Fatalf("expected ErrCustomFFmpegNoOutputToken, got %v", err)
	}
}

func TestValidateCustomCommand_DuplicateInputToken(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -i {input} -c copy {output}")
	if !errors.Is(err, ErrCustomFFmpegNoInputToken) {
		t.Fatalf("expected ErrCustomFFmpegNoInputToken for duplicate token, got %v", err)
	}
}

func TestValidateCustomCommand_ShellMetacharRejected(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -c copy {output}; rm -rf /")
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

func TestValidateCustomCommand_AbsolutePathRejected(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -c copy /etc/passwd {output}")
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

func TestValidateCustomCommand_DotDotRejected(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -c copy ../../etc/passwd {output}")
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

func TestValidateCustomCommand_ProtocolHandlerRejected(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -i lavfi:testsrc -c copy {output}")
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

// TestValidateCustomCommand_QuotedFilterChainSemicolonAllowed is a regression
// test for a reported failure: a multi-chain -filter_complex graph
// (chains separated by ';', as ffmpeg's filtergraph syntax requires) was
// rejected because the semicolon tripped shellMetacharPattern even though it
// only ever appeared inside the quoted -filter_complex value.
func TestValidateCustomCommand_QuotedFilterChainSemicolonAllowed(t *testing.T) {
	cmd := `ffmpeg -i {input} -filter_complex "[0:v]split[a][b];[a][b]hstack[v]" -map "[v]" {output}`
	if err := ValidateCustomCommand(cmd); err != nil {
		t.Fatalf("expected quoted filter-chain semicolon to be allowed, got error: %v", err)
	}
}

// TestValidateCustomCommand_UnquotedSemicolonStillRejected confirms the
// quoting exemption doesn't widen into a general semicolon allowance — a
// semicolon outside of any quotes is still blacklisted.
func TestValidateCustomCommand_UnquotedSemicolonStillRejected(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -c copy {output};rm -rf /tmp")
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

// TestValidateCustomCommand_MixedQuotedUnquotedSemicolonRejected confirms a
// token built from concatenated quoted/unquoted parts (e.g. foo"a;b") is
// conservatively treated as unquoted, so a semicolon smuggled in via partial
// quoting is still rejected.
func TestValidateCustomCommand_MixedQuotedUnquotedSemicolonRejected(t *testing.T) {
	err := ValidateCustomCommand(`ffmpeg -i {input} -c copy out"; rm -rf /tmp"{output}`)
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

func TestValidateCustomCommand_QuotedAbsolutePathRejected(t *testing.T) {
	// Quoting the path must not let it slip past the blacklist — the
	// blacklist has to run against the dequoted token, not the raw one.
	err := ValidateCustomCommand(`ffmpeg -i {input} -c copy "/etc/passwd" {output}`)
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

// TestValidateCustomCommand_MovieFilterOptionRejected is a regression test
// for the lavfi movie/amovie filter-option bypass: the blacklist used to
// only catch protocol-prefix syntax (movie:), not filter-option syntax
// (movie=), letting an attacker read arbitrary local files through a
// lavfi virtual source despite the path/protocol blacklist.
func TestValidateCustomCommand_MovieFilterOptionRejected(t *testing.T) {
	cmd := `ffmpeg -i {input} -f lavfi -i "movie=/etc/passwd" -filter_complex overlay {output}`
	err := ValidateCustomCommand(cmd)
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

// TestValidateCustomCommand_AmovieFilterOptionRejected mirrors
// TestValidateCustomCommand_MovieFilterOptionRejected for amovie, including
// the SSRF variant where the "file" is actually a URL.
func TestValidateCustomCommand_AmovieFilterOptionRejected(t *testing.T) {
	cmd := `ffmpeg -i {input} -f lavfi -i "amovie='http://169.254.169.254/'" -filter_complex amix {output}`
	err := ValidateCustomCommand(cmd)
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

// TestValidateCustomCommand_MovieFilterInsideFilterComplexRejected confirms
// the fix isn't scoped only to the "-f lavfi" case: movie/amovie are usable
// as a source filter inside any filtergraph attached to a normal input.
func TestValidateCustomCommand_MovieFilterInsideFilterComplexRejected(t *testing.T) {
	cmd := `ffmpeg -i {input} -filter_complex "movie=/etc/passwd[wm];[0:v][wm]overlay" {output}`
	err := ValidateCustomCommand(cmd)
	if !errors.Is(err, ErrCustomFFmpegBlacklisted) {
		t.Fatalf("expected ErrCustomFFmpegBlacklisted, got %v", err)
	}
}

// TestValidateCustomCommand_MovieSubstringInLargerWordAllowed guards against
// the \b word-boundary in the movie/amovie pattern regressing into a bare
// substring match, which would reject unrelated tokens that merely contain
// "movie=" as part of a longer identifier rather than the actual filter name.
func TestValidateCustomCommand_MovieSubstringInLargerWordAllowed(t *testing.T) {
	cmd := `ffmpeg -i {input} -metadata slowmovie=1 {output}`
	if err := ValidateCustomCommand(cmd); err != nil {
		t.Fatalf("expected command with non-filter substring match to be allowed, got error: %v", err)
	}
}

func TestValidateCustomCommand_UnterminatedQuoteRejected(t *testing.T) {
	err := ValidateCustomCommand(`ffmpeg -i {input} -vf "scale=320:-2 -c copy {output}`)
	if !errors.Is(err, ErrCustomFFmpegUnterminatedQuote) {
		t.Fatalf("expected ErrCustomFFmpegUnterminatedQuote, got %v", err)
	}
}

// ---- splitCommandTokens ----

func TestSplitCommandTokens_QuotedArgumentStripsQuotes(t *testing.T) {
	tokens, err := splitCommandTokens(
		`ffmpeg -i {input} -vf "zoompan=z='if(lte(zoom,1.0),1.5,max(1.001,zoom-0.0015))':d=125" -s "800x450" {output}`,
	)
	if err != nil {
		t.Fatalf("splitCommandTokens: %v", err)
	}
	want := []string{
		"ffmpeg", "-i", "{input}", "-vf",
		"zoompan=z='if(lte(zoom,1.0),1.5,max(1.001,zoom-0.0015))':d=125",
		"-s", "800x450", "{output}",
	}
	if !slices.Equal(tokens, want) {
		t.Fatalf("tokens = %#v, want %#v", tokens, want)
	}
}

func TestSplitCommandTokens_MixedQuoteConcatenation(t *testing.T) {
	tokens, err := splitCommandTokens(`foo"bar baz"qux 'next'`)
	if err != nil {
		t.Fatalf("splitCommandTokens: %v", err)
	}
	want := []string{"foobar bazqux", "next"}
	if !slices.Equal(tokens, want) {
		t.Fatalf("tokens = %#v, want %#v", tokens, want)
	}
}

func TestSplitCommandTokens_UnterminatedQuote(t *testing.T) {
	_, err := splitCommandTokens(`ffmpeg -vf "scale=320:-2`)
	if !errors.Is(err, ErrCustomFFmpegUnterminatedQuote) {
		t.Fatalf("expected ErrCustomFFmpegUnterminatedQuote, got %v", err)
	}
}

// ---- capStderrTail ----

func TestCapStderrTail_KeepsTailNotHead(t *testing.T) {
	// Banner alone exceeds the cap, so a head-based cap (the old behaviour)
	// would never reach realError at all — only a tail-based cap can.
	banner := strings.Repeat("b", customFFmpegStderrCap*2)
	realError := "No such filter: 'zoompan'"
	full := banner + realError

	if strings.Contains(full[:customFFmpegStderrCap], realError) {
		t.Fatalf("test setup invalid: realError must not fit in a head-based cap")
	}
	got := capStderrTail(full)
	if !strings.Contains(got, realError) {
		t.Fatalf("capStderrTail dropped the real error, got %q", got)
	}
}

func TestCapStderrTail_ShortStringUnchanged(t *testing.T) {
	short := "short error"
	if got := capStderrTail(short); got != short {
		t.Fatalf("capStderrTail = %q, want unchanged %q", got, short)
	}
}

func TestValidateCustomCommand_TooLong(t *testing.T) {
	cmd := "ffmpeg -i {input} -c copy {output} " + strings.Repeat("a", maxCustomCommandLen)
	err := ValidateCustomCommand(cmd)
	if !errors.Is(err, ErrCustomFFmpegTooLong) {
		t.Fatalf("expected ErrCustomFFmpegTooLong for too-long command, got %v", err)
	}
}

// ---- Asset/variant ref tokens — no ffmpeg needed ----

func TestExtractRefTokens_Asset(t *testing.T) {
	got := ExtractRefTokens("ffmpeg -i {input} -i {asset:abc123} {output}")
	want := []RefToken{{Token: "{asset:abc123}", Kind: "asset", ID: "abc123"}}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractRefTokens = %+v, want %+v", got, want)
	}
}

func TestExtractRefTokens_Variant(t *testing.T) {
	got := ExtractRefTokens("ffmpeg -i {input} -i {variant:xyz789} {output}")
	want := []RefToken{{Token: "{variant:xyz789}", Kind: "variant", ID: "xyz789"}}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractRefTokens = %+v, want %+v", got, want)
	}
}

func TestExtractRefTokens_Mixed(t *testing.T) {
	got := ExtractRefTokens("ffmpeg -i {input} -i {asset:A} -i {variant:B} {output}")
	want := []RefToken{
		{Token: "{asset:A}", Kind: "asset", ID: "A"},
		{Token: "{variant:B}", Kind: "variant", ID: "B"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractRefTokens = %+v, want %+v", got, want)
	}
}

func TestExtractRefTokens_None(t *testing.T) {
	got := ExtractRefTokens("ffmpeg -i {input} -vf scale=1280:-2 {output}")
	if got != nil {
		t.Fatalf("ExtractRefTokens = %+v, want nil", got)
	}
}

func TestExtractRefTokens_Deduplication(t *testing.T) {
	got := ExtractRefTokens("ffmpeg -i {input} -i {asset:abc123} -i {asset:abc123} {output}")
	want := []RefToken{{Token: "{asset:abc123}", Kind: "asset", ID: "abc123"}}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractRefTokens = %+v, want %+v", got, want)
	}
}

func TestExtractRefTokens_EmbeddedInFilterArg(t *testing.T) {
	got := ExtractRefTokens("ffmpeg -i {input} -vf subtitles={asset:abc123} {output}")
	want := []RefToken{{Token: "{asset:abc123}", Kind: "asset", ID: "abc123"}}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractRefTokens = %+v, want %+v", got, want)
	}
}

func TestExtractRefTokens_MultipleEmbeddedInSameToken(t *testing.T) {
	got := ExtractRefTokens(
		"ffmpeg -i {input} -filter_complex subtitles={asset:A},subtitles={variant:B} {output}",
	)
	want := []RefToken{
		{Token: "{asset:A}", Kind: "asset", ID: "A"},
		{Token: "{variant:B}", Kind: "variant", ID: "B"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractRefTokens = %+v, want %+v", got, want)
	}
}

func TestValidateCustomCommand_AssetRefPassesBlacklist(t *testing.T) {
	cmd := "ffmpeg -i {input} -i {asset:01ARZ3NDEKTSV4RRFFQ69G5FAV} {output}"
	if err := ValidateCustomCommand(cmd); err != nil {
		t.Fatalf("expected well-formed ref token to pass validation, got %v", err)
	}
}

func TestValidateCustomCommand_MalformedRefToken_ReturnsErr(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -i {asset:} {output}")
	if !errors.Is(err, ErrCustomFFmpegBadRefToken) {
		t.Fatalf("expected ErrCustomFFmpegBadRefToken, got %v", err)
	}
}

func TestValidateCustomCommand_RefTokenIDTooLong_ReturnsErr(t *testing.T) {
	cmd := "ffmpeg -i {input} -i {asset:" + strings.Repeat("a", 101) + "} {output}"
	err := ValidateCustomCommand(cmd)
	if !errors.Is(err, ErrCustomFFmpegBadRefToken) {
		t.Fatalf("expected ErrCustomFFmpegBadRefToken, got %v", err)
	}
}

func TestValidateCustomCommand_EmbeddedRefPassesBlacklist(t *testing.T) {
	cmd := "ffmpeg -i {input} -vf subtitles={asset:01ARZ3NDEKTSV4RRFFQ69G5FAV} {output}"
	if err := ValidateCustomCommand(cmd); err != nil {
		t.Fatalf("expected well-formed embedded ref token to pass validation, got %v", err)
	}
}

func TestValidateCustomCommand_MalformedEmbeddedRefToken_ReturnsErr(t *testing.T) {
	err := ValidateCustomCommand("ffmpeg -i {input} -vf subtitles={asset:} {output}")
	if !errors.Is(err, ErrCustomFFmpegBadRefToken) {
		t.Fatalf("expected ErrCustomFFmpegBadRefToken, got %v", err)
	}
}

func TestValidateCustomCommand_EmbeddedRefTokenIDTooLong_ReturnsErr(t *testing.T) {
	cmd := "ffmpeg -i {input} -vf subtitles={asset:" + strings.Repeat("a", 101) + "} {output}"
	err := ValidateCustomCommand(cmd)
	if !errors.Is(err, ErrCustomFFmpegBadRefToken) {
		t.Fatalf("expected ErrCustomFFmpegBadRefToken, got %v", err)
	}
}

// ---- RunCustomFFmpeg / DetectOutputMIME — integration, skip if no ffmpeg ----

// generateTestImage writes a tiny JPEG to dir via ffmpeg's lavfi color
// source — used by tests that need an image input (e.g. for -loop, which is
// only a valid option against an image source, not a video one).
func generateTestImage(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "input.jpg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx,
		"ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "color=c=blue:s=640x480",
		"-frames:v", "1", "-update", "1", path,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("generate test image: %v: %s", err, out)
	}
	return path
}

// TestRunCustomFFmpeg_QuotedZoompanCommand is a regression test for a
// reported failure: a command that double-quotes its -vf and -s values (as
// it would need to for a real shell, to protect the filtergraph's
// parentheses/colons) failed because the quote characters were passed
// through to ffmpeg literally instead of being stripped.
func TestRunCustomFFmpeg_QuotedZoompanCommand(t *testing.T) {
	requireFFmpeg(t)

	outDir := t.TempDir()
	src := generateTestImage(t, outDir)

	tr := NewTransformer()
	outputPath, err := tr.RunCustomFFmpeg(
		context.Background(),
		`ffmpeg -loop 1 -i {input} -vf "zoompan=z='if(lte(zoom,1.0),1.5,max(1.001,zoom-0.0015))':d=125" -c:v libx264 -t 5 -s "800x450" {output}`,
		src,
		outDir,
		nil,
	)
	if err != nil {
		t.Fatalf("RunCustomFFmpeg: %v", err)
	}
	assertFileWritten(t, outputPath)
}

func TestRunCustomFFmpeg_HappyPath(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	tr := NewTransformer()
	outputPath, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -i {input} -t 1 -c copy {output}",
		src,
		outDir,
		nil,
	)
	if err != nil {
		t.Fatalf("RunCustomFFmpeg: %v", err)
	}
	assertFileWritten(t, outputPath)
}

// TestRunCustomFFmpeg_NilRefs_NoSubstitutionError verifies a command with no
// {asset:}/{variant:} tokens still runs fine when refs is nil — ranging over
// a nil map is a no-op.
func TestRunCustomFFmpeg_NilRefs_NoSubstitutionError(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	tr := NewTransformer()
	outputPath, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -i {input} -t 1 -c copy {output}",
		src,
		outDir,
		nil,
	)
	if err != nil {
		t.Fatalf("RunCustomFFmpeg: %v", err)
	}
	assertFileWritten(t, outputPath)
}

// TestRunCustomFFmpeg_RefSubstitution verifies a {asset:ID} token in the
// command is replaced with the local path supplied via refs.
func TestRunCustomFFmpeg_RefSubstitution(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	refSrc, err := filepath.Abs(testdataPath(t, "sample_video_with_audio.mp4"))
	if err != nil {
		t.Fatalf("abs ref path: %v", err)
	}
	outDir := t.TempDir()

	tr := NewTransformer()
	outputPath, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -i {input} -i {asset:REF_ID} -filter_complex amix=inputs=2 -t 1 {output}",
		src,
		outDir,
		map[string]string{"{asset:REF_ID}": refSrc},
	)
	if err != nil {
		t.Fatalf("RunCustomFFmpeg: %v", err)
	}
	assertFileWritten(t, outputPath)
}

// TestRunCustomFFmpeg_RefSubstitution_Embedded verifies a {asset:ID} token
// embedded inside a larger argument (e.g. -metadata comment={asset:ID}, the
// same shape as -vf subtitles={asset:ID}) is substituted correctly, not just
// a ref token that is the entire argument by itself.
func TestRunCustomFFmpeg_RefSubstitution_Embedded(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	refSrc, err := filepath.Abs(testdataPath(t, "sample_video_with_audio.mp4"))
	if err != nil {
		t.Fatalf("abs ref path: %v", err)
	}
	outDir := t.TempDir()

	tr := NewTransformer()
	outputPath, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -i {input} -metadata comment=ref:{asset:REF_ID} -t 1 -c copy {output}",
		src,
		outDir,
		map[string]string{"{asset:REF_ID}": refSrc},
	)
	if err != nil {
		t.Fatalf("RunCustomFFmpeg: %v", err)
	}
	assertFileWritten(t, outputPath)
}

func TestRunCustomFFmpeg_DescriptionLineIgnored(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	tr := NewTransformer()
	outputPath, err := tr.RunCustomFFmpeg(
		context.Background(),
		"# trim to 1s\nffmpeg -i {input} -t 1 -c copy {output}",
		src,
		outDir,
		nil,
	)
	if err != nil {
		t.Fatalf("RunCustomFFmpeg: %v", err)
	}
	assertFileWritten(t, outputPath)
}

func TestRunCustomFFmpeg_BadCommand_ReturnsFFmpegFailed(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	tr := NewTransformer()
	_, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -i {input} -vf not_a_real_filter {output}",
		src,
		outDir,
		nil,
	)
	if !errors.Is(err, ErrCustomFFmpegFailed) {
		t.Fatalf("expected ErrCustomFFmpegFailed, got %v", err)
	}
}

func TestRunCustomFFmpeg_ProtocolWhitelistBlocksNonFileProtocol(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	tr := NewTransformer()
	// -v error suppresses ffmpeg's version banner and stream info so the
	// (size-capped) stderr snippet we assert on actually contains the
	// rejection message instead of being pushed out by the banner.
	_, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -v error -i {input} -i tcp://127.0.0.1:9 -map 0:v -c copy {output}",
		src,
		outDir,
		nil,
	)
	if !errors.Is(err, ErrCustomFFmpegFailed) {
		t.Fatalf("expected ErrCustomFFmpegFailed, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "whitelist") {
		t.Fatalf("expected ffmpeg to reject the non-file protocol via whitelist, got: %v", err)
	}
}

func TestRunCustomFFmpeg_UserProtocolWhitelistOverrideStripped(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	tr := NewTransformer()
	_, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -v error -protocol_whitelist file,tcp -i {input} -i tcp://127.0.0.1:9 -map 0:v -c copy {output}",
		src,
		outDir,
		nil,
	)
	if !errors.Is(err, ErrCustomFFmpegFailed) {
		t.Fatalf("expected user-supplied -protocol_whitelist override to be stripped, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "whitelist") {
		t.Fatalf("expected ffmpeg to reject the non-file protocol via whitelist, got: %v", err)
	}
}

// TestRunCustomFFmpeg_SafeFlagStripped is a regression test for the concat
// demuxer gap: -f concat reads the *contents* of the input file as a
// playlist that can reference arbitrary paths, and -safe 0 is what disables
// the concat demuxer's own rejection of unsafe (absolute/non-local)
// references in that playlist. The malicious paths live in the asset's
// bytes, not the command string, so the token blacklist can never see them
// — RunCustomFFmpeg must strip a user-supplied -safe so the demuxer falls
// back to its safe=1 default and rejects the absolute path itself.
func TestRunCustomFFmpeg_SafeFlagStripped(t *testing.T) {
	requireFFmpeg(t)

	outDir := t.TempDir()
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "playlist.txt")
	if err := os.WriteFile(src, []byte("file '/etc/passwd'\n"), 0o600); err != nil {
		t.Fatalf("write playlist: %v", err)
	}

	tr := NewTransformer()
	_, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -v error -f concat -safe 0 -i {input} -c copy {output}",
		src,
		outDir,
		nil,
	)
	if !errors.Is(err, ErrCustomFFmpegFailed) {
		t.Fatalf("expected ErrCustomFFmpegFailed (stripped -safe falls back to safe=1), got %v", err)
	}
}

func TestRunCustomFFmpeg_CmdDirPinnedToOutDir(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	// A bare relative filename only resolves if ffmpeg's cwd is pinned to
	// outDir — if cmd.Dir weren't set, this would resolve against the test
	// binary's own working directory instead and fail to find the file.
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read src: %v", err)
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, "second.mp4"), data, 0o600); writeErr != nil {
		t.Fatalf("write second input: %v", writeErr)
	}

	tr := NewTransformer()
	outputPath, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -i {input} -i second.mp4 -map 0:v -map 1:a -t 1 -c copy {output}",
		src,
		outDir,
		nil,
	)
	if err != nil {
		t.Fatalf("RunCustomFFmpeg: %v", err)
	}
	assertFileWritten(t, outputPath)
}

func TestDetectOutputMIME_MP4(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	tr := NewTransformer()
	outputPath, err := tr.RunCustomFFmpeg(
		context.Background(),
		"ffmpeg -i {input} -t 1 -c copy -f mp4 {output}",
		src,
		outDir,
		nil,
	)
	if err != nil {
		t.Fatalf("RunCustomFFmpeg: %v", err)
	}

	mimeType := tr.DetectOutputMIME(context.Background(), outputPath)
	if mimeType != MimeVideoMP4 {
		t.Fatalf("DetectOutputMIME = %q, want %q", mimeType, MimeVideoMP4)
	}
}

func TestDetectOutputMIME_UnknownFallback(t *testing.T) {
	requireFFmpeg(t)

	tr := NewTransformer()
	mimeType := tr.DetectOutputMIME(context.Background(), "/nonexistent/not_a_real_file.bin")
	if mimeType != MimeApplicationOctetStream {
		t.Fatalf("DetectOutputMIME = %q, want fallback %q", mimeType, MimeApplicationOctetStream)
	}
}

// ---- TrimToSeconds ----

func TestTrimToSeconds_ProducesShortFile(t *testing.T) {
	requireFFmpeg(t)

	src := testdataPath(t, "sample_video_with_audio.mp4")
	outDir := t.TempDir()

	tr := NewTransformer()
	trimmed, err := tr.TrimToSeconds(context.Background(), src, 1, outDir)
	if err != nil {
		t.Fatalf("TrimToSeconds: %v", err)
	}
	if trimmed == src {
		t.Fatalf("expected a new trimmed file, got source path back")
	}
	assertFileWritten(t, trimmed)
}

func TestTrimToSeconds_FallsBackOnError(t *testing.T) {
	tr := NewTransformer(config.FFmpegConfig{Path: "/definitely/missing/ffmpeg"})
	src := "/nonexistent/source.mp4"
	got, err := tr.TrimToSeconds(context.Background(), src, 5, t.TempDir())
	if err == nil {
		t.Fatalf("expected error when ffmpeg is unavailable")
	}
	if got != src {
		t.Fatalf("expected fallback to srcPath, got %q", got)
	}
}
