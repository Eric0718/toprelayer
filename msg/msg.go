package msg

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

type TopElectBlockHeader struct {
	Hash        common.Hash `json:"hash"`
	BlockNumber uint64      `json:"blockNumber"`
}

func (t *TopElectBlockHeader) EncodeHeader() ([]byte, error) {
	return rlp.EncodeToBytes(t)
}

func EncodeHeaders(headers interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(headers)
}
