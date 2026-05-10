// Package db is the SQLite-backed earnings tracker store.
//
// Mirrors the Python schema in backend/db/earnings_tracker.py:
//   - earnings_today: per-ticker per-date status, EPS, trade result
//   - watchlist:      user-managed ticker list
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: d}
	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS earnings_today (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			ticker         TEXT    NOT NULL,
			date           TEXT    NOT NULL,
			eps_estimate   REAL,
			eps_actual     REAL,
			surprise_pct   REAL,
			eps_beat       INTEGER,
			pre_notified   INTEGER DEFAULT 0,
			post_notified  INTEGER DEFAULT 0,
			pre_drift      REAL,
			gap_pct        REAL,
			day_pct        REAL,
			direction      TEXT,
			pnl_pct        REAL,
			vol_ratio      REAL,
			reason         TEXT,
			updated_at     TEXT    DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(ticker, date)
		)`,
		`CREATE TABLE IF NOT EXISTS watchlist (
			ticker     TEXT PRIMARY KEY,
			timing     TEXT DEFAULT 'Unknown',
			added_at   TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS scan_snapshots (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			watchlist   TEXT NOT NULL,
			date        TEXT NOT NULL,
			count       INTEGER NOT NULL,
			results     TEXT NOT NULL,
			created_at  TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(watchlist, date)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

// EarningsRow is one row of the earnings_today table.
type EarningsRow struct {
	Ticker       string   `json:"ticker"`
	Date         string   `json:"date"`
	EPSEstimate  *float64 `json:"eps_estimate,omitempty"`
	EPSActual    *float64 `json:"eps_actual,omitempty"`
	SurprisePct  *float64 `json:"surprise_pct,omitempty"`
	EPSBeat      *bool    `json:"eps_beat,omitempty"`
	PreNotified  bool     `json:"pre_notified"`
	PostNotified bool     `json:"post_notified"`
	PreDrift     *float64 `json:"pre_drift,omitempty"`
	GapPct       *float64 `json:"gap_pct,omitempty"`
	DayPct       *float64 `json:"day_pct,omitempty"`
	Direction    string   `json:"direction,omitempty"`
	PnLPct       *float64 `json:"pnl_pct,omitempty"`
	VolRatio     *float64 `json:"vol_ratio,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

func (s *Store) UpsertTicker(ticker, dateStr string, epsEstimate *float64) error {
	_, err := s.db.Exec(`
		INSERT INTO earnings_today (ticker, date, eps_estimate)
		VALUES (?, ?, ?)
		ON CONFLICT(ticker, date) DO UPDATE SET
			eps_estimate = COALESCE(excluded.eps_estimate, earnings_today.eps_estimate),
			updated_at   = CURRENT_TIMESTAMP
	`, ticker, dateStr, epsEstimate)
	return err
}

func (s *Store) MarkPreNotified(ticker, dateStr string, preDrift *float64) error {
	_, err := s.db.Exec(`
		UPDATE earnings_today
		   SET pre_notified = 1,
		       pre_drift    = COALESCE(?, pre_drift),
		       updated_at   = CURRENT_TIMESTAMP
		 WHERE ticker = ? AND date = ?
	`, preDrift, ticker, dateStr)
	return err
}

type PostNotifiedArgs struct {
	EPSActual   *float64
	SurprisePct *float64
	EPSBeat     bool
	GapPct      float64
	DayPct      float64
	Direction   string
	PnLPct      float64
	VolRatio    *float64
	Reason      string
}

func (s *Store) MarkPostNotified(ticker, dateStr string, a PostNotifiedArgs) error {
	_, err := s.db.Exec(`
		UPDATE earnings_today
		   SET post_notified = 1,
		       eps_actual    = ?,
		       surprise_pct  = ?,
		       eps_beat      = ?,
		       gap_pct       = ?,
		       day_pct       = ?,
		       direction     = ?,
		       pnl_pct       = ?,
		       vol_ratio     = ?,
		       reason        = ?,
		       updated_at    = CURRENT_TIMESTAMP
		 WHERE ticker = ? AND date = ?
	`, a.EPSActual, a.SurprisePct, boolInt(a.EPSBeat),
		a.GapPct, a.DayPct, a.Direction, a.PnLPct, a.VolRatio, a.Reason,
		ticker, dateStr)
	return err
}

func (s *Store) GetPendingPost(dateStr string) ([]EarningsRow, error) {
	return s.queryRows("SELECT * FROM earnings_today WHERE date = ? AND post_notified = 0", dateStr)
}

func (s *Store) GetAllForDate(dateStr string) ([]EarningsRow, error) {
	return s.queryRows("SELECT * FROM earnings_today WHERE date = ? ORDER BY ticker", dateStr)
}

func (s *Store) queryRows(query string, args ...any) ([]EarningsRow, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EarningsRow
	for rows.Next() {
		var (
			r                                                  EarningsRow
			id                                                 int
			epsEst, epsAct, surp, preDrift, gap, day, pnl, vol sql.NullFloat64
			beat, preN, postN                                  sql.NullInt64
			dir, reason, updated                               sql.NullString
		)
		if err := rows.Scan(&id, &r.Ticker, &r.Date,
			&epsEst, &epsAct, &surp, &beat,
			&preN, &postN, &preDrift, &gap, &day,
			&dir, &pnl, &vol, &reason, &updated,
		); err != nil {
			return nil, err
		}
		r.EPSEstimate = nullToPtr(epsEst)
		r.EPSActual = nullToPtr(epsAct)
		r.SurprisePct = nullToPtr(surp)
		r.PreDrift = nullToPtr(preDrift)
		r.GapPct = nullToPtr(gap)
		r.DayPct = nullToPtr(day)
		r.PnLPct = nullToPtr(pnl)
		r.VolRatio = nullToPtr(vol)
		if beat.Valid {
			b := beat.Int64 == 1
			r.EPSBeat = &b
		}
		r.PreNotified = preN.Valid && preN.Int64 == 1
		r.PostNotified = postN.Valid && postN.Int64 == 1
		r.Direction = dir.String
		r.Reason = reason.String
		r.UpdatedAt = updated.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Watchlist CRUD ───────────────────────────────────────────────────────────

func (s *Store) GetWatchlist() ([]string, error) {
	rows, err := s.db.Query("SELECT ticker FROM watchlist ORDER BY ticker")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) AddWatchlistTickers(tickers []string) error {
	if len(tickers) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT OR IGNORE INTO watchlist (ticker) VALUES (?)")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, t := range tickers {
		if _, err := stmt.Exec(strings.ToUpper(t)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RemoveWatchlistTicker(ticker string) error {
	_, err := s.db.Exec("DELETE FROM watchlist WHERE ticker = ?", strings.ToUpper(ticker))
	return err
}

func (s *Store) ClearWatchlist() error {
	_, err := s.db.Exec("DELETE FROM watchlist")
	return err
}

// ── Scan snapshots ───────────────────────────────────────────────────────────

type ScanSnapshot struct {
	Watchlist string `json:"watchlist"`
	Date      string `json:"date"`
	Count     int    `json:"count"`
	Results   string `json:"-"` // raw JSON of []ScanResult
	CreatedAt string `json:"created_at"`
}

func (s *Store) SaveSnapshot(watchlist, date string, count int, resultsJSON string) error {
	_, err := s.db.Exec(`
		INSERT INTO scan_snapshots (watchlist, date, count, results)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(watchlist, date) DO UPDATE SET
			count   = excluded.count,
			results = excluded.results,
			created_at = CURRENT_TIMESTAMP
	`, watchlist, date, count, resultsJSON)
	return err
}

// GetLatestSnapshot returns the most recent snapshot for a watchlist (any date).
func (s *Store) GetLatestSnapshot(watchlist string) (*ScanSnapshot, error) {
	row := s.db.QueryRow(`
		SELECT watchlist, date, count, results, created_at
		FROM scan_snapshots
		WHERE watchlist = ?
		ORDER BY date DESC
		LIMIT 1
	`, watchlist)
	var snap ScanSnapshot
	if err := row.Scan(&snap.Watchlist, &snap.Date, &snap.Count, &snap.Results, &snap.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &snap, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func TodayStr() string {
	return time.Now().Format("2006-01-02")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullToPtr(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

// ErrNotFound is returned when a row is not found.
var ErrNotFound = errors.New("not found")
