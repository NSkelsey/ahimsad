package ahimsad

import (
	"fmt"
	"log"

	"code.google.com/p/goprotobuf/proto"
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
	// Raw format to encode into data outs
	raw    *WireBulletin
	Author Author
}

func NewBulletin(author Author, topic string, msg string) (*Bulletin, error) {
	if len(topic) > 30 {
		return nil, fmt.Errorf("Topic too long! Length is: %d", len(topic))
	}

	if len(msg) > 500 {
		return nil, fmt.Errorf("Message too long! Length is: %d", len(msg))
	}

	wirebltn := WireBulletin{
		Version: proto.Uint32(ProtocolVersion),
		Topic:   proto.String(topic),
		Message: proto.String(msg),
	}
	bulletin := Bulletin{
		raw:    &wirebltn,
		Author: author,
	}
	return &bulletin, nil
}

func (bltn *Bulletin) TxOuts() ([]*btcwire.TxOut, error) {
	// returns the slices of bytes to encode
	pbyte, err := proto.Marshal(bltn.raw)
	if err != nil {
		return []*btcwire.TxOut{}, err
	}
	log.Printf("%x\n", pbyte)

	return []*btcwire.TxOut{}, nil
}

func (bltn *Bulletin) GetMessage() string {
	return bltn.raw.GetMessage()
}

func (bltn *Bulletin) GetTopic() string {
	return bltn.raw.GetTopic()
}
