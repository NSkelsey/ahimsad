package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/conformal/btcnet"
	"github.com/conformal/btcrpcclient"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwire"
	"github.com/jessevdk/go-flags"
)

var (
	appDataDir        = btcutil.AppDataDir("ahimsa", false)
	defaultConfigFile = filepath.Join(appDataDir, "ahimsa.conf")
	defaultDbName     = filepath.Join(appDataDir, "pubrecord.db")
	defaultBlockDir   = filepath.Join(btcutil.AppDataDir(".bitcoin", false), "blocks")
	// Sane defaults for a linux based OS
	cfg = &config{
		ConfigFile: defaultConfigFile,
		BlockDir:   defaultBlockDir,
		DbFile:     defaultDbName,
		Rebuild:    false,
	}

	// Application globals
)
var activeNetParams *btcnet.Params

type config struct {
	ConfigFile  string `short:"C" long:"configfile" description:"Path to configuration file"`
	BlockDir    string `long:"blockdir" description:"Path to bitcoin blockdir"`
	DbFile      string `long:"dbname" description:"Name of the database file"`
	Rebuild     bool   `long:"rebuild" description:"Flag to rebuild the pubrecord db"`
	RPCAddr     string `long:"rpcaddr" description:"Address of bitcoin rpc endpoint to use"`
	RPCUser     string `long:"rpcuser" description:"rpc user"`
	RPCPassword string `long:"rpcpassword" description:"rpc password"`
}

func main() {
	// Parse command line args first then use file args
	parser := flags.NewParser(cfg, flags.None)
	_, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	err = flags.NewIniParser(parser).ParseFile(cfg.ConfigFile)
	if err != nil {
		log.Fatal(err)
	}

	connCfg := &btcrpcclient.ConnConfig{
		Host:         cfg.RPCAddr,
		User:         cfg.RPCUser,
		Pass:         cfg.RPCPassword,
		HttpPostMode: true,
		DisableTLS:   true,
	}
	rpcclient, err := btcrpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Load db
	db := loadDb(rpcclient)

	if err != nil {
		log.Fatal(err)
	}
	println("Current Db height:", db.CurrentHeight())

	// Start a watchtower instance and listen for new blocks
}

func loadDb(client *btcrpcclient.Client) *LiteDb {
	// Load the db from the file specified in config and get it to a usuable state
	// where ahimsad can add blocks from the network
	db, err := LoadDb(cfg.DbFile)
	if err != nil {
		log.Fatal(err)
	}

	actualH, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}

	curH := db.CurrentHeight()

	println("Database heights: ", curH, actualH)
	if curH < actualH || cfg.Rebuild {
		println("Creating DB")
		// init db
		db, err = InitDb(cfg.DbFile)
		if err != nil {
			log.Fatal(err)
		}

		// get the tip of the longest valid chain
		tip, err := runBlockScan(cfg.BlockDir, db)
		if err != nil {
			log.Fatal(err)
		}

		err = storeChainBulletins(tip, db, client)
		if err != nil {
			log.Fatal(err)
		}

	}

	return db
}

func storeChainBulletins(genBlock *Block, db *LiteDb, client *btcrpcclient.Client) error {
	// Stores all of the Bulletins we found in the blockchain into the sqlite db.
	// This is done iteratively and is not optimized in any way. We log errors as
	// we encounter them.
	blks := make([]*Block, 0, 1000)
	for {
		// walk forwards through the blocks
		if blk.NextBlock == nil {
			break
		}

		if len(batch) >= 1000 {
			bh := btcBHFromBH(*blk.Head)
			err := db.BatchInsertBlockHeads(bh)
			if err != nil {
				return err
			}
			blks := make([]*Block, 0, 1000)
		}
		blks = append(blks, blk)

		blk = blk.NextBlock
	}

	// We are now back at the tip
	var tip *Block = blk
	for {
		// walk backwards through blocks
		if blk.Hash == genesisHash {
			break
		}

		chainHeight := getHeight(blk, genesisHash)

		var bh *btcwire.BlockHeader
		bh = btcBHFromBH(*blk.Head)

		blockhash, _ := bh.BlockSha()
		for _, tx := range blk.RelTxs {
			// Get author of bulletin via RPC call
			authOutpoint := tx.TxIn[0].PreviousOutpoint
			asyncRes := client.GetRawTransactionAsync(&authOutpoint.Hash)
			authorTx, err := asyncRes.Receive()
			if err != nil {
				return err
			}
			// This pubkeyscript defines the author of the post
			relScript := authorTx.MsgTx().TxOut[authOutpoint.Index].PkScript

			scriptClass, addrs, _, err := btcscript.ExtractPkScriptAddrs(relScript, activeNetParams)
			if err != nil {
				return err
			}
			if scriptClass != btcscript.PubKeyHashTy {
				return fmt.Errorf("Author script is not p2pkh")
			}
			// We know that the returned value is a P2PKH; therefore it must have
			// one address which is the author of the attached bulletin
			author := addrs[0].String()

			bltn, err := NewBulletin(tx, author, &blockhash)
			if err != nil {
				return err
			}
			if err := db.storeBulletin(bltn); err != nil {
				return err
			}
		}

		blk = blk.PrevBlock
	}
	return nil
}
