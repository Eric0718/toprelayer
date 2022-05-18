package eth2top

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
	"toprelayer/contract/top/bridge"
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
	METHOD_GETBRIDGESTATE        = "getbridgestate"
	SUBMITINTERVAL        int64  = 120
	ERRDELAY              int64  = 18
	CONFIRMSUCCESS        string = "0x1"
)

type Eth2TopRelayer struct {
	context.Context
	contract        common.Address
	chainId         uint64
	wallet          wallet.IWallet
	topsdk          *topsdk.TopSdk
	ethsdk          *ethsdk.EthSdk
	certaintyBlocks int
	subBatch        int
	verifyBlock     bool
}

func (et *Eth2TopRelayer) Init(topUrl, ethUrl, keypath, pass string, chainid uint64, contract common.Address, batch, cert int, verify bool) error {
	topsdk, err := topsdk.NewTopSdk(topUrl)
	if err != nil {
		return err
	}
	ethsdk, err := ethsdk.NewEthSdk(ethUrl)
	if err != nil {
		return err
	}

	et.topsdk = topsdk
	et.ethsdk = ethsdk
	et.contract = contract
	et.chainId = chainid
	et.subBatch = batch
	et.certaintyBlocks = cert
	et.verifyBlock = verify

	w, err := wallet.NewWallet(topUrl, keypath, pass, chainid)
	if err != nil {
		return err
	}
	et.wallet = w
	return nil
}
func (et *Eth2TopRelayer) ChainId() uint64 {
	return et.chainId
}

func (et *Eth2TopRelayer) submitEthHeader(header []byte, nonce uint64) (*types.Transaction, error) {
	logger.Debug("submitEthHeader length:%v,chainid:%v", len(header), et.chainId)
	gaspric, err := et.wallet.GasPrice(context.Background())
	if err != nil {
		return nil, err
	}

	/* msg := ethereum.CallMsg{
		From:     et.wallet.CurrentAccount().Address,
		To:       &et.contract,
		GasPrice: gaspric,
		Value:    big.NewInt(0),
		Data:     header,
	}

	gaslimit, err := et.wallet.EstimateGas(context.Background(), msg)
	if err != nil {
		return nil, err
	} */

	//test mock
	gaslimit := uint64(300000)

	/* balance, err := et.wallet.GetBalance()
	if err != nil {
		return nil, err
	}

	if balance.Uint64() <= gaspric.Uint64()*gaslimit {
		return nil, fmt.Errorf("account not sufficient funds,balance:%v", balance.Uint64())
	} */

	//must init ops as bellow
	ops := &bind.TransactOpts{
		From:     et.wallet.CurrentAccount().Address,
		Nonce:    big.NewInt(0).SetUint64(nonce),
		GasPrice: gaspric,
		GasLimit: gaslimit,
		Signer:   et.signTransaction,
		Context:  context.Background(),
		NoSend:   true, //false: Send the transaction to the target chain by default; true: don't send
	}

	contractcaller, err := bridge.NewBridgeTransactor(et.contract, et.topsdk)
	if err != nil {
		return nil, err
	}

	sigTx, err := contractcaller.AddLightClientBlock(ops, header)
	if err != nil {
		logger.Error("Eth2TopRelayer AddLightClientBlock:%v", err)
		return nil, err
	}

	if ops.NoSend {
		err = util.VerifyEthSignature(sigTx)
		if err != nil {
			logger.Error("Eth2TopRelayer VerifyEthSignature:%v", err)
			return nil, err
		}

		err := et.topsdk.SendTransaction(ops.Context, sigTx)
		if err != nil {
			logger.Error("Eth2TopRelayer SendTransaction:%v", err)
			return nil, err
		}
	}
	return sigTx, nil
}

//callback function to sign tx before send.
func (et *Eth2TopRelayer) signTransaction(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
	acc := et.wallet.CurrentAccount()
	if strings.EqualFold(acc.Address.Hex(), addr.Hex()) {
		stx, err := et.wallet.SignTx(tx)
		if err != nil {
			return nil, err
		}
		return stx, nil
	}
	return nil, fmt.Errorf("address:%v not available", addr)
}

func (et *Eth2TopRelayer) getTopBridgeState() (*msg.BridgeState, error) {
	hscaller, err := bridge.NewBridgeCaller(et.contract, et.topsdk)
	if err != nil {
		return nil, err
	}

	hscRaw := bridge.BridgeCallerRaw{Contract: hscaller}
	result := make([]interface{}, 1)

	err = hscRaw.Call(nil, &result, METHOD_GETBRIDGESTATE, et.chainId)
	if err != nil {
		return nil, err
	}
	state, success := result[0].(msg.BridgeState)
	if !success {
		return nil, err
	}
	return &state, nil
}

func (et *Eth2TopRelayer) StartRelayer(wg *sync.WaitGroup) error {
	logger.Info("Start Eth2TopRelayer... chainid:%v", et.chainId)
	defer wg.Done()

	var submitDelay int64 = 1
	var syncStartHeight uint64 = 1
	for {
		//time.Sleep(time.Second * time.Duration(SUBMITINTERVAL*submitDelay))
		time.Sleep(time.Second * 20)

		/* bridgeState, err := et.getTopBridgeState()
		if err != nil {
			logger.Error(err)
			continue
		}
		if bridgeState.ConfirmState == CONFIRMSUCCESS {
			syncStartHeight = bridgeState.LatestSyncedHeight.Uint64() + 1
		} else {
			logger.Warn("top bridge confirm eth header failed,height:%v.", bridgeState.LatestConfirmedHeight.Uint64())
			syncStartHeight = bridgeState.LatestConfirmedHeight.Uint64()
		} */

		ethCurrentHeight, err := et.ethsdk.BlockNumber(context.Background())
		if err != nil {
			logger.Error(err)
			continue
		}
		ethConfirmedBlockHeight := ethCurrentHeight - uint64(et.certaintyBlocks)

		logger.Debug("syncStartHeight:%v,ethCurrentHeight:%v,certaintyBlocks:%v,ethConfirmedBlockHeight:%v", syncStartHeight, ethCurrentHeight, et.certaintyBlocks, ethConfirmedBlockHeight)
		if syncStartHeight <= ethConfirmedBlockHeight {
			hashes, err := et.signAndSendTransactions(syncStartHeight, ethConfirmedBlockHeight)
			if len(hashes) > 0 {
				logger.Info("sent hashes:", hashes)
				submitDelay = int64(len(hashes))
			}
			if err != nil {
				logger.Error("Eth2TopRelayer signAndSendTransactions failed:%v,delay:%v", err, SUBMITINTERVAL*submitDelay)
				if len(hashes) > 0 {
					continue
				}
				submitDelay = ERRDELAY
			}
		} else {
			submitDelay = 1
		}
		//test mock
		syncStartHeight = ethConfirmedBlockHeight + 1
	}
}

func (et *Eth2TopRelayer) batch(headers []*types.Header, nonce uint64) (common.Hash, error) {
	if et.verifyBlock {
		for _, header := range headers {
			et.verifyBlocks(header)
		}
	}
	data, err := msg.EncodeHeaders(headers)
	if err != nil {
		logger.Error("Eth2TopRelayer EncodeHeaders failed:", err)
		return common.Hash{}, err
	}
	tx, err := et.submitEthHeader(data, nonce)
	if err != nil {
		logger.Error("Eth2TopRelayer submitHeaders failed:", err)
		return common.Hash{}, err
	}
	logger.Debug("nonce:%v,hash:%v", nonce, tx.Hash())
	return tx.Hash(), nil
}

func (et *Eth2TopRelayer) signAndSendTransactions(lo, hi uint64) ([]common.Hash, error) {
	var batchHeaders []*types.Header
	var hashes []common.Hash
	nonce, err := et.wallet.GetNonce(et.wallet.CurrentAccount().Address)
	if err != nil {
		return hashes, err
	}

	h := lo
	for ; h <= hi; h++ {
		header, err := et.ethsdk.HeaderByNumber(context.Background(), big.NewInt(0).SetUint64(h))
		if err != nil {
			logger.Error(err)
			return hashes, err
		}
		batchHeaders = append(batchHeaders, header)
		if (h-lo+1)%uint64(et.subBatch) == 0 {
			hash, err := et.batch(batchHeaders, nonce)
			if err != nil {
				return hashes, err
			}
			batchHeaders = []*types.Header{}
			hashes = append(hashes, hash)
			nonce++
		}
	}
	logger.Debug("check h:%v,hi:%v,len batchHeaders:%v", h, hi, len(batchHeaders))
	if h > hi {
		if len(batchHeaders) > 0 {
			hash, err := et.batch(batchHeaders, nonce)
			if err != nil {
				return hashes, err
			}
			batchHeaders = []*types.Header{}
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}

func (et *Eth2TopRelayer) verifyBlocks(header *types.Header) error {
	return nil
}
