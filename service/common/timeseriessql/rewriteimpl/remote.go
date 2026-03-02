package rewriteimpl

type RemoteArgs interface {
	GetAddr() string
	GetDatabase() string
	GetTable() string
	GetUser() string
	GetPassword() string
}
