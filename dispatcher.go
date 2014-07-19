package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/conformal/btcutil"
	"github.com/jessevdk/go-flags"
)

var (
	appDataDir        = btcutil.AppDataDir("ahimsa", false)
	defaultConfigFile = filepath.Join(appDataDir, "ahimsa.conf")
	defaultBlockDir   = filepath.Join(btcutil.AppDataDir(".bitcoin", false), "blocks")
	// Sane defaults for a linux based OS
	cfg = &config{
		ConfigFile: defaultConfigFile,
		BlockDir:   defaultBlockDir,
		Rebuild:    false,
	}
)

type config struct {
	ConfigFile string `short:"C" long:"configfile" description:"Path to configuration file"`
	BlockDir   string `long:"blockdir" description:"Path to bitcoin blockdir"`
	Rebuild    bool   `long:"rebuild" description:"Flag to rebuild the pubrecord db"`
}

func main() {

	parser := flags.NewParser(cfg, flags.None)

	// Parse command line args then use file args
	_, err := parser.Parse()
	if err != nil {
		parser.Usage()
		fmt.Println(err)
		os.Exit(1)
	}

	err = flags.NewIniParser(parser).ParseFile(cfg.ConfigFile)
	if err != nil {
		parser.Usage()
		fmt.Println(err)
		os.Exit(1)
	}

	// Load db
	loadDb()

	if err != nil {
		log.Fatal(err)
	}

	// Start a watchtower instance and listen for new blocks
	watchtower.
}

func loadDb() *LiteDb {
	// try to load db
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

		err := runBlockScan(db, cfg.BlockDir)
		if err != nil {
			log.Fatal(err)
		}

	}

	return db
}
