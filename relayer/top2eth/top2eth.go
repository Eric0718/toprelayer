package top2eth

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
	"toprelayer/contract/eth/hsc"
	"toprelayer/msg"
	"toprelayer/sdk/ethsdk"
	"toprelayer/sdk/topsdk"
	"toprelayer/util"
	"toprelayer/wallet"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/wonderivan/logger"
)

const (
	METHOD_GETCURRENTBLOCKHEIGHT       = "getCurrentBlockHeight"
	SUBMITINTERVAL               int64 = 1000
	ERRDELAY                     int64 = 18
)

type Top2Eth struct {
	context.Context
	contract        common.Address
	chainId         uint64
	wallet          wallet.IWallet_ETH
	ethsdk          *ethsdk.EthSdk
	topsdk          *topsdk.TopSdk
	certaintyBlocks int
	subBatch        int
}

func (te *Top2Eth) Init(ethUrl, topUrl, keypath, pass string, chainid uint64, contract common.Address, batch, cert int) error {
	ethsdk, err := ethsdk.NewEthSdk(ethUrl)
	if err != nil {
		return err
	}
	topsdk, err := topsdk.NewTopSdk(topUrl)
	if err != nil {
		return err
	}
	te.topsdk = topsdk
	te.ethsdk = ethsdk
	te.contract = contract
	te.chainId = chainid
	te.subBatch = batch
	te.certaintyBlocks = cert

	w, err := wallet.NewWallet(ethUrl, keypath, pass, chainid)
	if err != nil {
		return err
	}
	te.wallet = w.(wallet.IWallet_ETH)
	return nil
}

func (te *Top2Eth) ChainId() uint64 {
	return te.chainId
}

func (te *Top2Eth) submitTopHeader(headers []byte, nonce uint64) (*types.Transaction, error) {
	logger.Info("submitHeader header length:%v,chainid:%v", len(headers), te.chainId)
	gaspric, err := te.wallet.GasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	/* msg := ethereum.CallMsg{
		From:     te.wallet.CurrentAccount().Address,
		To:       &te.contract,
		GasPrice: gaspric,
		Value:    big.NewInt(0),
		Data:     header,
	}

	gaslimit, err := te.wallet.EstimateGas(context.Background(), msg)
	if err != nil {
		fmt.Println("EstimateGas error:", err)
		return "", err
	}
	*/

	//test mock
	gaslimit := uint64(300000)

	balance, err := te.wallet.GetBalance()
	if err != nil {
		return nil, err
	}
	fmt.Printf("EthSubmitter debug: account balance:%v", balance.Uint64())
	if balance.Uint64() <= gaspric.Uint64()*gaslimit {
		return nil, fmt.Errorf("account not sufficient funds,balance:%v", balance.Uint64())
	}

	balance, err = te.wallet.GetBalance()
	if err != nil {
		return nil, err
	}
	logger.Info("account balance:%v", balance.Uint64())
	if balance.Uint64() <= gaspric.Uint64()*gaslimit {
		return nil, fmt.Errorf("account[%v] not sufficient funds,balance:%v", te.wallet.CurrentAccount().Address, balance.Uint64())
	}

	//must init ops as bellow
	ops := &bind.TransactOpts{
		From:     te.wallet.CurrentAccount().Address,
		Nonce:    big.NewInt(0).SetUint64(nonce),
		GasPrice: gaspric,
		GasLimit: gaslimit,
		Signer:   te.signTransaction,
		Context:  context.Background(),
		NoSend:   true, //false: Send the transaction to the target chain by default; true: don't send
	}

	contractcaller, err := hsc.NewHscTransactor(te.contract, te.ethsdk)
	if err != nil {
		logger.Error("NewBridgeTransactor:", err)
		return nil, err
	}

	sigTx, err := contractcaller.SyncBlockHeader(ops, headers)
	if err != nil {
		logger.Error("AddLightClientBlock error:", err)
		return nil, err
	}

	if ops.NoSend {
		err = util.VerifyEthSignature(sigTx)
		if err != nil {
			logger.Error("VerifyEthSignature error:", err)
			return nil, err
		}

		err := te.ethsdk.SendTransaction(ops.Context, sigTx)
		if err != nil {
			logger.Error("SendTransaction error:", err)
			return nil, err
		}
	}
	return sigTx, nil
}

//callback function to sign tx before send.
func (te *Top2Eth) signTransaction(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
	acc := te.wallet.CurrentAccount()
	if strings.EqualFold(acc.Address.Hex(), addr.Hex()) {
		stx, err := te.wallet.SignTx(tx)
		if err != nil {
			return nil, err
		}
		return stx, nil
	}
	return nil, fmt.Errorf("address:%v not available", addr)
}

func (te *Top2Eth) getContractLatestsyncedHeight() (uint64, error) {
	hscaller, err := hsc.NewHscCaller(te.contract, te.ethsdk)
	if err != nil {
		return 0, err
	}

	hscRaw := hsc.HscCallerRaw{Contract: hscaller}
	result := make([]interface{}, 1)
	err = hscRaw.Call(nil, &result, METHOD_GETCURRENTBLOCKHEIGHT, te.chainId)
	if err != nil {
		return 0, err
	}

	h, success := result[0].(uint64)
	if !success {
		return 0, fmt.Errorf("fail to convert error")
	}

	return h, nil
}

func (te *Top2Eth) StartSubmitting(wg *sync.WaitGroup) error {
	logger.Info("Start Top2Eth relayer... chainid:%v", te.chainId)
	defer wg.Done()

	var submitDelay int64 = 1
	for {
		bridgeLatestSyncedHeight, err := te.getContractLatestsyncedHeight()
		if err != nil {
			logger.Error(err)
			return err
		}

		topCurrentHeight, err := te.topsdk.GetLatestTopElectBlockHeight()
		if err != nil {
			logger.Error(err)
			return err
		}

		chainConfirmedBlockHeight := topCurrentHeight - 2 - uint64(te.certaintyBlocks)
		if bridgeLatestSyncedHeight+1 <= chainConfirmedBlockHeight {
			hashes, err := te.signAndSendTransactions(bridgeLatestSyncedHeight+1, chainConfirmedBlockHeight)
			if len(hashes) > 0 {
				logger.Info("sent hashes:", hashes)
				submitDelay = int64(len(hashes))
			}
			if err != nil {
				logger.Error("signAndSendTransactions failed:%v,delay:%v", err, SUBMITINTERVAL*submitDelay)
				submitDelay = ERRDELAY
			}
		} else {
			submitDelay = 1
		}
		time.Sleep(time.Second * time.Duration(SUBMITINTERVAL*submitDelay))
	}
}

func (te *Top2Eth) batch(headers []*msg.TopElectBlockHeader, nonce uint64) (common.Hash, error) {
	data, err := msg.EncodeHeaders(headers)
	if err != nil {
		logger.Error("RlpEncodeHeaders failed:", err)
		return common.Hash{}, err
	}

	tx, err := te.submitTopHeader(data, nonce)
	if err != nil {
		logger.Error("submitHeaders failed:", err)
		return common.Hash{}, err
	}
	return tx.Hash(), nil
}

func (te *Top2Eth) signAndSendTransactions(lo, hi uint64) ([]common.Hash, error) {
	var batchHeaders []*msg.TopElectBlockHeader
	var hashes []common.Hash
	nonce, err := te.wallet.GetNonce(te.wallet.CurrentAccount().Address)
	if err != nil {
		return hashes, err
	}
	nums := (hi - lo + 1) / uint64(te.subBatch)

	for i := lo; i <= hi; i++ {
		header, err := te.topsdk.GetTopElectBlockHeadByHeight(i, topsdk.ElectBlock_Current)
		if err != nil {
			logger.Error(err)
			return hashes, err
		}
		batchHeaders = append(batchHeaders, header)
		if nums > 0 {
			if len(batchHeaders) == te.subBatch {
				hash, err := te.batch(batchHeaders, nonce)
				if err != nil {
					return hashes, err
				}
				batchHeaders = []*msg.TopElectBlockHeader{}
				hashes = append(hashes, hash)
				nonce++
				nums--
				continue
			}
		}
		if nums == 0 {
			if len(batchHeaders) > 0 {
				hash, err := te.batch(batchHeaders, nonce)
				if err != nil {
					return hashes, err
				}
				batchHeaders = []*msg.TopElectBlockHeader{}
				hashes = append(hashes, hash)
			}
		}
	}
	return hashes, nil
}
