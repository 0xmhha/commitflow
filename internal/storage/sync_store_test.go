package storage

import (
	"context"
	"testing"
)

func TestSyncStore_SaveAndGetConfig(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{
		Name:           "geth",
		UpstreamURL:    "https://github.com/ethereum/go-ethereum",
		ForkPath:       "/path/to/fork",
		LastSyncedHash: "abc123",
	}

	if err := store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if cfg.ID == 0 {
		t.Error("SaveConfig should set ID on the config")
	}

	got, err := store.GetConfig(ctx, "geth")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if got == nil {
		t.Fatal("GetConfig returned nil")
	}
	if got.Name != "geth" {
		t.Errorf("Name = %q, want %q", got.Name, "geth")
	}
	if got.UpstreamURL != cfg.UpstreamURL {
		t.Errorf("UpstreamURL = %q, want %q", got.UpstreamURL, cfg.UpstreamURL)
	}
	if got.ForkPath != cfg.ForkPath {
		t.Errorf("ForkPath = %q, want %q", got.ForkPath, cfg.ForkPath)
	}
	if got.LastSyncedHash != "abc123" {
		t.Errorf("LastSyncedHash = %q, want %q", got.LastSyncedHash, "abc123")
	}
}

func TestSyncStore_GetConfig_NotFound(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	got, err := store.GetConfig(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if got != nil {
		t.Error("GetConfig for nonexistent should return nil")
	}
}

func TestSyncStore_GetConfigByID(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{
		Name:        "test",
		UpstreamURL: "https://example.com/repo",
		ForkPath:    "/fork",
	}
	if err := store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := store.GetConfigByID(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("GetConfigByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetConfigByID returned nil")
	}
	if got.Name != "test" {
		t.Errorf("Name = %q, want %q", got.Name, "test")
	}
}

func TestSyncStore_ListConfigs(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	configs := []*UpstreamConfig{
		{Name: "a", UpstreamURL: "url_a", ForkPath: "/a"},
		{Name: "b", UpstreamURL: "url_b", ForkPath: "/b"},
	}
	for _, c := range configs {
		if err := store.SaveConfig(ctx, c); err != nil {
			t.Fatalf("SaveConfig: %v", err)
		}
	}

	list, err := store.ListConfigs(ctx)
	if err != nil {
		t.Fatalf("ListConfigs: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListConfigs returned %d items, want 2", len(list))
	}
}

func TestSyncStore_UpdateLastSyncedHash(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "test", UpstreamURL: "url", ForkPath: "/fork", LastSyncedHash: "old"}
	if err := store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if err := store.UpdateLastSyncedHash(ctx, cfg.ID, "newhash"); err != nil {
		t.Fatalf("UpdateLastSyncedHash: %v", err)
	}

	got, _ := store.GetConfig(ctx, "test")
	if got.LastSyncedHash != "newhash" {
		t.Errorf("LastSyncedHash = %q, want %q", got.LastSyncedHash, "newhash")
	}
}

func TestSyncStore_SaveAndGetSync(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "test", UpstreamURL: "url", ForkPath: "/fork"}
	if err := store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	sync := &UpstreamSync{
		ConfigID:            cfg.ID,
		UpstreamCommit:      "commit123",
		Status:              "pending",
		ApplicabilityReason: "test reason",
		RelevanceScore:      7,
	}

	if err := store.SaveSync(ctx, sync); err != nil {
		t.Fatalf("SaveSync: %v", err)
	}

	got, err := store.GetSync(ctx, cfg.ID, "commit123")
	if err != nil {
		t.Fatalf("GetSync: %v", err)
	}
	if got == nil {
		t.Fatal("GetSync returned nil")
	}
	if got.Status != "pending" {
		t.Errorf("Status = %q, want %q", got.Status, "pending")
	}
	if got.RelevanceScore != 7 {
		t.Errorf("RelevanceScore = %d, want 7", got.RelevanceScore)
	}
}

func TestSyncStore_IsSynced(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "test", UpstreamURL: "url", ForkPath: "/fork"}
	if err := store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	synced, err := store.IsSynced(ctx, cfg.ID, "commit123")
	if err != nil {
		t.Fatalf("IsSynced: %v", err)
	}
	if synced {
		t.Error("IsSynced before save = true, want false")
	}

	sync := &UpstreamSync{ConfigID: cfg.ID, UpstreamCommit: "commit123", Status: "pending"}
	if err := store.SaveSync(ctx, sync); err != nil {
		t.Fatalf("SaveSync: %v", err)
	}

	synced, err = store.IsSynced(ctx, cfg.ID, "commit123")
	if err != nil {
		t.Fatalf("IsSynced after save: %v", err)
	}
	if !synced {
		t.Error("IsSynced after save = false, want true")
	}
}

func TestSyncStore_UpdateSyncStatus(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "test", UpstreamURL: "url", ForkPath: "/fork"}
	store.SaveConfig(ctx, cfg)

	sync := &UpstreamSync{ConfigID: cfg.ID, UpstreamCommit: "c1", Status: "pending"}
	store.SaveSync(ctx, sync)

	if err := store.UpdateSyncStatus(ctx, sync.ID, "applicable", "looks good"); err != nil {
		t.Fatalf("UpdateSyncStatus: %v", err)
	}

	got, _ := store.GetSync(ctx, cfg.ID, "c1")
	if got.Status != "applicable" {
		t.Errorf("Status = %q, want %q", got.Status, "applicable")
	}
	if got.ApplicabilityReason != "looks good" {
		t.Errorf("Reason = %q, want %q", got.ApplicabilityReason, "looks good")
	}
}

func TestSyncStore_MarkApplied(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "test", UpstreamURL: "url", ForkPath: "/fork"}
	store.SaveConfig(ctx, cfg)

	sync := &UpstreamSync{ConfigID: cfg.ID, UpstreamCommit: "c1", Status: "applicable"}
	store.SaveSync(ctx, sync)

	if err := store.MarkApplied(ctx, sync.ID, "newcommit456"); err != nil {
		t.Fatalf("MarkApplied: %v", err)
	}

	got, _ := store.GetSync(ctx, cfg.ID, "c1")
	if got.Status != "applied" {
		t.Errorf("Status = %q, want %q", got.Status, "applied")
	}
	if got.AppliedCommit != "newcommit456" {
		t.Errorf("AppliedCommit = %q, want %q", got.AppliedCommit, "newcommit456")
	}
	if got.AppliedAt == "" {
		t.Error("AppliedAt should be set after MarkApplied")
	}
}

func TestSyncStore_MarkSkipped(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "test", UpstreamURL: "url", ForkPath: "/fork"}
	store.SaveConfig(ctx, cfg)

	sync := &UpstreamSync{ConfigID: cfg.ID, UpstreamCommit: "c1", Status: "pending"}
	store.SaveSync(ctx, sync)

	if err := store.MarkSkipped(ctx, sync.ID, "not relevant"); err != nil {
		t.Fatalf("MarkSkipped: %v", err)
	}

	got, _ := store.GetSync(ctx, cfg.ID, "c1")
	if got.Status != "skipped" {
		t.Errorf("Status = %q, want %q", got.Status, "skipped")
	}
	if got.SkipReason != "not relevant" {
		t.Errorf("SkipReason = %q, want %q", got.SkipReason, "not relevant")
	}
}

func TestSyncStore_CountByStatus(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "test", UpstreamURL: "url", ForkPath: "/fork"}
	store.SaveConfig(ctx, cfg)

	syncs := []*UpstreamSync{
		{ConfigID: cfg.ID, UpstreamCommit: "c1", Status: "pending"},
		{ConfigID: cfg.ID, UpstreamCommit: "c2", Status: "applicable"},
		{ConfigID: cfg.ID, UpstreamCommit: "c3", Status: "applicable"},
		{ConfigID: cfg.ID, UpstreamCommit: "c4", Status: "applied"},
	}
	for _, s := range syncs {
		store.SaveSync(ctx, s)
	}

	counts, err := store.CountByStatus(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}

	if counts["pending"] != 1 {
		t.Errorf("pending = %d, want 1", counts["pending"])
	}
	if counts["applicable"] != 2 {
		t.Errorf("applicable = %d, want 2", counts["applicable"])
	}
	if counts["applied"] != 1 {
		t.Errorf("applied = %d, want 1", counts["applied"])
	}
}

func TestSyncStore_ListSyncs_WithFilter(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "test", UpstreamURL: "url", ForkPath: "/fork"}
	store.SaveConfig(ctx, cfg)

	syncs := []*UpstreamSync{
		{ConfigID: cfg.ID, UpstreamCommit: "c1", Status: "pending"},
		{ConfigID: cfg.ID, UpstreamCommit: "c2", Status: "applicable"},
		{ConfigID: cfg.ID, UpstreamCommit: "c3", Status: "applicable"},
	}
	for _, s := range syncs {
		store.SaveSync(ctx, s)
	}

	// Filter by status.
	results, err := store.ListSyncs(ctx, SyncFilter{ConfigID: cfg.ID, Status: "applicable"})
	if err != nil {
		t.Fatalf("ListSyncs: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListSyncs with status filter returned %d items, want 2", len(results))
	}

	// With limit.
	results, err = store.ListSyncs(ctx, SyncFilter{ConfigID: cfg.ID, Limit: 1})
	if err != nil {
		t.Fatalf("ListSyncs with limit: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("ListSyncs with limit returned %d items, want 1", len(results))
	}
}

func TestSyncStore_SaveCallLog(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	log := &CallLog{
		CallType:     "analysis",
		CommitHash:   "abc123",
		Model:        "sonnet",
		InputTokens:  1000,
		OutputTokens: 200,
		CostUSD:      0.05,
		DurationMS:   3000,
	}

	if err := store.SaveCallLog(ctx, log); err != nil {
		t.Fatalf("SaveCallLog: %v", err)
	}

	// Verify the record was inserted.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM ai_call_log").Scan(&count)
	if err != nil {
		t.Fatalf("count ai_call_log: %v", err)
	}
	if count != 1 {
		t.Errorf("ai_call_log count = %d, want 1", count)
	}
}

func TestSyncStore_SaveConfig_Upsert(t *testing.T) {
	db := testDB(t)
	store := NewSyncStore(db)
	ctx := context.Background()

	cfg := &UpstreamConfig{Name: "geth", UpstreamURL: "url1", ForkPath: "/fork1"}
	if err := store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("first SaveConfig: %v", err)
	}

	cfg2 := &UpstreamConfig{Name: "geth", UpstreamURL: "url2", ForkPath: "/fork2"}
	if err := store.SaveConfig(ctx, cfg2); err != nil {
		t.Fatalf("second SaveConfig (upsert): %v", err)
	}

	got, _ := store.GetConfig(ctx, "geth")
	if got.UpstreamURL != "url2" {
		t.Errorf("UpstreamURL after upsert = %q, want %q", got.UpstreamURL, "url2")
	}
}
