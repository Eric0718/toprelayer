package msg

import (
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
)

func TestEncodeAndDecode(t *testing.T) {
	var headers []*TopElectBlockHeader
	for i := 1; i < 5; i++ {
		headers = append(headers, &TopElectBlockHeader{BlockNumber: uint64(i)})
	}
	t.Log("before encode:", headers[2].BlockNumber)
	buff, err := rlp.EncodeToBytes(&headers)
	if err != nil {
		t.Fatal("EncodeToBytes:", err)
	}

	var deHeaders []*TopElectBlockHeader
	err = rlp.DecodeBytes(buff, &deHeaders)
	if err != nil {
		t.Fatal("DecodeBytes:", deHeaders)
	}
	t.Log("after decode:", deHeaders[2].BlockNumber)
}
