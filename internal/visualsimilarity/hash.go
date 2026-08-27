// Package visualsimilarity computes perceptual hashes for visual deduplication.
package visualsimilarity

import (
	"encoding/json"
	"image"

	"github.com/vitali-fedulov/imagehash2"
	"github.com/vitali-fedulov/images4"
)

const (
	numBuckets = 4
	epsilon    = 0.25
)

// Hashes holds the values to persist for one image.
type Hashes struct {
	CentralHash uint64
	HashSet     []uint64
}

// Compute derives perceptual hashes from a decoded image.
func Compute(img image.Image) (Hashes, error) {
	icon := images4.Icon(img)
	central := imagehash2.CentralHash9(icon, epsilon, numBuckets)
	set := imagehash2.HashSet9(icon, epsilon, numBuckets)
	return Hashes{CentralHash: central, HashSet: set}, nil
}

// MarshalHashSet serialises a hash set to a compact JSON string for storage.
func MarshalHashSet(set []uint64) (string, error) {
	b, err := json.Marshal(set)
	return string(b), err
}

// UnmarshalHashSet deserialises a stored hash set back to []uint64.
func UnmarshalHashSet(s string) ([]uint64, error) {
	var set []uint64
	return set, json.Unmarshal([]byte(s), &set)
}
