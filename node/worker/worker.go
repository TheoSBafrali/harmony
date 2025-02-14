package worker

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"

	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
)

// environment is the worker's current environment and holds all of the current state information.
type environment struct {
	state   *state.DB     // apply state changes here
	gasPool *core.GasPool // available gas used to pack transactions

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt
	outcxs   []*types.CXReceipt       // cross shard transaction receipts (source shard)
	incxs    []*types.CXReceiptsProof // cross shard receipts and its proof (desitinatin shard)
}

// Worker is the main object which takes care of submitting new work to consensus engine
// and gathering the sealing result.
type Worker struct {
	config  *params.ChainConfig
	chain   *core.BlockChain
	current *environment // An environment for current running cycle.

	engine consensus_engine.Engine

	gasFloor uint64
	gasCeil  uint64

	shardID uint32
}

// Returns a tuple where the first value is the txs sender account address,
// the second is the throttling result enum for the transaction of interest.
// Throttling happens based on the amount, frequency, etc.
func (w *Worker) throttleTxs(selected types.Transactions, recentTxsStats types.RecentTxsStats, txsThrottleConfig *shardingconfig.TxsThrottleConfig, tx *types.Transaction) (common.Address, shardingconfig.TxThrottleFlag) {
	var sender common.Address
	msg, err := tx.AsMessage(types.MakeSigner(w.config, w.chain.CurrentBlock().Number()))
	if err != nil {
		utils.Logger().Error().Err(err).Str("txId", tx.Hash().Hex()).Msg("Error when parsing tx into message")
	} else {
		sender = msg.From()
	}

	// already selected max num txs
	if len(selected) > txsThrottleConfig.MaxNumTxsPerBlockLimit {
		utils.Logger().Info().Str("txId", tx.Hash().Hex()).Int("MaxNumTxsPerBlockLimit", txsThrottleConfig.MaxNumTxsPerBlockLimit).Msg("Throttling tx with max num txs per block limit")
		return sender, shardingconfig.TxUnselect
	}

	// throttle a single sender sending too many transactions in one block
	if tx.Value().Cmp(txsThrottleConfig.MaxTxAmountLimit) > 0 {
		utils.Logger().Info().Str("txId", tx.Hash().Hex()).Uint64("MaxTxAmountLimit", txsThrottleConfig.MaxTxAmountLimit.Uint64()).Uint64("txAmount", tx.Value().Uint64()).Msg("Throttling tx with max amount limit")
		return sender, shardingconfig.TxInvalid
	}

	// throttle too large transaction
	var numTxsPastHour uint64
	for _, blockTxsCounts := range recentTxsStats {
		numTxsPastHour += blockTxsCounts[sender]
	}
	if numTxsPastHour >= txsThrottleConfig.MaxNumRecentTxsPerAccountLimit {
		utils.Logger().Info().Str("txId", tx.Hash().Hex()).Uint64("MaxNumRecentTxsPerAccountLimit", txsThrottleConfig.MaxNumRecentTxsPerAccountLimit).Msg("Throttling tx with max txs per account in a single block limit")
		return sender, shardingconfig.TxInvalid
	}

	return sender, shardingconfig.TxSelect
}

// SelectTransactionsForNewBlock selects transactions for new block.
func (w *Worker) SelectTransactionsForNewBlock(newBlockNum uint64, txs types.Transactions, recentTxsStats types.RecentTxsStats, txsThrottleConfig *shardingconfig.TxsThrottleConfig, coinbase common.Address) (types.Transactions, types.Transactions, types.Transactions) {
	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit)
	}

	selected := types.Transactions{}
	unselected := types.Transactions{}
	invalid := types.Transactions{}
	for _, tx := range txs {
		if tx.ShardID() != w.shardID && tx.ToShardID() != w.shardID {
			invalid = append(invalid, tx)
			continue
		}

		sender, flag := w.throttleTxs(selected, recentTxsStats, txsThrottleConfig, tx)
		switch flag {
		case shardingconfig.TxUnselect:
			unselected = append(unselected, tx)

		case shardingconfig.TxInvalid:
			invalid = append(invalid, tx)

		case shardingconfig.TxSelect:
			snap := w.current.state.Snapshot()
			_, err := w.commitTransaction(tx, coinbase)
			if err != nil {
				w.current.state.RevertToSnapshot(snap)
				invalid = append(invalid, tx)
				utils.Logger().Error().Err(err).Str("txId", tx.Hash().Hex()).Msg("Commit transaction error")
			} else {
				selected = append(selected, tx)
				// handle the case when msg was not able to extracted from tx
				if len(sender.String()) > 0 {
					recentTxsStats[newBlockNum][sender]++
				}
			}
		}

		// log invalid or unselected txs
		if flag == shardingconfig.TxUnselect || flag == shardingconfig.TxInvalid {
			utils.Logger().Info().Str("txId", tx.Hash().Hex()).Str("txThrottleFlag", flag.String()).Msg("Transaction Throttle flag")
		}

		utils.Logger().Info().Str("txId", tx.Hash().Hex()).Uint64("txGasLimit", tx.Gas()).Msg("Transaction gas limit info")
	}

	utils.Logger().Info().Uint64("newBlockNum", newBlockNum).Uint64("blockGasLimit", w.current.header.GasLimit).Uint64("blockGasUsed", w.current.header.GasUsed).Msg("Block gas limit and usage info")

	return selected, unselected, invalid
}

func (w *Worker) commitTransaction(tx *types.Transaction, coinbase common.Address) ([]*types.Log, error) {
	snap := w.current.state.Snapshot()

	receipt, cx, _, err := core.ApplyTransaction(w.config, w.chain, &coinbase, w.current.gasPool, w.current.state, w.current.header, tx, &w.current.header.GasUsed, vm.Config{})
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		return nil, err
	}
	if receipt == nil {
		utils.Logger().Warn().Interface("tx", tx).Interface("cx", cx).Msg("Receipt is Nil!")
		return nil, fmt.Errorf("Receipt is Nil")
	}
	w.current.txs = append(w.current.txs, tx)
	w.current.receipts = append(w.current.receipts, receipt)
	if cx != nil {
		w.current.outcxs = append(w.current.outcxs, cx)
	}

	return receipt.Logs, nil
}

// CommitTransactions commits transactions.
func (w *Worker) CommitTransactions(txs types.Transactions, coinbase common.Address) error {
	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit)
	}
	for _, tx := range txs {
		snap := w.current.state.Snapshot()
		_, err := w.commitTransaction(tx, coinbase)
		if err != nil {
			w.current.state.RevertToSnapshot(snap)
			return err

		}
	}
	return nil
}

// CommitReceipts commits a list of already verified incoming cross shard receipts
func (w *Worker) CommitReceipts(receiptsList []*types.CXReceiptsProof) error {
	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit)
	}

	if len(receiptsList) == 0 {
		w.current.header.IncomingReceiptHash = types.EmptyRootHash
	} else {
		w.current.header.IncomingReceiptHash = types.DeriveSha(types.CXReceiptsProofs(receiptsList))
	}

	for _, cx := range receiptsList {
		core.ApplyIncomingReceipt(w.config, w.current.state, w.current.header, cx)
	}

	for _, cx := range receiptsList {
		w.current.incxs = append(w.current.incxs, cx)
	}
	return nil
}

// UpdateCurrent updates the current environment with the current state and header.
func (w *Worker) UpdateCurrent(coinbase common.Address) error {
	parent := w.chain.CurrentBlock()
	num := parent.Number()
	timestamp := time.Now().Unix()
	// New block's epoch is the same as parent's...
	epoch := new(big.Int).Set(parent.Header().Epoch)

	// TODO: Don't depend on sharding state for epoch change.
	if len(parent.Header().ShardState) > 0 && parent.NumberU64() != 0 {
		// ... except if parent has a resharding assignment it increases by 1.
		epoch = epoch.Add(epoch, common.Big1)
	}
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent, w.gasFloor, w.gasCeil),
		Time:       big.NewInt(timestamp),
		Epoch:      epoch,
		ShardID:    w.chain.ShardID(),
		Coinbase:   coinbase,
	}
	return w.makeCurrent(parent, header)
}

// makeCurrent creates a new environment for the current cycle.
func (w *Worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := w.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	env := &environment{
		state:  state,
		header: header,
	}

	w.current = env
	return nil
}

// GetCurrentState gets the current state.
func (w *Worker) GetCurrentState() *state.DB {
	return w.current.state
}

// GetCurrentReceipts get the receipts generated starting from the last state.
func (w *Worker) GetCurrentReceipts() []*types.Receipt {
	return w.current.receipts
}

// OutgoingReceipts get the receipts generated starting from the last state.
func (w *Worker) OutgoingReceipts() []*types.CXReceipt {
	return w.current.outcxs
}

// IncomingReceipts get incoming receipts in destination shard that is received from source shard
func (w *Worker) IncomingReceipts() []*types.CXReceiptsProof {
	return w.current.incxs
}

// CommitWithCrossLinks generate a new block with cross links for the new txs.
func (w *Worker) CommitWithCrossLinks(sig []byte, signers []byte, viewID uint64, coinbase common.Address, crossLinks []byte) (*types.Block, error) {
	if len(sig) > 0 && len(signers) > 0 {
		copy(w.current.header.LastCommitSignature[:], sig[:])
		w.current.header.LastCommitBitmap = append(signers[:0:0], signers...)
	}
	w.current.header.Coinbase = coinbase
	w.current.header.ViewID = new(big.Int)
	w.current.header.ViewID.SetUint64(viewID)
	w.current.header.CrossLinks = crossLinks

	s := w.current.state.Copy()

	copyHeader := types.CopyHeader(w.current.header)
	block, err := w.engine.Finalize(w.chain, copyHeader, s, w.current.txs, w.current.receipts, w.current.outcxs, w.current.incxs)
	if err != nil {
		return nil, ctxerror.New("cannot finalize block").WithCause(err)
	}
	return block, nil
}

// Commit generate a new block for the new txs.
func (w *Worker) Commit(sig []byte, signers []byte, viewID uint64, coinbase common.Address) (*types.Block, error) {
	return w.CommitWithCrossLinks(sig, signers, viewID, coinbase, []byte{})
}

// New create a new worker object.
func New(config *params.ChainConfig, chain *core.BlockChain, engine consensus_engine.Engine, shardID uint32) *Worker {
	worker := &Worker{
		config: config,
		chain:  chain,
		engine: engine,
	}
	worker.gasFloor = 500000000000000000
	worker.gasCeil = 1000000000000000000
	worker.shardID = shardID

	parent := worker.chain.CurrentBlock()
	num := parent.Number()
	timestamp := time.Now().Unix()
	// New block's epoch is the same as parent's...
	epoch := new(big.Int).Set(parent.Header().Epoch)

	// TODO: Don't depend on sharding state for epoch change.
	if len(parent.Header().ShardState) > 0 && parent.NumberU64() != 0 {
		// ... except if parent has a resharding assignment it increases by 1.
		epoch = epoch.Add(epoch, common.Big1)
	}
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent, worker.gasFloor, worker.gasCeil),
		Time:       big.NewInt(timestamp),
		Epoch:      epoch,
		ShardID:    worker.chain.ShardID(),
	}
	worker.makeCurrent(parent, header)

	return worker
}
