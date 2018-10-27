package models

import (
	"bytes"
	"crypto/ecdsa"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	merkle "github.com/m0t0k1ch1/fixed-merkle"
)

type BlockSummary struct {
	Txes      []string  `json:"txes"`
	Number    uint64    `json:"num"`
	Signature Signature `json:"sig"`
}

type Block struct {
	Txes       []*Tx
	Number     uint64
	Signature  Signature
	merkleTree *merkle.Tree
}

func NewBlock(txes []*Tx, blkNum uint64) (*Block, error) {
	blk := &Block{
		Txes:       txes,
		Number:     blkNum,
		Signature:  nullSignature,
		merkleTree: nil,
	}

	if err := blk.initMerkleTree(); err != nil {
		return nil, err
	}

	return blk, nil
}

func (blk *Block) initMerkleTree() error {
	leaves := make([][]byte, len(blk.Txes))
	for i, tx := range blk.Txes {
		leaf, err := tx.MerkleLeaf()
		if err != nil {
			return err
		}

		leaves[i] = leaf
	}

	tree, err := merkle.NewTree(MerkleConfig(), leaves)
	if err != nil {
		return err
	}

	blk.merkleTree = tree

	return nil
}

// implements RLP Encoder interface
// ref. https://godoc.org/github.com/ethereum/go-ethereum/rlp#Encoder
func (blk *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		blk.Txes, blk.Number,
	})
}

func (blk *Block) Hash() ([]byte, error) {
	b, err := rlp.EncodeToBytes(blk)
	if err != nil {
		return nil, err
	}

	return crypto.Keccak256(b), nil
}

func (blk *Block) Root() []byte {
	return blk.merkleTree.Root().Bytes()
}

func (blk *Block) Summary() (*BlockSummary, error) {
	summary := &BlockSummary{
		Txes:      make([]string, len(blk.Txes)),
		Number:    blk.Number,
		Signature: blk.Signature,
	}

	for i, tx := range blk.Txes {
		b, err := tx.Hash()
		if err != nil {
			return nil, err
		}

		summary.Txes[i] = hexutil.Encode(b)
	}

	return summary, nil
}

func (blk *Block) Sign(privKey *ecdsa.PrivateKey) error {
	hashBytes, err := blk.Hash()
	if err != nil {
		return err
	}

	sigBytes, err := crypto.Sign(hashBytes, privKey)
	if err != nil {
		return err
	}
	blk.Signature = newSignatureFromBytes(sigBytes)

	return nil
}

func (blk *Block) Signer() (common.Address, error) {
	hashBytes, err := blk.Hash()
	if err != nil {
		return common.Address{}, err
	}

	if bytes.Equal(blk.Signature.Bytes(), nullSignature.Bytes()) {
		return nullAddress, nil
	}

	pubKey, err := crypto.SigToPub(hashBytes, blk.Signature.Bytes())
	if err != nil {
		return common.Address{}, err
	}

	return crypto.PubkeyToAddress(*pubKey), nil
}
