package main

import (
	"database/sql"
	"errors"
	"fmt"
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
}

func LoadDb(dbpath string) (*LiteDb, error) {
	conn, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatal(err)
	}

	db := &LiteDb{
		conn: conn,
	}

	return db, nil
}

func (db *LiteDb) CurrentHeight() int64 {
	// Returns the current height of the blocks in the db, if db is not initialized
	// return 0.
	cmd := `SELECT max(height) FROM blocks`
	rows, err := db.conn.Query(cmd)
	if err != nil {
		return 0
	}
	defer rows.Close()

	rows.Next()
	var height *int64
	err = rows.Scan(height)
	if err != nil {
		log.Println(err)
		return 0
	}
	return *height
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

	dropcmd := `
	DROP TABLE IF EXISTS blocks;
	DROP TABLE IF EXISTS bulletins;
	`

	// DROP db if it exists and recreate it.
	_, err = conn.Exec(dropcmd + create)
	if err != nil {
		return nil, err
	}

	db := &LiteDb{
		conn: conn,
	}

	return db, nil
}

func (db *LiteDb) storeBlockHead(bh *btcwire.BlockHeader, height int) error {
	// Writes a block to the sqlite db

	cmd := `INSERT INTO blocks (hash, prevhash, height) VALUES($1, $2, $3)`

	hash, _ := bh.BlockSha()

	_, err := db.conn.Exec(cmd, hash.String(), bh.PrevBlock.String(), height)
	if err != nil {
		return err
	}
	return nil
}

func (db *LiteDb) storeBulletin(bltn *Bulletin) error {
	// Writes a bulletin into the sqlite db

	cmd := `INSERT INTO bulletins (txid, block, author, topic, message) VALUES($1, $2, $3, $4, $5)`

	_, err := db.conn.Exec(cmd,
		bltn.txid.String(),
		bltn.block.String(),
		bltn.Author,
		bltn.Topic,
		bltn.Message,
	)
	if err != nil {
		return err
	}

	return nil
}

func (db *LiteDb) BatchInsertBHeads(blcks []*Block) error {

	stmt, err := db.conn.Prepare("INSERT INTO blocks (hash, prevhash, height) VALUES(?, ?, ?)")
	if err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	tx.Stmt(stmt).Exec(fmt.Scanf("%x", blk.hash), bh.prev)

	return nil
}
