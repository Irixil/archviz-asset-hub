package transform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Sentinel errors returned by ValidateCustomCommand and RunCustomFFmpeg.
var (
	ErrCustomFFmpegBlacklisted       = errors.New("command contains a blacklisted pattern")
	ErrCustomFFmpegTooLong           = errors.New("command exceeds maximum length")
	ErrCustomFFmpegNoInputToken      = errors.New("command must contain {input}")
	ErrCustomFFmpegNoOutputToken     = errors.New("command must contain {output}")
	ErrCustomFFmpegFailed            = errors.New("ffmpeg exited with error")
	ErrCustomFFmpegUnterminatedQuote = errors.New("command has an unterminated quote")
	ErrCustomFFmpegBadRefToken       = errors.New("command contains a malformed asset/variant reference")
)

const (
	maxCustomCommandLen      = 2000
	customFFmpegStderrCap    = 500
	customFFmpegTimeout      = 10 * time.Minute
	customFFmpegProbeTimeout = 30 * time.Second
	customFFmpegTrimTimeout  = 30 * time.Second
)

// shellMetacharPattern is only checked against tokens that were NOT quoted
// in the original command (see [splitCommandTokensWithQuoting]). Legitimate
// ffmpeg syntax relies on these characters inside quoted arguments — e.g.
// ';' separates filter chains within a quoted -filter_complex value — so
// blocking them unconditionally would reject valid commands. The command is
// always executed via [exec.Command] (never sh -c), so none of this is an
// actual shell injection vector either way; the quoting distinction exists
// purely to avoid rejecting syntactically valid, quoted filtergraphs while
// still flagging a stray unquoted metacharacter as a clear error rather than
// a confusing ffmpeg failure.
var shellMetacharPattern = regexp.MustCompile(`[;&|` + "`" + `$<>]`)

// pathLikePatterns are matched against every token regardless of quoting, as
// produced by [splitCommandTokensWithQuoting] (i.e. quotes already stripped
// — matching against raw, still-quoted tokens would let a quoted absolute
// path or protocol handler slip past these patterns). {input} and {output}
// are the only path-like tokens allowed — anything else that looks like a
// path or an ffmpeg protocol handler that could read/write outside the
// sandboxed temp dirs is rejected.
var pathLikePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^/`),
	regexp.MustCompile(`^[A-Za-z]:\\`),
	regexp.MustCompile(`\.\.`),
	regexp.MustCompile(`^(lavfi|concat|pipe|data|crypto|subfile|amovie|movie):`),
	// Unlike the patterns above, this one is intentionally unanchored: the
	// movie/amovie source filters can read an arbitrary local file or URL
	// via filter-option syntax (movie=/etc/passwd, amovie='http://...')
	// rather than protocol-prefix syntax, and that syntax works inside any
	// -vf/-filter_complex value — with or without -f lavfi on the command
	// — so the dangerous substring can appear anywhere inside a token, not
	// just at its start. This also defeats the -protocol_whitelist
	// hardening below, since movie/amovie open their own internal
	// AVFormatContext that doesn't inherit it. \b prevents false positives
	// on unrelated identifiers that merely contain "movie" as a substring.
	regexp.MustCompile(`\b(amovie|movie)\s*=`),
}

// refTokenPattern matches a well-formed {asset:ID} or {variant:ID} token
// when applied to a candidate substring on its own (anchored both ends, so
// it only accepts the substring if it has no malformed leftovers). IDs may
// be ULIDs (26 chars) or UUID-style (32 hex + 4 hyphens); the 1-100 char
// range is deliberately broad to accommodate both without being so wide that
// an oversized token sails through unflagged.
var refTokenPattern = regexp.MustCompile(`^\{(asset|variant):([a-zA-Z0-9_-]{1,100})\}$`)

// refTokenFindPattern is refTokenPattern without anchors, used to locate
// well-formed ref placeholders anywhere inside a token — not just when the
// placeholder is the entire token. A ref like {asset:ID} can appear embedded
// in a larger filter argument (e.g. -vf subtitles={asset:ID}), since some
// ffmpeg filters take their input as part of the filter string rather than a
// separate -i.
var refTokenFindPattern = regexp.MustCompile(`\{(asset|variant):([a-zA-Z0-9_-]{1,100})\}`)

// refLikePattern matches anything shaped like a ref placeholder — including
// malformed ones (empty or oversized ID, bad characters) — anywhere inside a
// token. ValidateCustomCommand uses this to catch a malformed embedded ref
// that refTokenFindPattern would simply skip over, which would otherwise let
// it through to ffmpeg as literal, unsubstituted text instead of being
// rejected up front.
var refLikePattern = regexp.MustCompile(`\{(asset|variant):[^{}]*\}`)

// RefToken represents a single {asset:ID} or {variant:ID} placeholder
// extracted from a custom FFmpeg command.
type RefToken struct {
	Token string // full placeholder, e.g. "{asset:abc123}" — used as map key
	Kind  string // "asset" or "variant"
	ID    string // the raw ID portion
}

// ExtractRefTokens returns all well-formed {asset:ID} and {variant:ID} tokens
// present in cmd (description line ignored, same tokenizer as
// [ValidateCustomCommand]), deduplicated by Token value — the same token
// appearing more than once still maps to a single temp file. A ref may be a
// whole token on its own (e.g. -i {asset:ID}) or embedded inside a larger one
// (e.g. -vf subtitles={asset:ID}); both are found. Substrings that look like
// a reference but fail the format check are not returned here;
// [ValidateCustomCommand] is responsible for rejecting those. Returns nil
// (not an error) when no ref tokens are found or the command fails to
// tokenize (an unterminated quote, say) — callers that need to know about a
// malformed command should consult ValidateCustomCommand directly.
func ExtractRefTokens(cmd string) []RefToken {
	_, content := StripLeadingDescription(cmd)
	tokens, err := splitCommandTokens(content)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var result []RefToken
	for _, tok := range tokens {
		for _, m := range refTokenFindPattern.FindAllStringSubmatch(tok, -1) {
			token := m[0]
			if _, dup := seen[token]; dup {
				continue
			}
			seen[token] = struct{}{}
			result = append(result, RefToken{Token: token, Kind: m[1], ID: m[2]})
		}
	}
	return result
}

// StripLeadingDescription splits raw into an optional leading "# ..." label
// line and the remaining content. The label, if present, is for display only
// (e.g. the variant param reuse-history menu) — content is what's actually
// sent to an AI provider or interpreted as an ffmpeg command.
func StripLeadingDescription(raw string) (description, content string) {
	first := raw
	rest := ""
	if before, after, ok := strings.Cut(raw, "\n"); ok {
		first, rest = before, after
	}
	trimmedFirst := strings.TrimSpace(first)
	if after, ok := strings.CutPrefix(trimmedFirst, "#"); ok {
		return strings.TrimSpace(after), strings.TrimSpace(rest)
	}
	return "", strings.TrimSpace(raw)
}

// ValidateCustomCommand checks a user-supplied ffmpeg command for length,
// the presence of exactly one {input} and one {output} token, and the
// absence of blacklisted patterns. It performs no I/O. A leading "# ..."
// description line (see [StripLeadingDescription]) is ignored for all of
// these checks.
func ValidateCustomCommand(cmd string) error {
	if len(cmd) > maxCustomCommandLen {
		return fmt.Errorf("%w: command exceeds %d characters", ErrCustomFFmpegTooLong, maxCustomCommandLen)
	}
	_, content := StripLeadingDescription(cmd)
	if strings.Count(content, "{input}") != 1 {
		return ErrCustomFFmpegNoInputToken
	}
	if strings.Count(content, "{output}") != 1 {
		return ErrCustomFFmpegNoOutputToken
	}
	tokens, quoted, err := splitCommandTokensWithQuoting(content)
	if err != nil {
		return err
	}
	for i, tok := range tokens {
		for _, refLike := range refLikePattern.FindAllString(tok, -1) {
			if !refTokenPattern.MatchString(refLike) {
				return fmt.Errorf("%w: %q", ErrCustomFFmpegBadRefToken, refLike)
			}
		}
		if !quoted[i] && shellMetacharPattern.MatchString(tok) {
			return fmt.Errorf("%w: %q", ErrCustomFFmpegBlacklisted, tok)
		}
		for _, pat := range pathLikePatterns {
			if pat.MatchString(tok) {
				return fmt.Errorf("%w: %q", ErrCustomFFmpegBlacklisted, tok)
			}
		}
	}
	return nil
}

// splitCommandTokens tokenizes cmd the way a shell would for word-splitting
// and quote removal — but only that: no variable expansion, no globbing, no
// command substitution, and (critically) no actual shell is ever invoked.
// Single and double quotes group a token that contains characters a real
// shell would otherwise need protecting (e.g. a filtergraph wrapped in
// quotes to protect parentheses/colons), and the quote characters themselves
// are stripped from the resulting token — matching how the command would
// behave if pasted into a terminal. Without this, [exec.Command] (which
// never goes through a shell) receives the literal quote characters as part
// of the argument value, which ffmpeg then rejects.
func splitCommandTokens(cmd string) ([]string, error) {
	tokens, _, err := splitCommandTokensWithQuoting(cmd)
	return tokens, err
}

// splitCommandTokensWithQuoting is [splitCommandTokens] plus a parallel
// quoted[i] flag for each tokens[i], true when every character of that
// token came from inside a quoted run (e.g. -filter_complex "..."). A token
// built from a mix of quoted and unquoted parts (e.g. foo"bar baz"qux) is
// conservatively reported as unquoted, since real ffmpeg arguments never
// need that shape — only fully-quoted arguments are exempted from
// [shellMetacharPattern] in [ValidateCustomCommand].
func splitCommandTokensWithQuoting(cmd string) ([]string, []bool, error) {
	var tokens []string
	var quoted []bool
	var b strings.Builder
	inToken := false
	fullyQuoted := true
	var quote rune

	flush := func() {
		if inToken {
			tokens = append(tokens, b.String())
			quoted = append(quoted, fullyQuoted)
			b.Reset()
			inToken = false
			fullyQuoted = true
		}
	}

	for _, r := range cmd {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			b.WriteRune(r)
			inToken = true
			fullyQuoted = false
		}
	}
	if quote != 0 {
		return nil, nil, ErrCustomFFmpegUnterminatedQuote
	}
	flush()
	return tokens, quoted, nil
}

// RunCustomFFmpeg executes a user-supplied ffmpeg command, substituting
// {input} and {output} with real temp file paths, then returns the path to
// the produced output file. The command is split via [splitCommandTokens]
// and run through [exec.Command] — never sh -c — so shell injection is not
// possible regardless of the blacklist.
//
// refs maps ref token strings (e.g. "{asset:abc123}", as produced by
// [ExtractRefTokens]) to local file paths of already-downloaded files. Pass
// nil when the command has no references — ranging over a nil map is a
// no-op.
func (t *transformer) RunCustomFFmpeg(
	ctx context.Context,
	cmd, srcPath, outDir string,
	refs map[string]string,
) (string, error) {
	if err := ValidateCustomCommand(cmd); err != nil {
		return "", err
	}
	if !t.ffmpeg.available() {
		return "", errFFmpegUnavailable(t.ffmpeg.configuredPath)
	}

	// Resolve to an absolute path: cmd.Dir is pinned to outDir below, so a
	// relative srcPath would otherwise no longer resolve against it.
	if abs, err := filepath.Abs(srcPath); err == nil {
		srcPath = abs
	}

	// MP4 is the default container: it's the most broadly browser-playable
	// format, and without an explicit -f flag ffmpeg infers the muxer from
	// the output extension. A user-supplied -f flag always takes
	// precedence over the extension, so a command needing a codec MP4
	// can't hold (e.g. subtitle streams, certain audio codecs) can still
	// force a different container — e.g. -f matroska — with no effect on
	// commands that specify their own format.
	outputPath := filepath.Join(outDir, "custom_ffmpeg_output.mp4")

	_, content := StripLeadingDescription(cmd)
	tokens, err := splitCommandTokens(content)
	if err != nil {
		return "", err
	}
	if len(tokens) > 0 && strings.EqualFold(tokens[0], "ffmpeg") {
		tokens = tokens[1:]
	}

	args := make([]string, 0, len(tokens)+1)
	hasOverwriteFlag := false
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		// Strip any user-supplied -protocol_whitelist so it can't be widened
		// to allow http/https/etc — we always force our own value below.
		if tok == "-protocol_whitelist" {
			i++
			continue
		}
		// Strip any user-supplied -safe so it falls back to the concat
		// demuxer's own default (safe=1). -safe 0 disables concat's
		// built-in rejection of absolute paths and non-file protocols in a
		// playlist — and since the playlist is the *asset's contents*, not
		// the command string, the token blacklist above can never see the
		// arbitrary paths it would let through.
		if tok == "-safe" {
			i++
			continue
		}
		for refTok, refPath := range refs {
			tok = strings.ReplaceAll(tok, refTok, refPath)
		}
		tok = strings.ReplaceAll(tok, "{input}", srcPath)
		tok = strings.ReplaceAll(tok, "{output}", outputPath)
		if tok == "-y" {
			hasOverwriteFlag = true
		}
		// Restrict ffmpeg to the file protocol so a command can't read
		// arbitrary local paths via file: or perform SSRF via
		// http(s)/tcp/rtmp/etc — the token blacklist above only catches a
		// fixed, ever-growing list of protocol prefixes, this allowlist
		// closes the gap structurally. -protocol_whitelist only applies to
		// the -i that immediately follows it, so it must precede every one.
		if tok == "-i" {
			args = append(args, "-protocol_whitelist", "file")
		}
		args = append(args, tok)
	}
	if !hasOverwriteFlag {
		args = append([]string{"-y"}, args...)
	}

	ctx, cancel := context.WithTimeout(ctx, customFFmpegTimeout)
	defer cancel()

	var stderr bytes.Buffer
	runCmd := t.ffmpeg.commandFFmpeg(ctx, args...)
	runCmd.Stderr = &stderr
	// Pin the working directory to the job's throwaway scratch dir so a bare
	// relative path in the command (which passes the blacklist) can't read
	// whatever happens to live in the server process's own cwd.
	runCmd.Dir = outDir

	if err = runCmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", ErrCustomFFmpegFailed, capStderrTail(stderr.String()))
	}

	fi, err := os.Stat(outputPath)
	if err != nil || fi.Size() == 0 {
		return "", fmt.Errorf("%w: ffmpeg produced no output", ErrCustomFFmpegFailed)
	}
	return outputPath, nil
}

// capStderrTail caps s to the last customFFmpegStderrCap bytes. ffmpeg always
// prints its (often very long) version/build-config banner first and the
// actual failure reason last, so keeping the tail — rather than the head —
// is what makes the cap preserve the part of stderr that's actually useful.
func capStderrTail(s string) string {
	if len(s) > customFFmpegStderrCap {
		return s[len(s)-customFFmpegStderrCap:]
	}
	return s
}

// ffprobeFormatMime maps an ffprobe format_name token to a MIME type.
// ffprobe's format_name is a comma-separated list of demuxer names that can
// read the container (e.g. "mov,mp4,m4a,3gp,3g2,mj2" for a plain MP4 file).
const ffprobeFormatMov = "mov"

var ffprobeFormatMime = map[string]string{
	FormatMP4:        MimeVideoMP4,
	ffprobeFormatMov: "video/quicktime",
	FormatWebM:       MimeVideoWebM,
	"matroska":       "video/x-matroska",
	"avi":            "video/x-msvideo",
	"gif":            MimeImageGIF,
	"image2":         MimeImageJPEG,
	"png_pipe":       MimeImagePNG,
	"apng":           "image/apng",
	audioFormatMP3:   audioMimeMPEG,
	audioFormatWAV:   audioMimeWAV,
	audioFormatOGG:   audioMimeOGG,
	audioFormatFLAC:  audioMimeFLAC,
}

// ffprobeFormatPriority resolves ambiguity when multiple known tokens appear
// in the same format_name list — e.g. plain MP4 files report
// "mov,mp4,m4a,3gp,3g2,mj2", and mp4 should win over the more generic mov.
// ffmpeg's matroska demuxer always reports "matroska,webm" regardless of
// whether the file is genuinely mkv or webm, and format_name alone can't
// distinguish the two — so matroska (the more common case for an
// unlabelled ambiguous output) wins over the more specific-sounding webm.
var ffprobeFormatPriority = []string{
	FormatMP4, "matroska", FormatWebM, "avi", "gif", "apng", "png_pipe", "image2",
	audioFormatMP3, audioFormatWAV, audioFormatOGG, audioFormatFLAC, ffprobeFormatMov,
}

// DetectOutputMIME runs ffprobe on outputPath to detect its MIME type.
// It never fails loudly — if ffprobe is unavailable or the format can't be
// recognised, it returns application/octet-stream and logs a warning. The
// variant is stored either way.
func (t *transformer) DetectOutputMIME(ctx context.Context, outputPath string) string {
	const fallbackMime = MimeApplicationOctetStream

	if !t.ffmpeg.ffprobeAvailable() {
		slog.WarnContext(ctx, "custom_ffmpeg: ffprobe unavailable, using fallback mime type")
		return fallbackMime
	}

	ctx, cancel := context.WithTimeout(ctx, customFFmpegProbeTimeout)
	defer cancel()

	out, err := t.ffmpeg.commandFFprobe(
		ctx, "-v", "quiet", "-print_format", "json", "-show_format", outputPath,
	).Output()
	if err != nil {
		slog.WarnContext(ctx, "custom_ffmpeg: ffprobe failed, using fallback mime type", "error", err)
		return fallbackMime
	}

	var probe struct {
		Format struct {
			FormatName string `json:"format_name"`
		} `json:"format"`
	}
	if err = json.Unmarshal(out, &probe); err != nil {
		slog.WarnContext(ctx, "custom_ffmpeg: could not parse ffprobe output, using fallback mime type", "error", err)
		return fallbackMime
	}

	tokens := make(map[string]bool)
	for name := range strings.SplitSeq(probe.Format.FormatName, ",") {
		tokens[strings.TrimSpace(name)] = true
	}
	for _, key := range ffprobeFormatPriority {
		if tokens[key] {
			return ffprobeFormatMime[key]
		}
	}
	slog.WarnContext(ctx, "custom_ffmpeg: unrecognised ffprobe format, using fallback mime type",
		"format_name", probe.Format.FormatName)
	return fallbackMime
}

// TrimToSeconds creates a stream-copy trim of src to the first n seconds,
// writing the result into outDir. It is used by the custom_ffmpeg dry-run
// (draft) flow to keep test runs fast. If trimming fails — some containers
// don't support copy-trim — it returns srcPath unchanged so the caller can
// fall back to testing against the full file rather than erroring out.
func (t *transformer) TrimToSeconds(ctx context.Context, srcPath string, n int, outDir string) (string, error) {
	if !t.ffmpeg.available() {
		return srcPath, errFFmpegUnavailable(t.ffmpeg.configuredPath)
	}

	// Resolve to an absolute path: cmd.Dir is pinned to outDir below, so a
	// relative srcPath would otherwise no longer resolve against it.
	if abs, err := filepath.Abs(srcPath); err == nil {
		srcPath = abs
	}

	dstPath := filepath.Join(outDir, "trim_"+strconv.Itoa(n)+"s"+filepath.Ext(srcPath))

	ctx, cancel := context.WithTimeout(ctx, customFFmpegTrimTimeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := t.ffmpeg.commandFFmpeg(
		ctx,
		"-protocol_whitelist",
		"file",
		"-y",
		"-i",
		srcPath,
		"-t",
		strconv.Itoa(n),
		"-c",
		"copy",
		dstPath,
	)
	cmd.Stderr = &stderr
	cmd.Dir = outDir

	if err := cmd.Run(); err != nil {
		return srcPath, fmt.Errorf("trim: %w — stderr: %s", err, stderr.String())
	}
	if fi, err := os.Stat(dstPath); err != nil || fi.Size() == 0 {
		return srcPath, errors.New("trim produced no output")
	}
	return dstPath, nil
}
