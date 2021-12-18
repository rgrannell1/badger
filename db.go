package main

import (
	"database/sql"
	"path/filepath"
)

type BadgerDb struct {
	db *sql.DB
}

/*
 * Construct a database
 */
func NewSqliteDB(opts *BadgerOpts) (*sql.DB, error) {
	dbPath := filepath.Join(opts.to, ".badger_metadata.sqlite")
	return sql.Open("sqlite3", dbPath)
}

func (conn *BadgerDb) Close() error {
	return conn.db.Close()
}

func (conn *BadgerDb) CreateTables() error {
	tx, err := conn.db.Begin()
	defer tx.Rollback()

	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS mediaData (
      src             TEXT NOT NULL,
			dst             TEXT NOT NULL,
			hash            TEXT NOT NULL,
			id              INTEEGR NOT NULL,
			clusterId       INTEGER NOT NULL,
			blur            INTEGER,
			mediaType       TEXT NOT NULL,
			iso             TEXT,
			aperture        TEXT,
			shutterSpeed    TEXT,
			mtime           TEXT
	)`)

	if err != nil {
		return err
	}

	tx.Commit()

	return nil
}

func (conn *BadgerDb) InsertMedia(media *Media) error {
	tx, err := conn.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	iso := ""
	aperture := ""
	shutterSpeed := ""

	if media.exifData != nil {
		iso = media.exifData.Iso
		aperture = media.exifData.Aperture
		shutterSpeed = media.exifData.ShutterSpeed
	}

	_, err = tx.Exec(`
	INSERT INTO mediaData (
		src,
		dst,
		hash,
		id,
		clusterId,
		blur,
		mediaType,
		iso,
		aperture,
		shutterSpeed
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		media.source,
		media.GetChosenName(),
		media.hash,
		media.id,
		media.clusterId,
		media.blur,
		media.GetType(),
		iso,
		aperture,
		shutterSpeed,
	)

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (conn *BadgerDb) GetMedia(media *Media) error {
	return nil
}
