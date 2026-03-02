package rewriteimpl

type DatabaseTableArgs interface {
	GetDatabase() string
	GetTable() string
}
