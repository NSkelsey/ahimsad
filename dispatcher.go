package main

import (
	"log"
	"path/filepath"

	"github.com/conformal/btcrpcclient"
	"github.com/conformal/btcutil"
	"github.com/jessevdk/go-flags"
)

var (
	appDataDir        = btcutil.AppDataDir("ahimsa", false)
	defaultConfigFile = filepath.Join(appDataDir, "ahimsa.conf")
	defaultBlockDir   = filepath.Join(btcutil.AppDataDir(".bitcoin", false), "blocks")
	defaultDbName     = "pubrecord.db"
	// Sane defaults for a linux based OS
	cfg = &config{
		ConfigFile: defaultConfigFile,
		BlockDir:   defaultBlockDir,
		DbFile:     defaultDbName,
		Rebuild:    false,
	}
)

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
	loadDb(rpcclient)

	if err != nil {
		log.Fatal(err)
	}

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

	if curH < actualH || cfg.Rebuild {
		// init db
		db, err = InitDb(cfg.DbFile)
		if err != nil {
			log.Fatal(err)
		}

		err := runBlockScan(cfg.BlockDir, db)
		if err != nil {
			log.Fatal(err)
		}

	}

	return db
}
