package rewriteimpl

type CommonTableExpression interface {
	GetAlias() string
	GetSql() string
}
