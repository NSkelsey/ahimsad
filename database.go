package main

import (
	"database/sql"
	"errors"
	"log"

	_ "code.google.com/p/go-sqlite/go1/sqlite3"
	"github.com/NSkelsey/protocol"
	"github.com/conformal/btcwire"
)

var (
	errNoDb = errors.New("Could not find a db to load")
)

type LiteDb struct {
	writes int
	conn   *sql.DB
	height int
}

func LoadDb(dbpath string, height int) (*LiteDb, error) {
	conn, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatal(err)
	}

	db := &LiteDb{
		conn: conn,
	}

	return db, nil
}

func InitDb(dbpath string) (*LiteDb, error) {
	conn, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return nil, err
	}

	// Get the database schema for the public record.
	create, err := protocol.GetCreateSql()
	if err != nil {
		return nil, err
	}

	// DROP db if it exists and recreate it.
	_, err = conn.Exec(create)
	if err != nil {
		return nil, err
	}

	db := &LiteDb{
		conn: conn,
	}

	return db, nil
}

func (db *LiteDb) storeBlock(blk *btcwire.MsgBlock) {
	// Writes a block to the sqlite db

}

func (db *LiteDb) storeBulletin(bltn Bulletin) {
	// Writes a bulletin into the sqlite db

}
