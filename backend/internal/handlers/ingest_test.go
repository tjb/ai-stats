package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/ingest"
)

// fakeIngestStore is an in-memory implementation of ingestQuerier for tests.
type fakeIngestStore struct {
	keys      map[string]*db.IngestAPIKey // keyed by hash
	snapshots []capturedSnapshot
	upsertErr error
}

type capturedSnapshot struct {
	orgID        string
	toolSource   string
	date         time.Time
	suggestions  int
	acceptances  int
	lines        int
	activeUsers  int
	planningUses int
	codingUses   int
}

func (f *fakeIngestStore) GetIngestKeyByHash(_ context.Context, hash string) (*db.IngestAPIKey, error) {
	k, ok := f.keys[hash]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return k, nil
}

func (f *fakeIngestStore) UpdateIngestKeyLastUsed(_ context.Context, _ string) error {
	return nil
}

func (f *fakeIngestStore) UpsertToolSnapshot(_ context.Context, orgID, toolSource string, date time.Time,
	suggestions, acceptances, lines, activeUsers, _, _ int,
	planningUses, codingUses int, _ json.RawMessage) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.snapshots = append(f.snapshots, capturedSnapshot{
		orgID: orgID, toolSource: toolSource, date: date,
		suggestions: suggestions, acceptances: acceptances, lines: lines, activeUsers: activeUsers,
		planningUses: planningUses, codingUses: codingUses,
	})
	return nil
}

// helpers

func newTestStore() *fakeIngestStore {
	rawKey, hash, _ := ingest.GenerateKey()
	_ = rawKey
	store := &fakeIngestStore{
		keys: map[string]*db.IngestAPIKey{
			hash: {ID: "key-1", OrganizationID: "org-1", KeyHash: hash},
		},
	}
	return store
}

// validKey returns a raw key and pre-seeds its hash into the given store.
func validKey(store *fakeIngestStore) string {
	raw, hash, _ := ingest.GenerateKey()
	store.keys[hash] = &db.IngestAPIKey{ID: "key-1", OrganizationID: "org-1", KeyHash: hash}
	return raw
}

func postIngest(handler *IngestHandler, body any, key string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-AIStats-Key", key)
	}
	w := httptest.NewRecorder()
	handler.PostEvents(w, req)
	return w
}

func validPayload() map[string]any {
	return map[string]any{
		"tool_source": "claude_code",
		"date":        time.Now().Format("2006-01-02"),
		"events": []map[string]any{
			{
				"type":             "tool_use",
				"tool_name":        "Edit",
				"lines_written":    10,
				"duration_minutes": 5,
				"user_email":       "dev@example.com",
			},
		},
	}
}

// ---- Tests ----

func TestIngest_MissingKey(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	w := postIngest(h, validPayload(), "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestIngest_InvalidKey(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	w := postIngest(h, validPayload(), "aistats_live_notavalidkey")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestIngest_ValidRequest(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	w := postIngest(h, validPayload(), key)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200; body: %s", w.Code, w.Body)
	}
	if len(store.snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(store.snapshots))
	}
	snap := store.snapshots[0]
	if snap.toolSource != "claude_code" {
		t.Errorf("tool_source = %q, want claude_code", snap.toolSource)
	}
	if snap.lines != 10 {
		t.Errorf("lines = %d, want 10", snap.lines)
	}
}

func TestIngest_BearerTokenAuth(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	raw, hash, _ := ingest.GenerateKey()
	store.keys[hash] = &db.IngestAPIKey{ID: "key-2", OrganizationID: "org-1", KeyHash: hash}

	b, _ := json.Marshal(validPayload())
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	h.PostEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestIngest_MissingToolSource(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	delete(payload, "tool_source")
	w := postIngest(h, payload, key)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestIngest_EmptyEvents(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []any{}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestIngest_TooManyEvents(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	events := make([]map[string]any, 1001)
	for i := range events {
		events[i] = map[string]any{"type": "session_end", "lines_written": 1, "duration_minutes": 1, "user_email": "x@x.com"}
	}
	payload := validPayload()
	payload["events"] = events
	w := postIngest(h, payload, key)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got %d, want 413", w.Code)
	}
}

func TestIngest_InvalidDate(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["date"] = "not-a-date"
	w := postIngest(h, payload, key)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestIngest_LinesWrittenOutOfRange(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "session_end", "lines_written": 200000, "duration_minutes": 1, "user_email": "x@x.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestIngest_DurationOutOfRange(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "session_end", "lines_written": 1, "duration_minutes": 9999, "user_email": "x@x.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestIngest_DistinctUserCounting(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "session_end", "lines_written": 1, "duration_minutes": 1, "user_email": "alice@example.com"},
		{"type": "session_end", "lines_written": 1, "duration_minutes": 1, "user_email": "alice@example.com"}, // duplicate
		{"type": "session_end", "lines_written": 1, "duration_minutes": 1, "user_email": "bob@example.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	if store.snapshots[0].activeUsers != 2 {
		t.Errorf("activeUsers = %d, want 2 (alice deduped)", store.snapshots[0].activeUsers)
	}
}

func TestIngest_NonSessionEndEventsSkipped(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "session_start", "lines_written": 0, "duration_minutes": 0, "user_email": "x@x.com"},
		{"type": "session_end", "lines_written": 7, "duration_minutes": 3, "user_email": "x@x.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	if store.snapshots[0].lines != 7 {
		t.Errorf("lines = %d, want 7 (session_start should be ignored)", store.snapshots[0].lines)
	}
}

func TestIngest_EditRateToolUse(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	// 4 tool uses: 2 edits, 2 reads — expect 50% edit rate
	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "tool_use", "tool_name": "Edit", "lines_written": 5, "duration_minutes": 1, "user_email": "a@example.com"},
		{"type": "tool_use", "tool_name": "Write", "lines_written": 3, "duration_minutes": 1, "user_email": "a@example.com"},
		{"type": "tool_use", "tool_name": "Read", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
		{"type": "tool_use", "tool_name": "Bash", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body: %s", w.Code, w.Body)
	}
	snap := store.snapshots[0]
	if snap.acceptances != 2 {
		t.Errorf("acceptances = %d, want 2 (Edit + Write only)", snap.acceptances)
	}
	if store.snapshots[0].lines != 8 {
		t.Errorf("lines = %d, want 8 (5+3)", store.snapshots[0].lines)
	}
}

func TestIngest_NonEditToolsNotAccepted(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "tool_use", "tool_name": "Read", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
		{"type": "tool_use", "tool_name": "Bash", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
		{"type": "tool_use", "tool_name": "Glob", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	snap := store.snapshots[0]
	if snap.acceptances != 0 {
		t.Errorf("acceptances = %d, want 0 (no edit tools)", snap.acceptances)
	}
	// suggestions should still count all tool uses
	if snap.acceptances > snap.lines {
		t.Errorf("acceptances (%d) should not exceed suggestions for read-only session", snap.acceptances)
	}
}

func TestIngest_MultipleSessionsAccumulate(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "session_end", "lines_written": 10, "duration_minutes": 5, "user_email": "a@example.com"},
		{"type": "session_end", "lines_written": 20, "duration_minutes": 10, "user_email": "b@example.com"},
		{"type": "session_end", "lines_written": 5, "duration_minutes": 2, "user_email": "c@example.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	snap := store.snapshots[0]
	if snap.lines != 35 {
		t.Errorf("lines = %d, want 35 (10+20+5)", snap.lines)
	}
	if snap.acceptances != 3 {
		t.Errorf("acceptances = %d, want 3 (one per session_end)", snap.acceptances)
	}
	if snap.activeUsers != 3 {
		t.Errorf("activeUsers = %d, want 3 (three distinct users)", snap.activeUsers)
	}
}

func TestIngest_EmptyUserEmailAllowed(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "session_end", "lines_written": 5, "duration_minutes": 1, "user_email": ""},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body: %s", w.Code, w.Body)
	}
	if store.snapshots[0].activeUsers != 0 {
		t.Errorf("activeUsers = %d, want 0 (no email = not counted)", store.snapshots[0].activeUsers)
	}
}

func TestPlanningToolsClassified(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "tool_use", "tool_name": "Read", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
		{"type": "tool_use", "tool_name": "Grep", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
		{"type": "tool_use", "tool_name": "Bash", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body: %s", w.Code, w.Body)
	}
	snap := store.snapshots[0]
	if snap.planningUses != 3 {
		t.Errorf("planningUses = %d, want 3", snap.planningUses)
	}
	if snap.codingUses != 0 {
		t.Errorf("codingUses = %d, want 0", snap.codingUses)
	}
}

func TestCodingToolsClassified(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "tool_use", "tool_name": "Edit", "lines_written": 5, "duration_minutes": 1, "user_email": "a@example.com"},
		{"type": "tool_use", "tool_name": "Write", "lines_written": 3, "duration_minutes": 1, "user_email": "a@example.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body: %s", w.Code, w.Body)
	}
	snap := store.snapshots[0]
	if snap.planningUses != 0 {
		t.Errorf("planningUses = %d, want 0", snap.planningUses)
	}
	if snap.codingUses != 2 {
		t.Errorf("codingUses = %d, want 2", snap.codingUses)
	}
}

func TestUnknownToolCountsAsPlanning(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "tool_use", "tool_name": "SomeFutureTool", "lines_written": 0, "duration_minutes": 0, "user_email": "a@example.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body: %s", w.Code, w.Body)
	}
	snap := store.snapshots[0]
	if snap.planningUses != 1 {
		t.Errorf("planningUses = %d, want 1 (unknown tool counted as planning)", snap.planningUses)
	}
	if snap.codingUses != 0 {
		t.Errorf("codingUses = %d, want 0", snap.codingUses)
	}
}

func TestSessionEndDoesNotCountPlanningOrCoding(t *testing.T) {
	store := newTestStore()
	h := &IngestHandler{queries: store}
	key := validKey(store)

	payload := validPayload()
	payload["events"] = []map[string]any{
		{"type": "session_end", "lines_written": 5, "duration_minutes": 1, "user_email": "a@example.com"},
	}
	w := postIngest(h, payload, key)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body: %s", w.Code, w.Body)
	}
	snap := store.snapshots[0]
	if snap.planningUses != 0 {
		t.Errorf("planningUses = %d, want 0 (session_end must not count)", snap.planningUses)
	}
	if snap.codingUses != 0 {
		t.Errorf("codingUses = %d, want 0 (session_end must not count)", snap.codingUses)
	}
}
