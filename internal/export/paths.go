package export

import (
	"path"
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts s into a lowercase dash-separated slug.
// If the result is empty (e.g. all special chars), returns "file".
func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "file"
	}
	return s
}

// PathRegistry tracks all resolved paths in a run to detect collisions.
// Paths are case-insensitive for collision purposes.
type PathRegistry struct {
	seen map[string]int // normalized path → collision count
}

// NewPathRegistry creates a new PathRegistry.
func NewPathRegistry() *PathRegistry {
	return &PathRegistry{seen: map[string]int{}}
}

// Resolve returns a unique, collision-safe path for a file in the archive.
//
// - projectName: the project name (will be slugified as first path segment)
// - folderName: the folder name, or "" for root (renders as "(root)")
// - stem: the file stem (without extension)
// - ext: the file extension including dot (e.g. ".jpg"), or ""
// - versionSuffix: "" for current-only, "__vN" for historical
// - variantSlug: "" for original file, "__slug" for a variant.
func (r *PathRegistry) Resolve(projectName, folderName, stem, ext, versionSuffix, variantSlug string) string {
	projectSeg := slugify(projectName)
	var folderSeg string
	if folderName == "" {
		folderSeg = "(root)"
	} else {
		folderSeg = folderName
	}

	// Build the base filename without collision suffix.
	baseName := slugify(stem) + versionSuffix + variantSlug + ext
	base := path.Join(projectSeg, folderSeg, baseName)
	key := strings.ToLower(base)

	count := r.seen[key]
	r.seen[key]++

	if count == 0 {
		return base
	}
	// Collision: insert numeric suffix before extension.
	// e.g. "proj/folder/file__v2.jpg" → "proj/folder/file__v2__2.jpg"
	dir := path.Dir(base)
	filename := path.Base(base)
	dotIdx := strings.LastIndex(filename, ".")
	var newFilename string
	if dotIdx >= 0 {
		newFilename = filename[:dotIdx] + "__" + itoa(count+1) + filename[dotIdx:]
	} else {
		newFilename = filename + "__" + itoa(count+1)
	}
	deduped := path.Join(dir, newFilename)

	// Also record the deduped path to prevent future triple collisions.
	dedupedKey := strings.ToLower(deduped)
	r.seen[dedupedKey]++

	return deduped
}

// itoa converts an int to a string without importing strconv everywhere.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
