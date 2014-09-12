/*
Handles the network level connection to a local bitcoin node that is providing
the live feed into the network.
*/
package main

import (
	"time"

	"github.com/NSkelsey/btcsubprotos"
	"github.com/NSkelsey/protocol/ahimsa"
	"github.com/NSkelsey/watchtower"
	"github.com/conformal/btcrpcclient"
	"github.com/conformal/btcwire"
)

func txClosure(db *LiteDb, subChan chan *TxReq) func(*watchtower.TxMeta) {
	// builds the parser to use for watchtower

	txParser := func(meta *watchtower.TxMeta) {
		// Log incoming txs trying to update if they are in a block
		// just inserting if they are not.
		if btcsubprotos.IsBulletin(meta.MsgTx) {

			// Here we determine the author of the tx
			author, err := ahimsa.GetAuthor(meta.MsgTx, activeNetParams)
			if err != nil {
				author = "NULL"
				logger.Println(err)
				return
			}

			var bltn *ahimsa.Bulletin
			if meta.BlockSha != nil {
				bhash := btcwire.ShaHash{}
				err = bhash.SetBytes(meta.BlockSha)
				if err != nil {
					logger.Println(err)
					return
				}
				bltn, err = ahimsa.NewBulletin(meta.MsgTx, author, &bhash)
				if err != nil {
					logger.Println(err)
					return
				}
			} else {
				bltn, err = ahimsa.NewBulletin(meta.MsgTx, author, nil)
				if err != nil {
					logger.Println(err)
					return
				}
			}

			logger.Printf("Topic: %s", bltn.Topic)
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
		prevblk, err := db.GetBlkRecord(&blk.Header.PrevBlock)
		if err != nil {
			logger.Println(err)
			return
		}

		err = db.storeBlockHead(&blk.Header, prevblk.height+1)
		if err != nil {
			logger.Println("We tried")
			logger.Println(err)
			return
		}
		logger.Println("Stored a block")

		return
	}
	return blockParser
}

type TxReq struct {
	txid         *btcwire.ShaHash
	responseChan chan *btcwire.MsgTx
}

func authorlookup(client *btcrpcclient.Client, subChan chan *TxReq) {
	// Runs a loop listening for requests for author transactions
	// Responds with the relevant msgtx
	queue := make([]*TxReq, 0)
	for {
		select {
		case newReq := <-subChan:
			queue = append(queue, newReq)
		default:
			if len(queue) > 0 {
				req := queue[0]
				queue = queue[1:]
				rpctx, err := client.GetRawTransaction(req.txid)
				if err != nil {
					close(req.responseChan)
					logger.Println(err)
				}
				req.responseChan <- rpctx.MsgTx()
				close(req.responseChan)
			}
		}
	}
}

func makeBlockMsg(db *LiteDb) (btcwire.Message, error) {
	// Returns a getblocks msg whose hashtop is the current highest chain we have.
	tipR, err := db.GetChainTip()
	if err != nil {
		return nil, err
	}
	msg := btcwire.NewMsgGetBlocks(tipR.hash)
	msg.AddBlockLocatorHash(tipR.hash)
	return msg, nil
}
