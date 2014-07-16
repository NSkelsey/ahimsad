package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/NSkelsey/btcsubprotos"
	"github.com/conformal/btcwire"
)

var (
	blockdir    = flag.String("blockdir", "/root/.bitcoin/blocks", "The directory containing bitcoin blocks")
	logger      = log.New(os.Stdout, "", log.Llongfile)
	empt        = [32]byte{}
	genesisHash = [32]byte{}
	maxBlocks   = 500000
)

type BlockHead struct {
	// A struct that matches the exact format of blocks stored in blk*.dat files
	Magic      [4]byte
	Length     uint32
	Version    int32
	PrevHash   [32]byte
	MerkleRoot [32]byte
	Timestamp  uint32
	Difficulty uint32
	Nonce      uint32
}

type Block struct {
	// A custom block object for processing
	PrevBlock *Block
	NextBlock *Block
	Head      *BlockHead
	RelTxs    []*btcwire.MsgTx
	Hash      [32]byte
	depth     int
}

func check(err error) {
	if err != nil {
		logger.Fatal(err)
	}
}

func btcBHFromBH(bh BlockHead) *btcwire.BlockHeader {
	// utility function to convert custom BlockHead type to btcwire BlockHeader
	prevhash, _ := btcwire.NewShaHash(bh.PrevHash[:])
	merkle, _ := btcwire.NewShaHash(bh.MerkleRoot[:])
	timestamp := time.Unix(int64(bh.Timestamp), 0)

	btcbh := btcwire.BlockHeader{
		Version:    bh.Version,
		PrevBlock:  *prevhash,
		MerkleRoot: *merkle,
		Timestamp:  timestamp,
		Bits:       bh.Difficulty,
		Nonce:      bh.Nonce,
	}
	return &btcbh
}

func blockHash(bh BlockHead) [32]byte {
	// Print the hash of the block from the headers in the block
	btcbh := btcBHFromBH(bh)
	hash, _ := btcbh.BlockSha()
	return [32]byte(hash)
}

func proceed(f *os.File) bool {
	// finds the start of the next block and places the cursor on it
	for {
		var b [4]byte
		_, err := io.ReadFull(f, b[:])
		if err != nil {
			return true
		}
		discrim := binary.BigEndian.Uint32(b[:])
		if discrim != 0x00000000 {
			// seek backwards to start of block
			// TODO make more effecient
			f.Seek(-4, 1)
			return false
		}
	}
}

func processFile(fname string, blkList []*Block, blkMap map[[32]byte]*Block) ([]*Block, map[[32]byte]*Block) {
	// given a blk file attempts to parse every block within it. Adding the block
	// to a global list of seen blocks. Additionally we strip out the interesting
	// transactions at this stage.
	file, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	seenGenesis := true
	if len(blkList) == 0 {
		seenGenesis = false
	}
	for {
		var blk Block
		var bh BlockHead

		done := proceed(file)
		if done {
			fmt.Printf("\rFinished file: %s", fname)
			break
		}
		err = binary.Read(file, binary.LittleEndian, &bh)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			fmt.Printf("\rFinished file: %s", fname)
			break
		}
		check(err)

		tx_num, err := readVarInt(file, 0)
		check(err)

		hash := blockHash(bh)

		reltxs := make([]*btcwire.MsgTx, 0)
		// Process each tx in block
		for i := uint64(0); i < tx_num; i++ {
			tx := &btcwire.MsgTx{}
			err := tx.Deserialize(file)
			check(err)

			if btcsubprotos.IsBulletin(tx) {
				reltxs = append(reltxs, tx)
			}
		}

		blk = Block{
			PrevBlock: nil,
			NextBlock: nil,
			Head:      &bh,
			RelTxs:    reltxs,
			Hash:      hash,
			depth:     1,
		}
		if !seenGenesis {
			seenGenesis = true
			genesisHash = hash
			// Make the hash of the genesis block useful
			var s [32]byte
			copy(s[:], hash[:])
			for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
				s[i], s[j] = s[j], s[i]
			}
			fmt.Printf("The hash of the genesis block:\n%x\n", s)
		}
		blkMap[hash] = &blk
		blkList = append(blkList, &blk)
	}

	return blkList, blkMap
}

func calcHeight(blkList []*Block, blkMap map[[32]byte]*Block) int {
	// Computes the best chain's total height by starting from the latest blocks
	// and working pack to the genesis block.
	for j := len(blkList) - 1; j >= 0; j-- {
		blk := blkList[j]
		//if blk.PrevBlock == nil && blk.Hash == genesisHash {
		if blk.Hash == genesisHash {
			println("Found Genesis Hash")
			return blk.depth
		}
		nextD := blk.depth + 1
		prevBlock, ok := blkMap[blk.Head.PrevHash]
		if ok {
			if prevBlock.depth < nextD {
				prevBlock.depth = nextD
			}
		}
	}
	return -1
}

func main() {
	flag.Parse()

	glob := "/blk*.dat"
	blockfiles, err := filepath.Glob(*blockdir + glob)
	if err != nil {
		log.Fatal(err)
	}
	if len(blockfiles) < 1 {
		log.Fatal(errors.New("Could not find any blockfiles at " + *blockdir))
	}

	blkList := make([]*Block, 0, maxBlocks)
	blkMap := make(map[[32]byte]*Block)
	for _, filename := range blockfiles {
		blkList, blkMap = processFile(filename, blkList, blkMap)
		fmt.Printf("\tProcessed: %d", len(blkList))
	}

	// glue block pointers together
	genesisBlk := linkChain(blkList, blkMap)
	// find the tip of the longest chain
	tip, h := chainTip(genesisBlk)
	fmt.Printf("\nHeight: %d\n", h)
	//printBlockHead(*tip.Head)

	txs := collectRelTxs(tip)
	for _, tx := range txs {
		hash, _ := tx.TxSha()
		fmt.Println(hash.String())
	}
	println("We found: ", len(txs))
}

func collectRelTxs(blk *Block) []*btcwire.MsgTx {
	txs := make([]*btcwire.MsgTx, 0, 10000)
	for {
		if blk.Hash == genesisHash {
			break
		}
		for _, tx := range blk.RelTxs {
			txs = append(txs, tx)
		}
		blk = blk.PrevBlock
	}
	return txs
}

func linkChain(blkList []*Block, blkMap map[[32]byte]*Block) *Block {
	// Walks the block list backwards & builds out the linked list so that on a
	// walk back up we can return the block at the end of the longest chain
	absents := 0
	for j := len(blkList) - 1; j >= 0; j-- {
		blk := blkList[j]
		if blk.Hash == genesisHash {
			break
		}

		prevBlk, ok := blkMap[blk.Head.PrevHash]
		if !ok {
			absents++
		} else {
			// this block points back to another block that we have in memory
			currentD := blk.depth + 1
			if prevBlk.depth < currentD {
				prevBlk.depth = currentD
				prevBlk.NextBlock = blk
				blk.PrevBlock = prevBlk
			}
		}
	}
	genesisBlk, ok := blkMap[genesisHash]
	if !ok {
		logger.Fatal("Could not find the genesis block. Big problem!")
	}
	return genesisBlk
}

func chainTip(blk *Block) (*Block, int) {
	return recurseTip(blk, 0)
}

func recurseTip(blk *Block, confs int) (*Block, int) {
	if blk.NextBlock == nil {
		return blk, confs
	} else {
		//		println(blk.Head.Nonce)
		return recurseTip(blk.NextBlock, confs+1)
	}
}

// From btcwire common.go
func readVarInt(r io.Reader, pver uint32) (uint64, error) {
	// readVarInt reads a variable length integer from r and returns it as a uint64.
	var b [8]byte
	_, err := io.ReadFull(r, b[0:1])
	if err != nil {
		return 0, err
	}

	var rv uint64
	discriminant := uint8(b[0])
	switch discriminant {
	case 0xff:
		_, err := io.ReadFull(r, b[:])
		if err != nil {
			return 0, err
		}
		rv = binary.LittleEndian.Uint64(b[:])

	case 0xfe:
		_, err := io.ReadFull(r, b[0:4])
		if err != nil {
			return 0, err
		}
		rv = uint64(binary.LittleEndian.Uint32(b[:]))

	case 0xfd:
		_, err := io.ReadFull(r, b[0:2])
		if err != nil {
			return 0, err
		}
		rv = uint64(binary.LittleEndian.Uint16(b[:]))

	default:
		rv = uint64(discriminant)
	}

	return rv, nil
}

func printBlockHead(blk BlockHead) {
	// Prints out header from a given block

	prevhash, _ := btcwire.NewShaHash(blk.PrevHash[:])
	merkle, _ := btcwire.NewShaHash(blk.MerkleRoot[:])
	timestamp := time.Unix(int64(blk.Timestamp), 0)

	bh := btcwire.BlockHeader{
		Version:    blk.Version,
		PrevBlock:  *prevhash,
		MerkleRoot: *merkle,
		Timestamp:  timestamp,
		Bits:       blk.Difficulty,
		Nonce:      blk.Nonce,
	}
	hash, err := bh.BlockSha()
	check(err)
	fmt.Printf(`
Hash:		%s
prevHash:	%s
merkle root:	%s
timestamp:	%s
difficulty:	%d
nonce:		%d
bit len:	%d
==========
`,
		hash, prevhash.String(), merkle.String(), timestamp, blk.Difficulty, blk.Nonce, blk.Length)
}
