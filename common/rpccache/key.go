package cache

import (
	"strconv"
)

const (
	defaultPrefixKey = "rpccache:"
)

type Identifier struct {
	ProjectID    string
	ProjectOwner string
	ProjectSlug  string
	Version      int32
	UserID       *string
}

func (i Identifier) String() string {
	str := i.ProjectOwner + ":" + i.ProjectSlug + ":" + strconv.Itoa(int(i.Version)) + ":" + i.ProjectID
	if i.UserID != nil {
		str += ":" + *i.UserID
	}
	return str
}

type Key struct {
	Prefix     string
	UniqueID   string
	Identifier *Identifier
}

func (k Key) UniqueKey() string {
	if k.Identifier != nil {
		return k.Identifier.String() + ":" + k.UniqueID
	}
	return k.UniqueID
}

func (k Key) String() string {
	return defaultPrefixKey + k.Prefix + ":" + k.UniqueKey()
}

func (k Key) RefreshString() string {
	return defaultPrefixKey + k.Prefix + ":refresh:" + k.UniqueKey()
}

func (k Key) ConcurrencyControlString() string {
	return defaultPrefixKey + k.Prefix + ":concurrency:" + k.UniqueKey()
}
