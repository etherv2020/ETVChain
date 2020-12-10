// Copyright 2016 The go-etvchaineum Authors
// This file is part of the go-etvchaineum library.
//
// The go-etvchaineum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-etvchaineum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-etvchaineum library. If not, see <http://www.gnu.org/licenses/>.

// Package les implements the Light Etvchain Subprotocol.
package les

import (
	"fmt"
	"sync"
	"time"

	"github.com/etvchaineum/go-etvchaineum/accounts"
	"github.com/etvchaineum/go-etvchaineum/common"
	"github.com/etvchaineum/go-etvchaineum/common/hexutil"
	"github.com/etvchaineum/go-etvchaineum/consensus"
	"github.com/etvchaineum/go-etvchaineum/core"
	"github.com/etvchaineum/go-etvchaineum/core/bloombits"
	"github.com/etvchaineum/go-etvchaineum/core/rawdb"
	"github.com/etvchaineum/go-etvchaineum/core/types"
	"github.com/etvchaineum/go-etvchaineum/ech"
	"github.com/etvchaineum/go-etvchaineum/ech/downloader"
	"github.com/etvchaineum/go-etvchaineum/ech/filters"
	"github.com/etvchaineum/go-etvchaineum/ech/gasprice"
	"github.com/etvchaineum/go-etvchaineum/event"
	"github.com/etvchaineum/go-etvchaineum/internal/echapi"
	"github.com/etvchaineum/go-etvchaineum/light"
	"github.com/etvchaineum/go-etvchaineum/log"
	"github.com/etvchaineum/go-etvchaineum/node"
	"github.com/etvchaineum/go-etvchaineum/p2p"
	"github.com/etvchaineum/go-etvchaineum/p2p/discv5"
	"github.com/etvchaineum/go-etvchaineum/params"
	rpc "github.com/etvchaineum/go-etvchaineum/rpc"
)

type LightEtvchain struct {
	lesCommons

	odr         *LesOdr
	relay       *LesTxRelay
	chainConfig *params.ChainConfig
	// Channel for shutting down the service
	shutdownChan chan bool

	// Handlers
	peers      *peerSet
	txPool     *light.TxPool
	blockchain *light.LightChain
	serverPool *serverPool
	reqDist    *requestDistributor
	retriever  *retrieveManager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer

	ApiBackend *LesApiBackend

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	networkId     uint64
	netRPCService *echapi.PublicNetAPI

	wg sync.WaitGroup
}

func New(ctx *node.ServiceContext, config *ech.Config) (*LightEtvchain, error) {
	chainDb, err := ech.CreateDB(ctx, config, "lightchaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, config.ConstantinopleOverride)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newPeerSet()
	quitSync := make(chan struct{})

	lech := &LightEtvchain{
		lesCommons: lesCommons{
			chainDb: chainDb,
			config:  config,
			iConfig: light.DefaultClientIndexerConfig,
		},
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		peers:          peers,
		reqDist:        newRequestDistributor(peers, quitSync),
		accountManager: ctx.AccountManager,
		engine:         ech.CreateConsensusEngine(ctx, chainConfig, &config.Ethash, nil, false, chainDb),
		shutdownChan:   make(chan bool),
		networkId:      config.NetworkId,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   ech.NewBloomIndexer(chainDb, params.BloomBitsBlocksClient, params.HelperTrieConfirmations),
	}

	lech.relay = NewLesTxRelay(peers, lech.reqDist)
	lech.serverPool = newServerPool(chainDb, quitSync, &lech.wg)
	lech.retriever = newRetrieveManager(peers, lech.reqDist, lech.serverPool)

	lech.odr = NewLesOdr(chainDb, light.DefaultClientIndexerConfig, lech.retriever)
	lech.chtIndexer = light.NewChtIndexer(chainDb, lech.odr, params.CHTFrequencyClient, params.HelperTrieConfirmations)
	lech.bloomTrieIndexer = light.NewBloomTrieIndexer(chainDb, lech.odr, params.BloomBitsBlocksClient, params.BloomTrieFrequency)
	lech.odr.SetIndexers(lech.chtIndexer, lech.bloomTrieIndexer, lech.bloomIndexer)

	// Note: NewLightChain adds the trusted checkpoint so it needs an ODR with
	// indexers already set but not started yet
	if lech.blockchain, err = light.NewLightChain(lech.odr, lech.chainConfig, lech.engine); err != nil {
		return nil, err
	}
	// Note: AddChildIndexer starts the update process for the child
	lech.bloomIndexer.AddChildIndexer(lech.bloomTrieIndexer)
	lech.chtIndexer.Start(lech.blockchain)
	lech.bloomIndexer.Start(lech.blockchain)

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		lech.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	lech.txPool = light.NewTxPool(lech.chainConfig, lech.blockchain, lech.relay)
	if lech.protocolManager, err = NewProtocolManager(lech.chainConfig, light.DefaultClientIndexerConfig, true, config.NetworkId, lech.eventMux, lech.engine, lech.peers, lech.blockchain, nil, chainDb, lech.odr, lech.relay, lech.serverPool, quitSync, &lech.wg); err != nil {
		return nil, err
	}
	lech.ApiBackend = &LesApiBackend{lech, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.MinerGasPrice
	}
	lech.ApiBackend.gpo = gasprice.NewOracle(lech.ApiBackend, gpoParams)
	return lech, nil
}

func lesTopic(genesisHash common.Hash, protocolVersion uint) discv5.Topic {
	var name string
	switch protocolVersion {
	case lpv1:
		name = "LES"
	case lpv2:
		name = "LES2"
	default:
		panic(nil)
	}
	return discv5.Topic(name + "@" + common.Bytes2Hex(genesisHash.Bytes()[0:8]))
}

type LightDummyAPI struct{}

// Etvchainbase is the address that mining rewards will be send to
func (s *LightDummyAPI) Etvchainbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("not supported")
}

// Coinbase is the address that mining rewards will be send to (alias for Etvchainbase)
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("not supported")
}

// Hashrate returns the POW hashrate
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}

// Mining returns an indication if this node is currently mining.
func (s *LightDummyAPI) Mining() bool {
	return false
}

// APIs returns the collection of RPC services the etvchaineum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *LightEtvchain) APIs() []rpc.API {
	return append(echapi.GetAPIs(s.ApiBackend), []rpc.API{
		{
			Namespace: "ech",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "ech",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "ech",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *LightEtvchain) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *LightEtvchain) BlockChain() *light.LightChain      { return s.blockchain }
func (s *LightEtvchain) TxPool() *light.TxPool              { return s.txPool }
func (s *LightEtvchain) Engine() consensus.Engine           { return s.engine }
func (s *LightEtvchain) LesVersion() int                    { return int(ClientProtocolVersions[0]) }
func (s *LightEtvchain) Downloader() *downloader.Downloader { return s.protocolManager.downloader }
func (s *LightEtvchain) EventMux() *event.TypeMux           { return s.eventMux }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *LightEtvchain) Protocols() []p2p.Protocol {
	return s.makeProtocols(ClientProtocolVersions)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Etvchain protocol implementation.
func (s *LightEtvchain) Start(srvr *p2p.Server) error {
	log.Warn("Light client mode is an experimental feature")
	s.startBloomHandlers(params.BloomBitsBlocksClient)
	s.netRPCService = echapi.NewPublicNetAPI(srvr, s.networkId)
	// clients are searching for the first advertised protocol in the list
	protocolVersion := AdvertiseProtocolVersions[0]
	s.serverPool.start(srvr, lesTopic(s.blockchain.Genesis().Hash(), protocolVersion))
	s.protocolManager.Start(s.config.LightPeers)
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Etvchain protocol.
func (s *LightEtvchain) Stop() error {
	s.odr.Stop()
	s.bloomIndexer.Close()
	s.chtIndexer.Close()
	s.blockchain.Stop()
	s.protocolManager.Stop()
	s.txPool.Stop()
	s.engine.Close()

	s.eventMux.Stop()

	time.Sleep(time.Millisecond * 200)
	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}
