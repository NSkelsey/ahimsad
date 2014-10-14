/*
Handles the network level connection to a local bitcoin node that is providing
the live feed into the network.
*/
package main

import (
	"database/sql"
	"time"

	"github.com/NSkelsey/btcsubprotos"
	"github.com/NSkelsey/protocol/ahimsa"
	"github.com/NSkelsey/watchtower"
	"github.com/conformal/btcwire"
)

// Builds the transaction parser that records bulletins as they come in off the wire.
// The txParser attempts to update a row if the bulletin is already in the DB, otherwise
// it just inserts a new row.
func txClosure(db *LiteDb) func(*watchtower.TxMeta) {

	txParser := func(meta *watchtower.TxMeta) {
		if btcsubprotos.IsBulletin(meta.MsgTx) {

			var bltn *ahimsa.Bulletin
			var err error
			if meta.BlockSha != nil {
				// The bltn is in a block
				bhash := btcwire.ShaHash{}
				err = bhash.SetBytes(meta.BlockSha)
				if err != nil {
					logger.Println(err)
					return
				}
				bltn, err = ahimsa.NewBulletin(meta.MsgTx, &bhash, activeNetParams)
				if err != nil {
					logger.Println(err)
					return
				}
			} else {
				bltn, err = ahimsa.NewBulletin(meta.MsgTx, nil, activeNetParams)
				if err != nil {
					logger.Println(err)
					return
				}
			}

			logger.Printf("Stored bltn: [board: %s]", bltn.Board)
			if err := db.storeBulletin(bltn); err != nil {
				logger.Println(err)
				return
			}
		}

	}

	return txParser
}

// Records blocks as they are seen. If the previous block is not in the
// db, we ignore the block and log the problem
func blockClosure(db *LiteDb) func(time.Time, *btcwire.MsgBlock) {
	blockParser := func(now time.Time, blk *btcwire.MsgBlock) {

		hash, _ := blk.Header.BlockSha()
		if cfg.Debug {
			logger.Printf("Block: [%s]\n", hash.String())
		}

		prevblkrec, err := db.GetBlkRecord(&blk.Header.PrevBlock)
		if err == sql.ErrNoRows {
			logger.Printf("Prevblk is not in the DB: [%s]\n", blk.Header.PrevBlock)
			return
		}
		if err != nil {
			logger.Println(err)
			return
		}
		height := prevblkrec.height + 1

		err = db.storeBlockHead(&blk.Header, height)
		if err != nil {
			logger.Println(err)
			return
		}
		logger.Printf("Stored block: [height: %d]\n", height)

		return
	}
	return blockParser
}

// Returns a getblocks msg whose hashstop is the current highest chain in the db.
func makeBlockMsg(db *LiteDb, chaintip *blockRecord) btcwire.Message {
	msg := btcwire.NewMsgGetBlocks(chaintip.hash)
	msg.AddBlockLocatorHash(chaintip.hash)
	return msg
}
