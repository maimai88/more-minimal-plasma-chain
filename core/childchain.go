package core

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/m0t0k1ch1/more-minimal-plasma-chain/core/types"
	"github.com/m0t0k1ch1/more-minimal-plasma-chain/utils"
)

const (
	DefaultBlockNumber = 1

	CurrentBlockNumberKey = "current_blknum"
	TxMempoolKeyPrefix    = "tx_mempool_"
)

type ChildChain struct {
	mu           *sync.RWMutex
	currentBlock *types.Block
	chain        map[string]*types.Block // key: blkNum
}

func NewChildChain(txn *badger.Txn) (*ChildChain, error) {
	cc := &ChildChain{
		mu:    &sync.RWMutex{},
		chain: map[string]*types.Block{},
	}

	currentBlkNum, err := cc.getCurrentBlockNumber(txn)
	if err == badger.ErrKeyNotFound {
		if err := cc.setCurrentBlockNumber(txn, big.NewInt(DefaultBlockNumber)); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	blk, err := types.NewBlock(nil, currentBlkNum)
	if err != nil {
		return nil, err
	}
	cc.currentBlock = blk

	return cc, nil
}

func (cc *ChildChain) GetCurrentBlockNumber(txn *badger.Txn) (*big.Int, error) {
	return cc.getCurrentBlockNumber(txn)
}

func (cc *ChildChain) GetBlock(blkNum *big.Int) (*types.Block, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if !cc.isExistBlock(blkNum) {
		return nil, ErrBlockNotFound
	}

	return cc.getBlock(blkNum), nil
}

func (cc *ChildChain) AddBlock(txn *badger.Txn, signer *types.Account) (*big.Int, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	blk := cc.currentBlock

	// check block validity
	if len(blk.Txes) == 0 {
		return nil, ErrEmptyBlock
	}

	// sign block
	if err := blk.Sign(signer); err != nil {
		return nil, err
	}

	// add block
	cc.addBlock(blk)

	// increment current block number
	nextBlkNum, err := cc.incrementCurrentBlockNumber(txn)
	if err != nil {
		return nil, err
	}

	// reset current block
	blkNext, err := types.NewBlock(nil, nextBlkNum)
	if err != nil {
		return nil, err
	}
	cc.currentBlock = blkNext

	return blk.Number, nil
}

func (cc *ChildChain) AddDepositBlock(txn *badger.Txn, ownerAddr common.Address, amount *big.Int, signer *types.Account) (*big.Int, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// create deposit tx
	tx := types.NewTx()
	txOut := types.NewTxOut(ownerAddr, amount)
	if err := tx.SetOutput(big.NewInt(0), txOut); err != nil {
		return nil, err
	}

	// get current block number
	currentBlkNum, err := cc.getCurrentBlockNumber(txn)
	if err != nil {
		return nil, err
	}

	// create deposit block
	blk, err := types.NewBlock([]*types.Tx{tx}, currentBlkNum)
	if err != nil {
		return nil, err
	}

	// sign deposit block
	if err := blk.Sign(signer); err != nil {
		return nil, err
	}

	// add deposit block
	cc.addBlock(blk)

	// increment current block number
	nextBlkNum, err := cc.incrementCurrentBlockNumber(txn)
	if err != nil {
		return nil, err
	}
	cc.currentBlock.Number = nextBlkNum

	return blk.Number, nil
}

func (cc *ChildChain) GetTx(txPos *types.Position) (*types.Tx, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	blkNum, txIndex := types.ParseTxPosition(txPos)

	if !cc.isExistTx(blkNum, txIndex) {
		return nil, ErrTxNotFound
	}

	return cc.getTx(blkNum, txIndex), nil
}

func (cc *ChildChain) GetTxProof(txPos *types.Position) ([]byte, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	blkNum, txIndex := types.ParseTxPosition(txPos)

	blk := cc.getBlock(blkNum)

	// build tx Merkle tree
	tree, err := blk.MerkleTree()
	if err != nil {
		return nil, err
	}

	// create tx proof
	return tree.CreateMembershipProof(txIndex.Uint64())
}

func (cc *ChildChain) AddTxToMempool(txn *badger.Txn, tx *types.Tx) (*types.Position, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// validate tx
	if err := cc.validateTx(tx); err != nil {
		return types.NullPosition, err
	}

	// spend utxo
	for _, txIn := range tx.Inputs {
		if txIn.IsNull() {
			continue
		}
		if err := cc.spendUTXO(txIn.BlockNumber, txIn.TxIndex, txIn.OutputIndex); err != nil {
			return types.NullPosition, err
		}
	}

	// add tx to mempool
	if err := cc.addTxToMempool(txn, tx); err != nil {
		return types.NullPosition, err
	}

	return types.NewTxPosition(cc.currentBlock.Number, cc.currentBlock.LastTxIndex()), nil
}

func (cc *ChildChain) ConfirmTx(txInPos *types.Position, confSig types.Signature) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	blkNum, txIndex, inIndex := types.ParseTxInPosition(txInPos)

	// check tx existence
	if !cc.isExistTx(blkNum, txIndex) {
		return ErrTxNotFound
	}

	tx := cc.getTx(blkNum, txIndex)

	// check txin existence
	if !tx.IsExistInput(inIndex) {
		return ErrTxInNotFound
	}

	txIn := tx.GetInput(inIndex)

	// check txin validity
	if txIn.IsNull() {
		return ErrNullTxInConfirmation
	}

	inTxOut := cc.getTxOut(txIn.BlockNumber, txIn.TxIndex, txIn.OutputIndex)

	// verify confirmation signature
	h, err := tx.ConfirmationHash()
	if err != nil {
		return err
	}
	signerAddr, err := confSig.SignerAddress(h)
	if err != nil {
		return ErrInvalidTxConfirmationSignature
	}
	if !bytes.Equal(signerAddr.Bytes(), inTxOut.OwnerAddress.Bytes()) {
		return ErrInvalidTxConfirmationSignature
	}

	// update confirmation signature
	if err := cc.setConfirmationSignature(blkNum, txIndex, inIndex, confSig); err != nil {
		return err
	}

	return nil
}

func (cc *ChildChain) getCurrentBlockNumber(txn *badger.Txn) (*big.Int, error) {
	item, err := txn.Get([]byte(CurrentBlockNumberKey))
	if err != nil {
		return nil, err
	}

	val, err := item.Value()
	if err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(val), nil
}

func (cc *ChildChain) setCurrentBlockNumber(txn *badger.Txn, blkNum *big.Int) error {
	return txn.Set([]byte(CurrentBlockNumberKey), blkNum.Bytes())
}

func (cc *ChildChain) getNextBlockNumber(txn *badger.Txn) (*big.Int, error) {
	currentBlkNum, err := cc.getCurrentBlockNumber(txn)
	if err != nil {
		return nil, err
	}

	return currentBlkNum.Add(currentBlkNum, big.NewInt(1)), nil
}

func (cc *ChildChain) incrementCurrentBlockNumber(txn *badger.Txn) (*big.Int, error) {
	nextBlkNum, err := cc.getNextBlockNumber(txn)
	if err != nil {
		return nil, err
	}

	if err := cc.setCurrentBlockNumber(txn, nextBlkNum); err != nil {
		return nil, err
	}

	return nextBlkNum, nil
}

func (cc *ChildChain) getBlock(blkNum *big.Int) *types.Block {
	return cc.chain[blkNum.String()]
}

func (cc *ChildChain) isExistBlock(blkNum *big.Int) bool {
	_, ok := cc.chain[blkNum.String()]
	return ok
}

func (cc *ChildChain) addBlock(blk *types.Block) {
	cc.chain[blk.Number.String()] = blk
}

func (cc *ChildChain) getTx(blkNum, txIndex *big.Int) *types.Tx {
	return cc.getBlock(blkNum).GetTx(txIndex)
}

func (cc *ChildChain) isExistTx(blkNum, txIndex *big.Int) bool {
	if !cc.isExistBlock(blkNum) {
		return false
	}

	return cc.getBlock(blkNum).IsExistTx(txIndex)
}

func (cc *ChildChain) validateTx(tx *types.Tx) error {
	nullTxInNum := 0
	iAmount, oAmount := big.NewInt(0), big.NewInt(0)

	for _, txOut := range tx.Outputs {
		oAmount.Add(oAmount, txOut.Amount)
	}

	for i, txIn := range tx.Inputs {
		// check spending txout existence
		if !cc.isExistTxOut(txIn.BlockNumber, txIn.TxIndex, txIn.OutputIndex) {
			if txIn.IsNull() {
				nullTxInNum++
				continue
			}
			return ErrInvalidTxIn
		}

		inTxOut := cc.getTxOut(txIn.BlockNumber, txIn.TxIndex, txIn.OutputIndex)

		// check double spent
		if inTxOut.IsSpent {
			return ErrTxOutAlreadySpent
		}

		// verify signature
		signerAddr, err := tx.SignerAddress(big.NewInt(int64(i)))
		if err != nil {
			return ErrInvalidTxSignature
		}
		if txIn.Signature == types.NullSignature ||
			!bytes.Equal(signerAddr.Bytes(), inTxOut.OwnerAddress.Bytes()) {
			return ErrInvalidTxSignature
		}

		iAmount.Add(iAmount, inTxOut.Amount)
	}

	// check txins validity
	if nullTxInNum == len(tx.Inputs) {
		return ErrInvalidTxIn
	}

	// check in/out balance
	if iAmount.Cmp(oAmount) < 0 {
		return ErrInvalidTxBalance
	}

	return nil
}

func (cc *ChildChain) txMempoolKey(tx *types.Tx) ([]byte, error) {
	txHash, err := tx.Hash()
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(TxMempoolKeyPrefix + utils.HashToHex(txHash))), nil
}

func (cc *ChildChain) addTxToMempool(txn *badger.Txn, tx *types.Tx) error {
	// TODO: delete
	// add tx to current block
	if err := cc.currentBlock.AddTx(tx); err != nil {
		return err
	}

	key, err := cc.txMempoolKey(tx)
	if err != nil {
		return err
	}

	txBytes, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}

	return txn.Set(key, txBytes)
}

func (cc *ChildChain) getTxOut(blkNum, txIndex, outIndex *big.Int) *types.TxOut {
	return cc.getTx(blkNum, txIndex).GetOutput(outIndex)
}

func (cc *ChildChain) isExistTxOut(blkNum, txIndex, outIndex *big.Int) bool {
	if !cc.isExistTx(blkNum, txIndex) {
		return false
	}

	return cc.getTx(blkNum, txIndex).IsExistOutput(outIndex)
}

func (cc *ChildChain) spendUTXO(blkNum, txIndex, outIndex *big.Int) error {
	return cc.getTx(blkNum, txIndex).SpendOutput(outIndex)
}

func (cc *ChildChain) setConfirmationSignature(blkNum, txIndex, inIndex *big.Int, confSig types.Signature) error {
	return cc.getTx(blkNum, txIndex).SetConfirmationSignature(inIndex, confSig)
}
