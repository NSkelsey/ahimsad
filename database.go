package main

import (
	"database/sql"
	"errors"
	"log"

	_ "code.google.com/p/go-sqlite/go1/sqlite3"
	"github.com/NSkelsey/protocol/ahimsa"
	"github.com/conformal/btcwire"
)

var (
	errNoDb = errors.New("Could not find a db to load")
)

type LiteDb struct {
	writes int
	conn   *sql.DB
}

type blockRecord struct {
	// maps to a row stored in the db
	hash     *btcwire.ShaHash
	prevhash *btcwire.ShaHash
	height   int
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
	var height int64
	err = rows.Scan(&height)
	if err != nil {
		log.Println(err)
		return 0
	}
	return height
}

func InitDb(dbpath string) (*LiteDb, error) {
	conn, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return nil, err
	}

	// Get the database schema for the public record.
	create, err := ahimsa.GetCreateSql()
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

	println(hash.String(), height)
	_, err := db.conn.Exec(cmd, hash.String(), bh.PrevBlock.String(), height)
	if err != nil {
		return err
	}
	return nil
}

func (db *LiteDb) storeBulletin(bltn *ahimsa.Bulletin) error {
	// Writes a bulletin into the sqlite db, runs an insert or update

	var err error
	if bltn.Block == nil {
		cmd := `INSERT OR REPLACE INTO bulletins (txid, author, topic, message) VALUES($1, $2, $3, $4)`
		_, err = db.conn.Exec(cmd,
			bltn.Txid.String(),
			bltn.Author,
			bltn.Topic,
			bltn.Message,
		)
	} else {
		blockstr := bltn.Block.String()
		cmd := `INSERT OR REPLACE INTO bulletins (txid, block, author, topic, message) VALUES($1, $2, $3, $4, $5)`
		_, err = db.conn.Exec(cmd,
			bltn.Txid.String(),
			blockstr,
			bltn.Author,
			bltn.Topic,
			bltn.Message,
		)
	}
	if err != nil {
		return err
	}

	return nil
}

func (db *LiteDb) BatchInsertBH(blcks []*Block, height int) error {

	stmt, err := db.conn.Prepare("INSERT INTO blocks (hash, prevhash, height) VALUES(?, ?, ?)")
	if err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	for _, blk := range blcks {
		bh := btcBHFromBH(*blk.Head)
		hash, _ := bh.BlockSha()
		prevh := bh.PrevBlock
		_, err = tx.Stmt(stmt).Exec(hash.String(), prevh.String(), height-blk.depth)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *LiteDb) GetBlkRecord(target *btcwire.ShaHash) (*blockRecord, error) {
	cmd := `SELECT hash, prevhash, height FROM blocks WHERE hash=$1`
	rows, err := db.conn.Query(cmd, target.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blkrec, err := scanBlkRec(rows)
	if err != nil {
		return nil, err
	}
	return blkrec, nil
}

func (db *LiteDb) GetChainTip() (*blockRecord, error) {
	cmd := `SELECT hash, prevhash, max(height) FROM blocks`
	rows, err := db.conn.Query(cmd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blkrec, err := scanBlkRec(rows)
	if err != nil {
		return nil, err
	}
	return blkrec, nil
}

func scanBlkRec(rows *sql.Rows) (*blockRecord, error) {
	rows.Next()
	// called for effect
	var hash, prevhash string
	var height int
	if err := rows.Scan(&hash, &prevhash, &height); err != nil {
		return nil, err
	}

	btchash, err := btcwire.NewShaHashFromStr(hash)
	if err != nil {
		return nil, err
	}

	btcprevhash, err := btcwire.NewShaHashFromStr(prevhash)
	if err != nil {
		return nil, err
	}

	blkrec := &blockRecord{
		hash:     btchash,
		prevhash: btcprevhash,
		height:   height,
	}
	return blkrec, nil
}
