package gonanoid

import (
	"regexp"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"sentioxyz/sentio-core/common/version"
)

var IDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz"
var IDAlphabetLowercase = "0123456789_abcdefghijklmnopqrstuvwxyz"
var IDAlphabetWithoutUnderline = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func CheckIDMatchPattern(str string, allowLeadingNumber bool, lowercaseOnly bool) bool {
	var re *regexp.Regexp

	if allowLeadingNumber {
		if lowercaseOnly {
			re = regexp.MustCompile(`(?s)^[a-z0-9_-]{1,31}$`)
		} else {
			re = regexp.MustCompile(`(?is)^[a-z0-9_-]{2,32}$`)
		}
	} else {
		if lowercaseOnly {
			re = regexp.MustCompile(`(?s)^[a-z_][a-z0-9_-]{1,31}$`)
		} else {
			re = regexp.MustCompile(`(?is)^[a-z_][a-z0-9_-]{1,31}$`)
		}
	}
	return re.MatchString(str)
}

var IDBlacklist = map[string]bool{
	"admin":         true,
	"administrator": true,
	"root":          true,
	"superuser":     true,
	"sysadmin":      true,
	"system":        true,
	"users":         true,
	"username":      true,
	"usernames":     true,
	// app first level routs
	"user":          true,
	"login":         true,
	"logout":        true,
	"signup":        true,
	"register":      true,
	"api":           true,
	"billing":       true,
	"callback":      true,
	"compilation":   true,
	"contract":      true,
	"dashboard":     true,
	"discover":      true,
	"explorer":      true,
	"favorite":      true,
	"library":       true,
	"organization":  true,
	"organizations": true,
	"profile":       true,
	"projects":      true,
	"redirect":      true,
	"share":         true,
	"sim":           true,
	"snapshots":     true,
	"tx":            true,
	"index":         true,
}

func CheckIDBlacklist(str string) bool {
	_, inBlackList := IDBlacklist[str]
	return !inBlackList
}

func Must(size int) string {
	return gonanoid.MustGenerate(IDAlphabetWithoutUnderline, size)
}

func Generate(size int) (string, error) {
	if version.IsProduction() {
		return gonanoid.Generate(IDAlphabetWithoutUnderline, size)
	} else {
		return gonanoid.MustGenerate(IDAlphabetWithoutUnderline, size), nil
	}
}

func GenerateID() (string, error) {
	return Generate(8)
}

func GenerateLongID() (string, error) {
	return Generate(12)
}

func GenerateWithAlphabet(alphabet string, size int) string {
	return gonanoid.MustGenerate(alphabet, size)
}
