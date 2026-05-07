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

// TestPlainState_Clone_Isolation guards against shallow-clone regressions:
// every reference field reachable from PlainState must be independent in the
// clone so handler mutations on a working copy never bleed back into the
// source state. If a future change adds a slice/map/pointer field to any of
// the State value types and forgets to extend Clone, this test fails.
func TestPlainState_Clone_Isolation(t *testing.T) {
	src := &PlainState{
		LastBlock: 100,
		ProcessorAllocations: map[string]map[uint64]ProcessorAllocation{
			"proc_a": {1: {ProcessorId: "proc_a", IndexerId: 1}},
		},
		ProcessorInfos: map[string]ProcessorInfo{
			"proc_a": {ProcessorId: "proc_a", EntitySchema: "schema-a"},
		},
		IndexerInfos: map[uint64]IndexerInfo{
			1: {IndexerId: 1, IndexerUrl: "host-1"},
		},
		HostedProcessors: map[string]bool{"proc_a": true},
		Databases: map[string]DatabaseInfo{
			"db_a": {
				DatabaseId: "db_a",
				Tables:     []TableInfo{{TableId: "t1", TableType: "entity"}},
			},
		},
		DatabasePermissions: map[string]map[string]string{
			"0xalice": {"db_a": "5"},
		},
	}

	clone := src.Clone()
	ctx := context.Background()

	// Mutate every collection on the clone via the public methods that
	// handlers actually use. The src snapshot taken before mutations must
	// remain bit-for-bit identical afterwards.
	_ = clone.UpdateLastBlock(ctx, 200)
	_ = clone.DeleteProcessorAllocation(ctx, "proc_a", 1)
	_ = clone.UpsertProcessorInfo(ctx, ProcessorInfo{ProcessorId: "proc_a", EntitySchema: "schema-b"})
	_ = clone.DeleteIndexerInfo(ctx, 1)
	_ = clone.DeleteHostedProcessor(ctx, "proc_a")
	_ = clone.UpsertDatabaseTable(ctx, "db_a", TableInfo{TableId: "t2", TableType: "event"})
	_ = clone.SetDatabasePermission(ctx, "0xalice", "db_a", "9")

	if src.LastBlock != 100 {
		t.Errorf("src.LastBlock leaked: got %d", src.LastBlock)
	}
	if _, ok := src.ProcessorAllocations["proc_a"][1]; !ok {
		t.Errorf("src.ProcessorAllocations[proc_a][1] dropped")
	}
	if got := src.ProcessorInfos["proc_a"].EntitySchema; got != "schema-a" {
		t.Errorf("src.ProcessorInfos[proc_a].EntitySchema = %q, want schema-a", got)
	}
	if _, ok := src.IndexerInfos[1]; !ok {
		t.Errorf("src.IndexerInfos[1] dropped")
	}
	if !src.HostedProcessors["proc_a"] {
		t.Errorf("src.HostedProcessors[proc_a] cleared")
	}
	if got := src.Databases["db_a"].Tables; len(got) != 1 || got[0].TableId != "t1" {
		t.Errorf("src.Databases[db_a].Tables = %v, want [{t1, entity}]", got)
	}
	if got := src.DatabasePermissions["0xalice"]["db_a"]; got != "5" {
		t.Errorf("src.DatabasePermissions[0xalice][db_a] = %q, want 5", got)
	}
}
