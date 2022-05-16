package top2eth

import (
	"testing"
	"toprelayer/base"
	"toprelayer/msg"

	"github.com/ethereum/go-ethereum/common"
)

const SUBMITTERURL string = "http://192.168.30.32:8545"
const LISTENURL string = "http://0.0.0.0:37399"

var DEFAULTPATH = "../../.relayer/wallet/eth"

var CONTRACT common.Address = common.HexToAddress("0xb12138ce2c9e801f28c66b08bd8a2fd27fa50b89")

func TestSubmitHeader(t *testing.T) {
	sub := new(EthSubmitter)
	err := sub.Init(SUBMITTERURL, LISTENURL, DEFAULTPATH, "", base.ETH, CONTRACT, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	header := &msg.TopElectBlockHeader{}
	data, _ := header.EncodeHeader()
	hash, err := sub.submitTopHeader(data, 0)
	if err != nil {
		t.Fatal("SubmitHeader error:", err)
	}
	t.Log("SubmitHeader hash:", hash)
}
