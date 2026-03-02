package converter

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

type Label interface {
	Hash() string
	Tags() map[string]string
	ToProto() map[string]string
}

type label struct {
	tags map[string]string
	hash string
}

// labelHash returns a hash of the given tags.
// If the number of tags is less than or equal to 5, return uncompressed string,
// otherwise use sha256 to compress the tags.
func labelHash(tags map[string]string) string {
	if len(tags) == 0 {
		return "<empty>"
	}
	const compressionThreshold = 5
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) <= compressionThreshold {
		var str string
		for _, k := range keys {
			str += k + tags[k]
		}
		return str
	}
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(tags[k]))
	}
	bytes := h.Sum(nil)
	return hex.EncodeToString(bytes)
}

func NewLabel(tags map[string]string) Label {
	return &label{
		tags: tags,
		hash: labelHash(tags),
	}
}

func (l *label) Hash() string {
	return l.hash
}

func (l *label) Tags() map[string]string {
	return l.tags
}

func (l *label) ToProto() map[string]string {
	return l.tags
}
