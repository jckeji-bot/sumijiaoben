package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// SQLite does not support concurrent writes
	db.SetMaxOpenConns(1)
	return db, nil
}

func initSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS snapshots (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			captured_at TEXT    NOT NULL,
			rank        INTEGER NOT NULL,
			username    TEXT    NOT NULL,
			volume      TEXT    NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_captured_at ON snapshots(captured_at)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_username    ON snapshots(username)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func saveSnapshot(db *sql.DB, entries []Entry) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO snapshots (captured_at, rank, username, volume) VALUES (?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, e := range entries {
		if _, err := stmt.Exec(now, e.Rank, e.Username, e.Volume); err != nil {
			return err
		}
	}
	return tx.Commit()
}
