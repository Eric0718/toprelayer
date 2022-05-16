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
	METHOD_GETCURRENTBLOCKHEIGHT       = "getCurrentBlockHeight"
	SUBMITINTERVAL               int64 = 1000
	ERRDELAY                     int64 = 18
)

type Eth2Top struct {
	context.Context
	contract        common.Address
	chainId         uint64
	wallet          wallet.IWallet_TOP
	topsdk          *topsdk.TopSdk
	ethsdk          *ethsdk.EthSdk
	certaintyBlocks int
	subBatch        int
}

func (et *Eth2Top) Init(topUrl, ethUrl, keypath, pass string, chainid uint64, contract common.Address, batch, cert int) error {
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

	w, err := wallet.NewWallet(topUrl, keypath, pass, chainid)
	if err != nil {
		return err
	}
	et.wallet = w.(wallet.IWallet_TOP)
	return nil
}
func (et *Eth2Top) ChainId() uint64 {
	return et.chainId
}

func (et *Eth2Top) submitEthHeader(header []byte, nonce uint64) (*types.Transaction, error) {
	logger.Debug("submitEthHeader header length:%v,chainid:%v", len(header), et.chainId)
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
	fmt.Printf("EthSubmitter debug: account balance:%v", balance.Uint64())
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
		logger.Error("AddLightClientBlock:%v", err)
		return nil, err
	}

	if ops.NoSend {
		err = util.VerifyEthSignature(sigTx)
		if err != nil {
			logger.Error("VerifyEthSignature:%v", err)
			return nil, err
		}

		err := et.topsdk.SendTransaction(ops.Context, sigTx)
		if err != nil {
			logger.Error("SendTransaction:%v", err)
			return nil, err
		}
	}
	return sigTx, nil
}

//callback function to sign tx before send.
func (et *Eth2Top) signTransaction(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
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

func (et *Eth2Top) getContractLatestsyncedHeight() (uint64, error) {
	hscaller, err := bridge.NewBridgeCaller(et.contract, et.topsdk)
	if err != nil {
		return 0, err
	}

	hscRaw := bridge.BridgeCallerRaw{Contract: hscaller}
	result := make([]interface{}, 1)
	err = hscRaw.Call(nil, &result, METHOD_GETCURRENTBLOCKHEIGHT, et.chainId)
	if err != nil {
		return 0, err
	}

	h, success := result[0].(uint64)
	if !success {
		return 0, fmt.Errorf("fail to convert error")
	}

	return h, nil
}

func (et *Eth2Top) StartSubmitting(wg *sync.WaitGroup) error {
	logger.Info("Start Eth2Toprelayer... chainid:%v", et.chainId)
	defer wg.Done()

	var submitDelay int64 = 1

	//test mock
	var bridgeLatestSyncedHeight uint64 = 0
	for {
		// bridgeLatestSyncedHeight, err := et.getContractLatestsyncedHeight()
		// if err != nil {
		// 	logger.Error(err)
		// 	return err
		// }

		ethCurrentHeight, err := et.ethsdk.BlockNumber(context.Background())
		if err != nil {
			logger.Error(err)
			return err
		}

		chainConfirmedBlockHeight := ethCurrentHeight - uint64(et.certaintyBlocks)
		if bridgeLatestSyncedHeight+1 <= chainConfirmedBlockHeight {
			hashes, err := et.signAndSendTransactions(bridgeLatestSyncedHeight+1, chainConfirmedBlockHeight)
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
		//time.Sleep(time.Second * time.Duration(SUBMITINTERVAL*submitDelay))
		time.Sleep(time.Second * 2)
	}
}

func (et *Eth2Top) batch(headers []*types.Header, nonce uint64) (common.Hash, error) {
	data, err := msg.EncodeHeaders(headers)
	if err != nil {
		logger.Error("RlpEncodeHeaders failed:", err)
		return common.Hash{}, err
	}
	tx, err := et.submitEthHeader(data, nonce)
	if err != nil {
		logger.Error("submitHeaders failed:", err)
		return common.Hash{}, err
	}
	return tx.Hash(), nil
}

func (et *Eth2Top) signAndSendTransactions(lo, hi uint64) ([]common.Hash, error) {
	var batchHeaders []*types.Header
	var hashes []common.Hash
	nonce, err := et.wallet.GetNonce(et.wallet.CurrentAccount().Address)
	if err != nil {
		return hashes, err
	}
	nums := (hi - lo + 1) / uint64(et.subBatch)

	for i := lo; i <= hi; i++ {
		header, err := et.topsdk.HeaderByNumber(context.Background(), big.NewInt(0).SetUint64(i))
		if err != nil {
			logger.Error(err)
			return hashes, err
		}
		batchHeaders = append(batchHeaders, header)
		if nums > 0 {
			if len(batchHeaders) == et.subBatch {
				hash, err := et.batch(batchHeaders, nonce)
				if err != nil {
					return hashes, err
				}
				batchHeaders = []*types.Header{}
				hashes = append(hashes, hash)
				nonce++
				nums--
				continue
			}
		}
		if nums == 0 {
			if len(batchHeaders) > 0 {
				hash, err := et.batch(batchHeaders, nonce)
				if err != nil {
					return hashes, err
				}
				batchHeaders = []*types.Header{}
				hashes = append(hashes, hash)
			}
		}
	}
	return hashes, nil
}
