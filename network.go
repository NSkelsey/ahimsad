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
func txClosure(db *LiteDb, subChan chan *TxReq) func(*watchtower.TxMeta) {

	txParser := func(meta *watchtower.TxMeta) {
		if btcsubprotos.IsBulletin(meta.MsgTx) {

			var bltn *ahimsa.Bulletin
			if meta.BlockSha != nil {
				// The bltn is in a block
				bhash := btcwire.ShaHash{}
				err = bhash.SetBytes(meta.BlockSha)
				if err != nil {
					logger.Println(err)
					return
				}
				bltn, err = ahimsa.NewBulletin(meta.MsgTx, &bhash)
				if err != nil {
					logger.Println(err)
					return
				}
			} else {
				bltn, err = ahimsa.NewBulletin(meta.MsgTx, nil)
				if err != nil {
					logger.Println(err)
					return
				}
			}

			logger.Printf("Board: %s", bltn.Board)
			if err := db.storeBulletin(bltn); err != nil {
				logger.Println(err)
				return
			}
		}

	}

	return txParser
}

func blockClosure(db *LiteDb) func(time.Time, *btcwire.MsgBlock) {
	blockParser := func(now time.Time, blk *btcwire.MsgBlock) {

		hash, _ := blk.Header.BlockSha()
		logger.Printf("Block: %s\n", hash.String())

		prevblkrec, err := db.GetBlkRecord(&blk.Header.PrevBlock)
		if err == sql.ErrNoRows {
			logger.Printf("Prevblk not in DB. %s\n", prevblkrec.hash.String())
			return
		}
		if err != nil {
			logger.Println(err)
			return
		}
		height := prevblkrec.height + 1

		err = db.storeBlockHead(&blk.Header, height)
		if err != nil {
			logger.Println("We tried")
			logger.Println(err)
			return
		}
		logger.Printf("Stored the block at height: %d\n\n", height)

		return
	}
	return blockParser
}

// Returns a getblocks msg whose hashstop is the current highest chain in the db.
func makeBlockMsg(db *LiteDb) (btcwire.Message, error) {
	tipR, err := db.GetChainTip()
	if err != nil {
		return nil, err
	}
	msg := btcwire.NewMsgGetBlocks(tipR.hash)
	msg.AddBlockLocatorHash(tipR.hash)
	return msg, nil
}
