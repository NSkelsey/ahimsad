package main

import (
	"bytes"
	"fmt"

	"code.google.com/p/goprotobuf/proto"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcwire"
)

var (
	ProtocolVersion uint32 = 0x1
	Magic                  = [8]byte{
		0x42, 0x52, 0x45, 0x54, 0x48, 0x52, 0x45, 0x4e, /* | BRETHREN | */
	}
)

type Author string

type Bulletin struct {
	txid    *btcwire.ShaHash
	block   *btcwire.ShaHash
	Author  string
	Topic   string
	Message string
}

func extractData(txOuts []*btcwire.TxOut) ([]byte, error) {
	// Munges the pushed data of TxOuts into a single universal slice that we can
	// use as whole message.

	alldata := make([]byte, 0)

	first := true
	for _, txout := range txOuts {

		pushMatrix, err := btcscript.PushedData(txout.PkScript)
		if err != nil {
			return alldata, err
		}
		for _, pushedD := range pushMatrix {
			if len(pushedD) != 20 {
				return alldata, fmt.Errorf("Pushed Data is not the right length")
			}

			alldata = append(alldata, pushedD...)
			if first {
				// Check to see if magic bytes match and slice accordingly
				first = false
				lenM := len(Magic)
				if !bytes.Equal(alldata[:lenM], Magic[:]) {
					return alldata, fmt.Errorf("Magic bytes don't match, Saw: [% x]", alldata[:lenM])
				}
				alldata = alldata[lenM:]
			}

		}

	}
	return alldata, nil
}

func NewBulletin(tx *btcwire.MsgTx, author string, blkhash *btcwire.ShaHash) (*Bulletin, error) {
	// Creates a new bulletin from the containing Tx, supplied author and optional blockhash

	// unpack txOuts that are considered data, We are going to ignore extra junk at the end of data
	wireBltn := &WireBulletin{}

	bytes, err := extractData(tx.TxOut)
	if err != nil {
		return nil, err
	}

	err = proto.Unmarshal(bytes, wireBltn)
	if err != nil {
		return nil, err
	}

	hash, _ := tx.TxSha()
	bltn := &Bulletin{
		txid:    &hash,
		block:   blkhash,
		Author:  author,
		Topic:   wireBltn.GetTopic(),
		Message: wireBltn.GetMessage(),
	}

	return bltn, nil
}

func NewBulletinFromStr(author string, topic string, msg string) (*Bulletin, error) {
	if len(topic) > 30 {
		return nil, fmt.Errorf("Topic too long! Length is: %d", len(topic))
	}

	if len(msg) > 500 {
		return nil, fmt.Errorf("Message too long! Length is: %d", len(msg))
	}

	bulletin := Bulletin{
		Author:  author,
		Topic:   topic,
		Message: msg,
	}
	return &bulletin, nil
}

func (bltn *Bulletin) TxOuts() ([]*btcwire.TxOut, error) {
	// returns the slices of bytes to encode
	/*	pbyte, err := proto.Marshal(bltn.raw)
		if err != nil {
			return []*btcwire.TxOut{}, err
		}
		log.Printf("%x\n", pbyte)
		// Take 20 byte chunks and decode
	*/

	return []*btcwire.TxOut{}, nil
}
