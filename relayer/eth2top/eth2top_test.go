package eth2top

import (
	"testing"
	"toprelayer/base"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const SUBMITTERURL string = "http://0.0.0.0:37399"

var DEFAULTPATH = "../../.relayer/wallet/top"
var CONTRACT common.Address = common.HexToAddress("0xa6D2b331B03fdDB8c6A8830A63fE47E42c4bDF4E")

func TestSubmitHeader(t *testing.T) {
	sub := &TopSubmitter{} //new(TopSubmitter)
	err := sub.Init(SUBMITTERURL, "", DEFAULTPATH, base.ETH, CONTRACT, 0, 0, 0, 0, 0, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	head := &types.Header{}
	header, err := head.MarshalJSON()
	if err != nil {
		t.Fatal("MarshalJSON error:", err)
	}
	if sub.wallet == nil {
		t.Fatal("nil wallet!!!")
	}
	hash, err := sub.submitEthHeader(header, 0)
	if err != nil {
		t.Fatal("SubmitHeader error:", err)
	}
	t.Log("SubmitHeader hash:", hash)
}
