package clickhouse

import "crypto/sha256"

// idKey is what an idSet stores for an entity ID: the SHA-256 digest of the ID.
type idKey = [sha256.Size]byte

// idSet is the set of entity IDs behind the full-ID cache. It keeps the SHA-256 digest of each
// ID instead of the ID: the caches of a large processor hold tens of millions of IDs, most of
// them 50 to 70 byte hex strings, and a fixed 32-byte key with no string header and no separate
// heap allocation takes about a quarter of the memory per ID. SHA-256 is collision resistant, so
// two IDs sharing a key is not a practical concern (the assumption git makes for its object IDs)
// and no membership answer is qualified by it.
//
// An idSet is not safe for concurrent use; ChainStore guards its sets with mu.
type idSet struct {
	keys map[idKey]struct{}
}

func newIDSet(ids ...string) *idSet {
	s := &idSet{keys: make(map[idKey]struct{}, len(ids))}
	for _, id := range ids {
		s.Add(id)
	}
	return s
}

func idKeyOf(id string) idKey {
	return sha256.Sum256([]byte(id))
}

func (s *idSet) Add(id string) {
	s.keys[idKeyOf(id)] = struct{}{}
}

func (s *idSet) Remove(id string) {
	delete(s.keys, idKeyOf(id))
}

func (s *idSet) Contains(id string) bool {
	_, ok := s.keys[idKeyOf(id)]
	return ok
}

func (s *idSet) Size() int {
	return len(s.keys)
}

// Merge adds every ID of other to s.
func (s *idSet) Merge(other *idSet) {
	for key := range other.keys {
		s.keys[key] = struct{}{}
	}
}
