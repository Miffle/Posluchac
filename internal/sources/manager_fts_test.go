package sources

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func damerauLevenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}

	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = d[i-1][j] + 1 // deletion
			if d[i][j-1]+1 < d[i][j] {
				d[i][j] = d[i][j-1] + 1 // insertion
			}
			if d[i-1][j-1]+cost < d[i][j] {
				d[i][j] = d[i-1][j-1] + cost // substitution
			}
			// transposition
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				if d[i-2][j-2]+1 < d[i][j] {
					d[i][j] = d[i-2][j-2] + 1
				}
			}
		}
	}
	return d[la][lb]
}

func TestDamerauLevenshtein(t *testing.T) {
	cases := []struct {
		w1, w2 string
		dist   int
	}{
		{"ramstein", "rammstein", 1},
		{"linken", "linkin", 1},
		{"nrivana", "nirvana", 1}, // transposition
		{"металика", "металлика", 1},
		{"floid", "floyd", 1},
		{"zepelin", "zeppelin", 1},
		{"quen", "queen", 1},
		{"metallica", "metallica", 0},
	}

	for _, c := range cases {
		r1 := []rune(c.w1)
		r2 := []rune(c.w2)
		res := damerauLevenshtein(r1, r2)
		if res != c.dist {
			t.Errorf("damerauLevenshtein(%s, %s) = %d, expected %d", c.w1, c.w2, res, c.dist)
		}
	}
}

func TestFTS5VocabAndTrigram(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "posluchac-vocab-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE items (id INTEGER PRIMARY KEY, title TEXT);
		CREATE VIRTUAL TABLE items_fts USING fts5(title, tokenize='unicode61');
		INSERT INTO items_fts(rowid, title) VALUES (1, 'Rammstein - Mutter'), (2, 'Linkin Park - Meteora'), (3, 'Металлика - Master of Puppets');
	`)
	if err != nil {
		t.Fatalf("fts5 create failed: %v", err)
	}

	// Test fts5vocab
	_, err = db.Exec(`CREATE VIRTUAL TABLE items_vocab USING fts5vocab(items_fts, 'row');`)
	if err != nil {
		t.Logf("fts5vocab not supported: %v", err)
	} else {
		t.Logf("fts5vocab supported!")
		rows, err := db.Query(`SELECT term, cnt FROM items_vocab`)
		if err == nil {
			for rows.Next() {
				var term string
				var cnt int
				_ = rows.Scan(&term, &cnt)
				t.Logf("Vocab term: %s (count: %d)", term, cnt)
			}
			rows.Close()
		}
	}

	// Test trigram
	_, err = db.Exec(`CREATE VIRTUAL TABLE items_tri USING fts5(title, tokenize='trigram');`)
	if err != nil {
		t.Logf("trigram not supported: %v", err)
	} else {
		t.Logf("trigram supported!")
	}
}

func TestFTS5Support(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "posluchac-fts-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 1. Create base table
	_, err = db.Exec(`
		CREATE TABLE source_downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			magnet_uri TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to create source_downloads: %v", err)
	}

	// 2. Create FTS5 external content table
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS source_downloads_fts USING fts5(
			title,
			content='source_downloads',
			content_rowid='id',
			tokenize='unicode61'
		);
	`)
	if err != nil {
		t.Fatalf("failed to create fts5 table: %v", err)
	}

	// 3. Create triggers
	triggers := `
		CREATE TRIGGER IF NOT EXISTS source_downloads_ai AFTER INSERT ON source_downloads BEGIN
			INSERT INTO source_downloads_fts(rowid, title) VALUES (new.id, new.title);
		END;

		CREATE TRIGGER IF NOT EXISTS source_downloads_ad AFTER DELETE ON source_downloads BEGIN
			INSERT INTO source_downloads_fts(source_downloads_fts, rowid, title) VALUES('delete', old.id, old.title);
		END;

		CREATE TRIGGER IF NOT EXISTS source_downloads_au AFTER UPDATE ON source_downloads BEGIN
			INSERT INTO source_downloads_fts(source_downloads_fts, rowid, title) VALUES('delete', old.id, old.title);
			INSERT INTO source_downloads_fts(rowid, title) VALUES (new.id, new.title);
		END;
	`
	if _, err := db.Exec(triggers); err != nil {
		t.Fatalf("failed to create triggers: %v", err)
	}

	// 4. Insert test data
	testTitles := []string{
		"Pink Floyd - The Dark Side of the Moon [FLAC] 24/96",
		"Pink Floyd - Wish You Were Here (1975) [MP3 320 kbps]",
		"Linkin Park - Meteora (2003) FLAC Lossless",
		"Linkin Park - Hybrid Theory [MP3 320]",
		"Кино - Группа крови [Lossless, FLAC]",
		"Кино - Звезда по имени Солнце (MP3, 320 kbps)",
	}

	for _, title := range testTitles {
		_, err := db.Exec(`INSERT INTO source_downloads (source_id, title, magnet_uri) VALUES (?, ?, ?)`, 1, title, "magnet:?xt=urn:btih:dummy")
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// 5. Query using FTS5 MATCH and rank bm25
	query := `
		SELECT sd.id, sd.title, bm25(source_downloads_fts) as rank
		FROM source_downloads_fts fts
		JOIN source_downloads sd ON sd.id = fts.rowid
		WHERE source_downloads_fts MATCH ?
		ORDER BY rank
	`
	rows, err := db.QueryContext(context.Background(), query, `"pink"* AND "floyd"*`)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id int64
		var title string
		var rank float64
		if err := rows.Scan(&id, &title, &rank); err != nil {
			t.Fatal(err)
		}
		t.Logf("Found: %s (rank %f)", title, rank)
		count++
	}
	if count != 2 {
		t.Fatalf("expected 2 Pink Floyd results, got %d", count)
	}

	// 6. Test Cyrillic unicode61 tokenization
	rows2, err := db.QueryContext(context.Background(), query, `"кино"*`)
	if err != nil {
		t.Fatalf("cyrillic query failed: %v", err)
	}
	defer rows2.Close()
	var cyrCount int
	for rows2.Next() {
		var id int64
		var title string
		var rank float64
		_ = rows2.Scan(&id, &title, &rank)
		t.Logf("Cyrillic found: %s", title)
		cyrCount++
	}
	if cyrCount != 2 {
		t.Fatalf("expected 2 Kino results, got %d", cyrCount)
	}

	// 7. Test rebuild command
	_, err = db.Exec(`INSERT INTO source_downloads_fts(source_downloads_fts) VALUES('rebuild')`)
	if err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}

	// 8. Test delete trigger
	_, err = db.Exec(`DELETE FROM source_downloads WHERE title LIKE '%Pink Floyd%'`)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	var postDelCount int
	row := db.QueryRow(`SELECT count(*) FROM source_downloads_fts WHERE source_downloads_fts MATCH '"pink"*'`)
	if err := row.Scan(&postDelCount); err != nil {
		t.Fatalf("count scan failed: %v", err)
	}
	if postDelCount != 0 {
		t.Fatalf("expected 0 after delete, got %d", postDelCount)
	}

	// 9. Test format filter query on remaining tracks (Linkin Park, Кино)
	// Remaining:
	// "Linkin Park - Meteora (2003) FLAC Lossless"
	// "Linkin Park - Hybrid Theory [MP3 320]"
	// "Кино - Группа крови [Lossless, FLAC]"
	// "Кино - Звезда по имени Солнце (MP3, 320 kbps)"

	// Search Linkin Park with Lossless filter
	ftsFilterLossless := `("linkin"* AND "park"*) AND (flac* OR lossless* OR ape* OR alac* OR wav* OR "24bit"*)`
	rowsFilter, err := db.QueryContext(context.Background(), query, ftsFilterLossless)
	if err != nil {
		t.Fatalf("format filter query failed: %v", err)
	}
	var lpLosslessCount int
	for rowsFilter.Next() {
		var id int64
		var title string
		var rank float64
		_ = rowsFilter.Scan(&id, &title, &rank)
		t.Logf("LP Lossless found: %s", title)
		lpLosslessCount++
	}
	rowsFilter.Close()
	if lpLosslessCount != 1 {
		t.Fatalf("expected 1 Linkin Park lossless, got %d", lpLosslessCount)
	}

	// Search Кино with MP3 320 filter
	ftsFilter320 := `("кино"*) AND ("320"*)`
	rows320, err := db.QueryContext(context.Background(), query, ftsFilter320)
	if err != nil {
		t.Fatalf("format filter 320 query failed: %v", err)
	}
	var kino320Count int
	for rows320.Next() {
		var id int64
		var title string
		var rank float64
		_ = rows320.Scan(&id, &title, &rank)
		t.Logf("Kino 320 found: %s", title)
		kino320Count++
	}
	rows320.Close()
	if kino320Count != 1 {
		t.Fatalf("expected 1 Kino 320, got %d", kino320Count)
	}
}

func TestManagerSearchFTS(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "posluchac-mgr-search-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mgr, err := NewManager(db)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Insert dummy source
	_, err = db.Exec(`INSERT INTO sources (id, name, url, item_count) VALUES (1, 'Test Source', 'http://example.com/test.json', 4)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert items
	items := []struct {
		title  string
		magnet string
	}{
		{"Pink Floyd - The Dark Side of the Moon [FLAC] 24/96", "magnet:?xt=urn:btih:aaaa1111"},
		{"Pink Floyd - Wish You Were Here (1975) [MP3 320 kbps]", "magnet:?xt=urn:btih:bbbb2222"},
		{"Linkin Park - Meteora (2003) FLAC Lossless", "magnet:?xt=urn:btih:cccc3333"},
		{"Linkin Park - Hybrid Theory [MP3 320]", "magnet:?xt=urn:btih:dddd4444"},
		{"Кино - Группа крови [Lossless, FLAC]", "magnet:?xt=urn:btih:eeee5555"},
		{"Кино - Звезда по имени Солнце (MP3, 320 kbps)", "magnet:?xt=urn:btih:ffff6666"},
	}

	for _, it := range items {
		_, err := db.Exec(`INSERT INTO source_downloads (source_id, title, title_lower, magnet_uri) VALUES (1, ?, ?, ?)`,
			it.title, it.title, it.magnet)
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()

	// 1. Search without filter
	resAll := mgr.Search(ctx, "Pink Floyd", "", 10)
	if len(resAll) != 2 {
		t.Fatalf("expected 2 Pink Floyd results, got %d", len(resAll))
	}

	// 2. Search with FLAC / Lossless filter
	resFLAC := mgr.Search(ctx, "Pink Floyd", "lossless", 10)
	if len(resFLAC) != 1 || resFLAC[0].MagnetURI != "magnet:?xt=urn:btih:aaaa1111" {
		t.Fatalf("expected 1 FLAC Pink Floyd result, got %v", resFLAC)
	}

	// 3. Search with MP3 320 filter
	res320 := mgr.Search(ctx, "Pink Floyd", "mp3_320", 10)
	if len(res320) != 1 || res320[0].MagnetURI != "magnet:?xt=urn:btih:bbbb2222" {
		t.Fatalf("expected 1 MP3 320 Pink Floyd result, got %v", res320)
	}

	// 4. Search with inline format directive format:flac
	resInline := mgr.Search(ctx, "Linkin Park format:flac", "", 10)
	if len(resInline) != 1 || resInline[0].Title != "Linkin Park - Meteora (2003) FLAC Lossless" {
		t.Fatalf("expected 1 Meteora FLAC result, got %v", resInline)
	}

	// 5. Search Cyrillic
	resCyr := mgr.Search(ctx, "кино", "320", 10)
	if len(resCyr) != 1 || resCyr[0].MagnetURI != "magnet:?xt=urn:btih:ffff6666" {
		t.Fatalf("expected 1 Kino 320 result, got %v", resCyr)
	}

	// 6. Direct magnet search
	resMag := mgr.Search(ctx, "magnet:?xt=urn:btih:cccc3333", "", 10)
	if len(resMag) != 1 || resMag[0].Title != "Linkin Park - Meteora (2003) FLAC Lossless" {
		t.Fatalf("expected direct magnet result, got %v", resMag)
	}

	// 7. Typo test: "linken park" (should find Linkin Park via fuzzy matching)
	resTypo1 := mgr.Search(ctx, "linken park", "", 10)
	if len(resTypo1) != 2 {
		t.Fatalf("expected 2 results for typo 'linken park', got %d", len(resTypo1))
	}

	// 8. Typo test: "pink floid" (should find Pink Floyd)
	resTypo2 := mgr.Search(ctx, "pink floid", "", 10)
	if len(resTypo2) != 2 {
		t.Fatalf("expected 2 results for typo 'pink floid', got %d", len(resTypo2))
	}

	// 9. Typo test: "кино звизда" (should find Кино - Звезда)
	resTypo3 := mgr.Search(ctx, "кино звизда", "", 10)
	if len(resTypo3) != 1 {
		t.Fatalf("expected 1 result for typo 'кино звизда', got %d", len(resTypo3))
	}

	// 10. Test CorrectQuery helper directly
	corrected, ok := mgr.CorrectQuery(ctx, "linken park")
	if !ok || corrected != "linkin park" {
		t.Fatalf("expected CorrectQuery('linken park') to return ('linkin park', true), got (%s, %v)", corrected, ok)
	}

	// 11. Test that non-existent multi-word queries do not return random releases matching only 1 word
	resNotFound := mgr.Search(ctx, "Baby Cute", "", 10)
	if len(resNotFound) != 0 {
		t.Fatalf("expected 0 results for 'Baby Cute' not in DB, got %d results: %v", len(resNotFound), resNotFound)
	}

	// 12. Test that format stopwords (like cue) are never returned as fuzzy replacements
	vocab := mgr.getVocab(ctx)
	cand, found := FindFuzzyCandidate("cute", vocab)
	if found && cand == "cue" {
		t.Fatalf("FindFuzzyCandidate('cute') must not return format stopword 'cue'")
	}
}

