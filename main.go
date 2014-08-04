package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/NSkelsey/btcbuilder"
	"github.com/NSkelsey/protocol/ahimsa"
	"github.com/NSkelsey/watchtower"
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
	defaultNetwork    = "TestNet3"
	defaultNodeAddr   = "127.0.0.1:18333"
	// Sane defaults for a linux based OS
	cfg = &config{
		ConfigFile: defaultConfigFile,
		BlockDir:   defaultBlockDir,
		DbFile:     defaultDbName,
		NodeAddr:   defaultNodeAddr,
		NetName:    defaultNetwork,
		Rebuild:    false,
	}
)

// Application globals
var activeNetParams *btcnet.Params
var logger *log.Logger = log.New(os.Stdout, "", log.Ltime|log.Lshortfile)

type config struct {
	ConfigFile  string `short:"C" long:"configfile" description:"Path to configuration file"`
	BlockDir    string `long:"blockdir" description:"Path to bitcoin blockdir"`
	DbFile      string `long:"dbname" description:"Name of the database file"`
	Rebuild     bool   `long:"rebuild" description:"Flag to rebuild the pubrecord db"`
	RPCAddr     string `long:"rpcaddr" description:"Address of bitcoin rpc endpoint to use"`
	RPCUser     string `long:"rpcuser" description:"rpc user"`
	RPCPassword string `long:"rpcpassword" description:"rpc password"`
	NodeAddr    string `long:"nodeaddr" description:"Address + port of the bitcoin node to connect to"`
	NetName     string `short:"n" long:"network" description:"The name of the network to use"`
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

	activeNetParams, err = btcbuilder.NetParamsFromStr(cfg.NetName)
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

	rpcSubChan := make(chan *TxReq)

	// start a rpc command handler
	go authorlookup(rpcclient, rpcSubChan)

	fmt.Println(getBanner())
	db := loadDb(rpcclient)

	if err != nil {
		log.Fatal(err)
	}
	curH := db.CurrentHeight()
	actualH, err := rpcclient.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}
	println("Db Height:", curH)

	var towerCfg watchtower.TowerCfg
	if actualH-curH > 0 {
		getblocks, err := makeBlockMsg(db)
		if err != nil {
			log.Fatal(err)
		}
		towerCfg = watchtower.TowerCfg{
			Addr:        cfg.NodeAddr,
			Net:         activeNetParams.Net,
			StartHeight: int(db.CurrentHeight()),
			ToSend:      []btcwire.Message{getblocks},
		}
	} else {
		towerCfg = watchtower.TowerCfg{
			Addr:        cfg.NodeAddr,
			Net:         activeNetParams.Net,
			StartHeight: int(db.CurrentHeight()),
		}
	}

	// Start a watchtower instance and listen for new blocks
	txParser := txClosure(db, rpcSubChan)
	blockParser := blockClosure(db)

	watchtower.Create(towerCfg, txParser, blockParser)
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

	println("Database hieghts:", curH, actualH)
	// Fudge factor
	if curH < actualH-499 || cfg.Rebuild {
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

		genBlk := walkBackwards(tip)
		err = storeChainBulletins(genBlk, db, client)
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
	chainHeight := genBlock.depth
	blks := make([]*Block, 0, 1000)
	var blk *Block = genBlock
	for {
		// Walk forwards through the blocks
		if blk.NextBlock == nil {
			break
		}

		if len(blks) >= 1000 {
			err := db.BatchInsertBH(blks, chainHeight)
			if err != nil {
				return err
			}
			blks = make([]*Block, 0, 1000)
		}
		blks = append(blks, blk)

		blk = blk.NextBlock
	}

	// Insert remaining blocks
	err := db.BatchInsertBH(blks, chainHeight)
	if err != nil {
		return err
	}

	// We are now back at the tip
	//var tip *Block = blk
	for {
		// walk backwards through blocks
		if blk.Hash == genesisHash {
			break
		}

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

			bltn, err := ahimsa.NewBulletin(tx, author, &blockhash)
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
