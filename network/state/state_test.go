package state

import (
	"context"
	"testing"
)

func TestPlainState_DeleteDatabase_CascadesPermissions(t *testing.T) {
	s := &PlainState{
		Databases: map[string]DatabaseInfo{
			"db_keep":   {DatabaseId: "db_keep"},
			"db_delete": {DatabaseId: "db_delete"},
		},
		DatabasePermissions: map[string]map[string]string{
			"0xalice": {"db_keep": "1", "db_delete": "5"},
			"0xbob":   {"db_delete": "2"},
			"0xcarol": {"db_keep": "8"},
		},
	}

	if err := s.DeleteDatabase(context.Background(), "db_delete"); err != nil {
		t.Fatalf("DeleteDatabase: %v", err)
	}

	if _, ok := s.Databases["db_delete"]; ok {
		t.Errorf("Databases still contains db_delete")
	}
	if _, ok := s.Databases["db_keep"]; !ok {
		t.Errorf("Databases lost db_keep")
	}

	if got := s.DatabasePermissions["0xalice"]; len(got) != 1 || got["db_keep"] != "1" {
		t.Errorf("alice perms = %v, want only {db_keep: 1}", got)
	}
	if _, ok := s.DatabasePermissions["0xbob"]; ok {
		t.Errorf("bob's account entry should be removed once its only perm is gone, got %v", s.DatabasePermissions["0xbob"])
	}
	if got := s.DatabasePermissions["0xcarol"]; len(got) != 1 || got["db_keep"] != "8" {
		t.Errorf("carol perms = %v, want only {db_keep: 8}", got)
	}
}

func TestPlainState_DeleteDatabase_NoPermissions(t *testing.T) {
	s := &PlainState{
		Databases: map[string]DatabaseInfo{
			"db_delete": {DatabaseId: "db_delete"},
		},
		DatabasePermissions: map[string]map[string]string{},
	}

	if err := s.DeleteDatabase(context.Background(), "db_delete"); err != nil {
		t.Fatalf("DeleteDatabase: %v", err)
	}
	if _, ok := s.Databases["db_delete"]; ok {
		t.Errorf("db_delete not removed from Databases")
	}
}
