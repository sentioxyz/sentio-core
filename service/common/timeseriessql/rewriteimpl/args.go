package rewriteimpl

type Args interface {
	GetCommonTableExpressions() map[string]CommonTableExpression
	GetDatabaseTables() map[string]DatabaseTableArgs
	GetRemotes() map[string]RemoteArgs
	Merge(Args)
}
