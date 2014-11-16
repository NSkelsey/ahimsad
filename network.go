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
func blockClosure(db *LiteDb, watchTChan chan btcwire.Message) func(time.Time, *btcwire.MsgBlock) {
	blockParser := func(now time.Time, blk *btcwire.MsgBlock) {

		hash, _ := blk.Header.BlockSha()
		if cfg.Debug {
			logger.Printf("Block: [%s]\n", hash.String())
		}

		prevblkrec, err := db.GetBlkRecord(&blk.Header.PrevBlock)
		if err == sql.ErrNoRows {
			prevblk := blk.Header.PrevBlock
			logger.Printf("Prevblk is not in the DB: [%s]\n", prevblk)

			// Since the prevblock is not in the DB it is probably a reorg.
			// Therefore send a getBlk message to fill the missing blocks in.
			msgGetBlks, err := makeBlockMsg(db)
			if err != nil {
				logger.Println(err)
				return
			}

			logger.Println("Sending GetBlks msg")
			watchTChan <- msgGetBlks
			// Rerequest this block along with prevblk for good measure :-D
			getPrevBlk := btcwire.NewMsgGetData()
			getPrevBlk.InvList = []*btcwire.InvVect{
				btcwire.NewInvVect(btcwire.InvTypeBlock, &prevblk),
				btcwire.NewInvVect(btcwire.InvTypeBlock, &hash),
			}
			watchTChan <- getPrevBlk
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

// Returns a getblocks msg that requests the best chain.
func makeBlockMsg(db *LiteDb) (btcwire.Message, error) {

	chaintip, err := db.GetChainTip()
	if err != nil {
		return btcwire.NewMsgGetBlocks(nil), err
	}

	var curblk *blockRecord = chaintip
	msg := btcwire.NewMsgGetBlocks(curblk.hash)

	heights := []int{}
	step, start := 1, 0
	for i := chaintip.height; i > 0; i -= step {
		// Push last 10 indices first
		if start >= 10 {
			step *= 2
		}
		heights = append(heights, i)
		start++
	}
	heights = append(heights, 0)

	for _, h := range heights {

		var err error
		curblk, err := db.GetBlkRecHeight(h)
		if err != nil {
			return nil, err
		}
		msg.AddBlockLocatorHash(curblk.hash)
	}

	return msg, nil
}
