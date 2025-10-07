package version

var Version = "local"
var CommitSha = "head"
var BuildTimestamp = "<na>"

func IsProduction() bool {
	return Version != "local"
}
