package chliteral

import "testing"

func TestString(t *testing.T) {
	for in, want := range map[string]string{
		"":                 `''`,
		"plain":            `'plain'`,
		"Trader Joe's":     `'Trader Joe\'s'`,
		`a\`:               `'a\\'`,
		`a\' OR 1=1 --`:    `'a\\\' OR 1=1 --'`,
		"x') OR (SELECT 1": `'x\') OR (SELECT 1'`,
		"100%_done":        `'100%_done'`,
	} {
		if got := String(in); got != want {
			t.Errorf("String(%q) = %s; want %s", in, got, want)
		}
	}
}

func TestIdentifier(t *testing.T) {
	for in, want := range map[string]string{
		"amount":            "`amount`",
		"a`, (SELECT 1), `": "`a\\`, (SELECT 1), \\``",
		`a\`:                "`a\\\\`",
	} {
		if got := Identifier(in); got != want {
			t.Errorf("Identifier(%q) = %s; want %s", in, got, want)
		}
	}
}

func TestIsNumber(t *testing.T) {
	for _, s := range []string{"0", "42", "-1", "+3.5", "1.", ".5", "1e18", "2.5E-3",
		"115792089237316195423570985008687907853269984665640564039457584007913129639935"} {
		if !IsNumber(s) {
			t.Errorf("IsNumber(%q) = false; want true", s)
		}
	}
	for _, s := range []string{"", " 1", "1 ", "1 OR 1", "1)", "(SELECT 1)", "0x10", "1e", "--1", "NaN", "1;", "1/**/"} {
		if IsNumber(s) {
			t.Errorf("IsNumber(%q) = true; want false", s)
		}
	}
}
