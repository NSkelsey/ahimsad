package scanner

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/wire"
)

var (
	logger      *log.Logger
	empt        = [32]byte{}
	genesisHash = [32]byte{}
	maxBlocks   = 500000
)

// A struct that matches the exact format of blocks stored in blk*.dat files
type BlockHead struct {
	Magic      [4]byte
	Length     uint32
	Version    int32
	PrevHash   [32]byte
	MerkleRoot [32]byte
	Timestamp  uint32
	Difficulty uint32
	Nonce      uint32
}

// A custom block object for processing as a linked list
type Block struct {
	PrevBlock *Block
	NextBlock *Block
	Head      *BlockHead
	RelTxs    []*wire.MsgTx
	Hash      [32]byte
	Depth     int
}

type Scanner struct {
	logger *log.Logger
}

func check(err error) {
	if err != nil {
		logger.Fatal(err)
	}
}

// Public interface to convert a scanner.BlockHead into a wire BlockHeader
func ConvBHtoBTCBH(bh BlockHead) *wire.BlockHeader {
	prevhash, _ := wire.NewShaHash(bh.PrevHash[:])
	merkle, _ := wire.NewShaHash(bh.MerkleRoot[:])
	timestamp := time.Unix(int64(bh.Timestamp), 0)

	btcbh := wire.BlockHeader{
		Version:    bh.Version,
		PrevBlock:  *prevhash,
		MerkleRoot: *merkle,
		Timestamp:  timestamp,
		Bits:       bh.Difficulty,
		Nonce:      bh.Nonce,
	}
	return &btcbh
}

// Return the hash of the block from the headers in the block
func blockHash(bh BlockHead) [32]byte {
	btcbh := ConvBHtoBTCBH(bh)
	hash, _ := btcbh.BlockSha()
	return [32]byte(hash)
}

// Finds the start of the next block and places the cursor on it
func proceed(f *os.File) bool {
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

// Given a blk file attempts to parse every block within it. Adding the block
// to a global list of seen blocks. Additionally we strip out the interesting
// transactions at this stage.
func processFile(fname string, blkList []*Block, blkMap map[[32]byte]*Block) ([]*Block, map[[32]byte]*Block, error) {
	file, err := os.Open(fname)
	if err != nil {
		return blkList, blkMap, err
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

		reltxs := make([]*wire.MsgTx, 0)
		// Process each tx in block
		for i := uint64(0); i < tx_num; i++ {
			tx := &wire.MsgTx{}
			err := tx.Deserialize(file)
			if err != nil {
				logger.Fatal(err)
			}

			// Append every transaction to the relevant transaction list.
			// NOTE IF you wanted to classify transactions this is a good spot todo it.
			reltxs = append(reltxs, tx)
		}

		blk = Block{
			PrevBlock: nil,
			NextBlock: nil,
			Head:      &bh,
			RelTxs:    reltxs,
			Hash:      hash,
			Depth:     1,
		}
		if !seenGenesis {
			seenGenesis = true
			genesisHash = hash
			// Make the hash of the genesis block useful
			genBlock := ConvBHtoBTCBH(bh)
			_hash, _ := genBlock.BlockSha()
			fmt.Printf("The hash of the genesis block:\n%s\n", _hash)
		}
		blkMap[hash] = &blk
		blkList = append(blkList, &blk)
	}

	return blkList, blkMap, nil
}

func getHeight(blk *Block, target [32]byte) int {
	for {
		// give up after a few iterations
		if blk.Hash == target {
			return blk.Depth
		}
		if blk.PrevBlock == nil {
			err := fmt.Errorf("walked off the end of the chain")
			log.Fatal(err)
		}
		blk = blk.PrevBlock
	}
}

// Reads the bitcoin ~/.bitcoin/block dir for the block chain and returns the
// genesis blk linked all the way to the longest valid chain. Every orphaned
// block is left out of the chain.
func RunBlockScan(blockdir string, logger *log.Logger) (*Block, error) {
	// Global b/c it is its own package
	logger = logger

	glob := "/blk*.dat"
	blockfiles, err := filepath.Glob(blockdir + glob)
	if err != nil {
		return nil, err
	}
	if len(blockfiles) < 1 {
		return nil, fmt.Errorf("Could not find any blockfiles at %s", blockdir)
	}

	logger.Printf("About to process %d blockfiles\n", len(blockfiles))
	blkList := make([]*Block, 0, maxBlocks)
	blkMap := make(map[[32]byte]*Block)
	for _, filename := range blockfiles {
		blkList, blkMap, err = processFile(filename, blkList, blkMap)
		if err != nil {
			return nil, err
		}
		fmt.Printf("\tProcessed: %d", len(blkList))
	}

	// glue block pointers together
	genesisBlk := linkChain(blkList, blkMap)
	// find the tip of the longest chain
	_, h := chainTip(genesisBlk)
	logger.Printf("\nHeight from block files: [%d]\n", h)

	return genesisBlk, nil
}

// Walks the block list backwards & builds out the linked list so that on a
// walk back up we can return the block at the end of the longest chain
func linkChain(blkList []*Block, blkMap map[[32]byte]*Block) *Block {
	absents := 0
	// this loop starts at the end of the blocklist and proceeds backwards
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
			currentD := blk.Depth + 1
			if prevBlk.Depth < currentD {
				prevBlk.Depth = currentD
				prevBlk.NextBlock = blk
				blk.PrevBlock = prevBlk
			}
		}
	}
	// obtain genesis block after linkage (it is now the root of the linked chain)
	genesisBlk, ok := blkMap[genesisHash]
	if !ok {
		logger.Fatal("Could not find the genesis block. Big problem!")
	}
	return genesisBlk
}

func chainTip(blk *Block) (*Block, int) {
	return recurseTip(blk, 0)
}

func recurseTip(blk *Block, height int) (*Block, int) {
	if blk.NextBlock == nil {
		return blk, height
	} else {
		//		println(blk.Head.Nonce)
		return recurseTip(blk.NextBlock, height+1)
	}
}

// From wire common.go
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

// Prints out header from a given block
func printBlockHead(blk BlockHead) {

	prevhash, _ := wire.NewShaHash(blk.PrevHash[:])
	merkle, _ := wire.NewShaHash(blk.MerkleRoot[:])
	timestamp := time.Unix(int64(blk.Timestamp), 0)

	bh := wire.BlockHeader{
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
==============
`,
		hash, prevhash.String(),
		merkle.String(), timestamp,
		blk.Difficulty, blk.Nonce, blk.Length)
}

func walkBackwards(blk *Block) *Block {
	for {
		if blk.PrevBlock == nil {
			return blk
		}
		blk = blk.PrevBlock
	}
}
