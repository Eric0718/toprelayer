package top2eth

import (
	"testing"
	"toprelayer/base"
	"toprelayer/msg"

	"github.com/ethereum/go-ethereum/common"
)

const SUBMITTERURL string = "http://192.168.30.32:8545"

var DEFAULTPATH = "../../.relayer/wallet/eth"

var CONTRACT common.Address = common.HexToAddress("0xb12138ce2c9e801f28c66b08bd8a2fd27fa50b89")

func TestSubmitHeader(t *testing.T) {
	sub := new(Top2EthRelayer)
	err := sub.Init(SUBMITTERURL, SUBMITTERURL, DEFAULTPATH, "", base.ETH, CONTRACT, 10, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	var headers []*msg.TopElectBlockHeader
	for i := 1; i <= 100; i++ {
		headers = append(headers, &msg.TopElectBlockHeader{BlockNumber: uint64(i)})
	}
	data, _ := msg.EncodeHeader(headers)
	nonce, _ := sub.wallet.GetNonce(sub.wallet.CurrentAccount().Address)
	tx, err := sub.submitTopHeader(data, nonce)
	if err != nil {
		t.Fatal("SubmitHeader error:", err)
	}
	t.Log("SubmitHeader hash:", tx.Hash())
}
