package relayer

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"toprelayer/config"
)

type HeaderSyncHandler struct {
	ctx      context.Context
	wg       *sync.WaitGroup
	relayers map[uint64]IChainRelayer

	conf *config.HeaderSyncConfig
}

func NewHeaderSyncHandler(config *config.HeaderSyncConfig) *HeaderSyncHandler {
	var handler HeaderSyncHandler
	relayers := make(map[uint64]IChainRelayer)

	for _, chain := range config.Config.Chains {
		relayers[chain.ChainId_to] = GetRelayer(chain.ChainId_to)
	}
	handler.relayers = relayers
	handler.conf = config

	return &handler
}

func (h *HeaderSyncHandler) Init(wg *sync.WaitGroup, chainpass map[uint64]string) (err error) {
	h.wg = wg
	for _, chain := range h.conf.Config.Chains {
		err = h.relayers[chain.ChainId_to].Init(
			chain.SubmitNode,
			chain.ListenNode,
			chain.KeyPath,
			chainpass[chain.ChainId_to],
			chain.ChainId_to,
			common.HexToAddress(chain.Contract),
			chain.BlockCertainty,
			chain.SubBatch,
		)
		if err != nil {
			return err
		}
	}
	return
}

func (h *HeaderSyncHandler) StartRelayer() (err error) {
	for _, chain := range h.conf.Config.Chains {
		h.wg.Add(1)
		go func() {
			err = h.relayers[chain.ChainId_to].StartSubmitting(h.wg)
		}()
		if err != nil {
			return err
		}
		time.Sleep(time.Second * 5)
	}
	return nil
}
