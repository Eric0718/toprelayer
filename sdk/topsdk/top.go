package topsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"toprelayer/msg"
	"toprelayer/sdk"
	"toprelayer/util"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type TopSdk struct {
	*sdk.SDK
	url string
}

type ElectBlockType = uint

const (
	ElectBlock_Current ElectBlockType = iota
	ElectBlock_Next
)

const (
	ETHGETTRANSACTIONBYHASH        = "eth_getTransactionByHash"
	ETHGETTRANSACTIONRECEIPT       = "eth_getTransactionReceipt"
	GETLATESTETTOPELECTBLOCKHEADER = "getLatestTopElectBlockHeader"
	GETTOPELECTBLOCKHEADBYHEIGHT   = "getTopElectBlockHeadByHeight"
	GETLATESTTOPELECTBLOCKHEIGHT   = "getLatestTopElectBlockHeight"
)

type Transaction struct {
	BlockHash        common.Hash    `json:"blockHash"`
	BlockNumber      string         `json:"blockNumber"`
	From             common.Address `json:"from"`
	Gas              string         `json:"gas"`
	GasPrice         string         `json:"gasPrice"`
	Hash             common.Hash    `json:"hash"`
	Input            string         `json:"input"`
	Nonce            string         `json:"nonce"`
	To               common.Address `json:"to"`
	TransactionIndex string         `json:"transactionIndex"`
	Value            string         `json:"value"`
	V                string         `json:"v"`
	R                common.Hash    `json:"r"`
	S                common.Hash    `json:"S"`
}

type TransactionReceipt struct {
	BlockHash         common.Hash    `json:"blockHash"`
	BlockNumber       string         `json:"blockNumber"`
	ContractAddress   common.Address `json:"contractAddress"`
	CumulativeGasUsed string         `json:"cumulativeGasUsed"`
	From              common.Address `json:"from"`
	GasUsed           string         `json:"gasUsed"`
	Logs              []*types.Log   `json:"logs"`
	LogsBloom         types.Bloom    `json:"logsBloom"`
	Status            string         `json:"status"`
	To                common.Address `json:"to"`

	TransactionHash  common.Hash `json:"transactionHash"`
	TransactionIndex string      `json:"transactionIndex"`

	Root common.Hash `json:"root"`
}

func NewTopSdk(url string) (*TopSdk, error) {
	sdk, err := sdk.NewSDK(url)
	if err != nil {
		return nil, err
	}
	return &TopSdk{SDK: sdk, url: url}, nil
}

func (t *TopSdk) SaveEthBlockHead(rawTx string) (string, error) {
	etx, err := util.DecodeRawTx(rawTx)
	if err != nil {
		return "", err
	}
	err = t.SendTransaction(context.Background(), etx)
	if err != nil {
		return "", err
	}

	return etx.Hash().Hex(), nil
}

func (t *TopSdk) GetLatestTopElectBlockHeader() (*types.Block, error) {
	return t.getLatestTopElectBlockHeader()
}

func (t *TopSdk) getLatestTopElectBlockHeader() (*types.Block, error) {
	var data json.RawMessage
	err := t.Rpc.CallContext(context.Background(), &data, GETLATESTETTOPELECTBLOCKHEADER)
	if err != nil {
		return nil, err
	} else if len(data) == 0 {
		return nil, ethereum.NotFound
	}
	// Decode header and transactions.
	var head *types.Block
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	return head, nil
}

func (t *TopSdk) GetTransactionByHash(hash common.Hash) (isPending bool, err error) {
	var tx *Transaction
	err = t.Rpc.CallContext(context.Background(), tx, ETHGETTRANSACTIONBYHASH, hash)
	if err != nil {
		return false, err
	} else if tx == nil {
		return false, ethereum.NotFound
	}

	return tx.BlockNumber == "", nil
}

func (t *TopSdk) GetTransactionReceipt(hash common.Hash) (*TransactionReceipt, error) {
	var r *TransactionReceipt
	err := t.Rpc.CallContext(context.Background(), &r, ETHGETTRANSACTIONRECEIPT, hash)
	if err == nil {
		if r == nil {
			return nil, ethereum.NotFound
		}
	}
	return r, err
}

func (t *TopSdk) GetTopElectBlockHeadByHeight(height uint64, tag ElectBlockType) (*msg.TopElectBlockHeader, error) {
	return t.getTopElectBlockHeadByHeight(height, tag)
}

func (t *TopSdk) getTopElectBlockHeadByHeight(height uint64, tag ElectBlockType) (*msg.TopElectBlockHeader, error) {
	if tag == ElectBlock_Current || tag == ElectBlock_Next {
		var data json.RawMessage
		err := t.Rpc.CallContext(context.Background(), &data, GETTOPELECTBLOCKHEADBYHEIGHT, util.Uint64ToHexString(height), util.Uint64ToHexString(uint64(tag)))
		if err != nil {
			return nil, err
		} else if len(data) == 0 {
			return nil, ethereum.NotFound
		}
		// Decode header and transactions.
		var head msg.TopElectBlockHeader
		if err := json.Unmarshal(data, &head); err != nil {
			return nil, err
		}
		return &head, nil
	}
	return nil, fmt.Errorf("Unsupport tag:%v,expect[%v or %v]", tag, ElectBlock_Current, ElectBlock_Next)
}

func (t *TopSdk) GetLatestTopElectBlockHeight() (uint64, error) {
	return t.getLatestTopElectBlockHeight()
}

func (t *TopSdk) getLatestTopElectBlockHeight() (uint64, error) {
	var data json.RawMessage
	err := t.Rpc.CallContext(context.Background(), &data, GETLATESTTOPELECTBLOCKHEIGHT)
	if err != nil {
		return 0, err
	} else if len(data) == 0 {
		return 0, ethereum.NotFound
	}

	//var res string
	var res string
	if err := json.Unmarshal(data, &res); err != nil {
		log.Printf("sdk getLatestTopElectBlockHeight data: %v,error:%v", string(data), err)
		return 0, err
	}
	return util.HexToUint64(res)
}
