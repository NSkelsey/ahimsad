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

	"github.com/conformal/btcwire"
)

var (
	blockdir = flag.String("blockdir", "/home/ubuntu/.bitcoin/testnet3/blocks", "The directory containing bitcoin blocks")
	logger   = log.New(os.Stdout, "", log.Llongfile)
	empt     = [32]byte{}
	//_genesisHash, _ = hex.DecodeString("43497fd7f826957108f4a30fd9cec3aeba79972084e90ead01ea330900000000")
	genesisHash = [32]byte{}
	maxBlocks   = 100000
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
			return false
		}
		discrim := binary.BigEndian.Uint32(b[:])
		if discrim != 0x00000000 {
			// seek backwards to start of block
			f.Seek(-4, 1)
			return true
		}
	}
}

func playWithFile(fname string, blkList []*Block, blkMap map[[32]byte]*Block) ([]*Block, map[[32]byte]*Block) {
	// given a blk file attempts to parse every block within it. Adding the block
	// to a global list of seen blocks. Additionally we strip out the interesting
	// transactions at this stage.
	file, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	genesis := false
	if len(blkList) == 0 {
		genesis = true
	}
	bad := 0
	for i := 0; i < maxBlocks; i++ {
		var blk Block
		var bh BlockHead

		ok := proceed(file)
		if !ok {
			fmt.Println("Hit end of file: ", fname)
		}
		err = binary.Read(file, binary.LittleEndian, &bh)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			fmt.Println("At the end of file: ", fname)
			break
		}
		if err != nil {
			logger.Fatal(err)
		}

		tx_num, err := readVarInt(file, 0)
		if err != nil {
			logger.Fatal(err)
		}

		//printBlockHead(bh)
		//fmt.Printf("num_tx:\t\t%d\n", tx_num)

		hash := blockHash(bh)

		for i := uint64(0); i < tx_num; i++ {
			tx := btcwire.MsgTx{}
			err := tx.Deserialize(file)
			if err != nil {
				logger.Fatal(err)
			}
		}

		if genesis {
			blk = Block{
				PrevBlock: nil,
				Head:      &bh,
				RelTxs:    make([]*btcwire.MsgTx, 0),
				Hash:      hash,
				depth:     0,
			}
			genesis = false
			fmt.Printf("The hash of the genesis block:\n%x\n", hash)
		} else {
			prevBlock, ok := blkMap[bh.PrevHash]
			if !ok {
				logger.Fatal("No prevhash in older block files")
			}
			// use last blk to find prevhash
			blk = Block{
				PrevBlock: prevBlock,
				Head:      &bh,
				RelTxs:    make([]*btcwire.MsgTx, 0),
				Hash:      hash,
			}
		}
		blkMap[hash] = &blk
		blkList = append(blkList, &blk)
	}

	return blkList, blkMap
}

func calcHeight(blkList []*Block) int {
	// Computes the best chain's total height by starting from the latest blocks
	// and working pack to the genesis block.
	var blk *Block
	for j := len(blkList) - 1; j >= 0; j-- {
		blk = blkList[j]
		if blk.depth == 0 {
			blk.depth = 1
		}
		//if blk.PrevBlock == nil && blk.Hash == genesisHash {
		if blk.Hash == genesisHash {
			println("found genesis hash", j)
			break
		}
		nextD := blk.depth + 1
		if blk.PrevBlock != nil && blk.PrevBlock.depth < nextD {
			blk.PrevBlock.depth = nextD
		}
	}
	return blk.depth
}

func main() {
	flag.Parse()

	//copy(genesisHash[:], _genesisHash)

	glob := "/blk*.dat"

	blockfiles, err := filepath.Glob(*blockdir + glob)
	if err != nil {
		log.Fatal(err)
	}
	if len(blockfiles) < 1 {
		log.Fatal(errors.New("Could not find any blockfiles at " + *blockdir))
	}

	var blkList []*Block
	var blkMap map[[32]byte]*Block
	blkList = make([]*Block, 0, maxBlocks)
	blkMap = make(map[[32]byte]*Block)
	for _, filename := range blockfiles {
		println(filename)
		blkList, blkMap = playWithFile(filename, blkList, blkMap)
		fmt.Println("Processed:", len(blkList))
	}

	println("Finding blockchain height")
	h := calcHeight(blkList)
	println("Height: ", h)
}

// From btcwire common.go
// readVarInt reads a variable length integer from r and returns it as a uint64.
func readVarInt(r io.Reader, pver uint32) (uint64, error) {
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
	fmt.Printf("Hash:\t\t%s", hash)
	fmt.Printf(`
prevHash:	%s
merkle root:	%s
timestamp:	%s
difficulty:	%d
nonce:		%d
bit len:	%d
`,
		prevhash.String(), merkle.String(), timestamp, blk.Difficulty, blk.Nonce, blk.Length)
}
