package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/m0t0k1ch1/more-minimal-plasma-chain/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestNullBlock(t *testing.T) *Block {
	blk, err := NewBlock(nil, 0)
	require.NoError(t, err)
	return blk
}

func newTestBlock(t *testing.T, txes []*Tx, blkNum uint64) *Block {
	blk, err := NewBlock(txes, blkNum)
	require.NoError(t, err)
	return blk
}

func TestBlock_Hash(t *testing.T) {
	type output struct {
		blkHash common.Hash
		err     error
	}
	testCases := []struct {
		name string
		blk  *Block
		out  output
	}{
		{
			"null block",
			newTestNullBlock(t),
			output{
				utils.HexToHash("0x122d07b601c05953fe8229d17e5b5c0a66fbec3b9da839aea24afc18d86a6219"),
				nil,
			},
		},
		{
			"deposit block",
			newTestBlock(t, []*Tx{newTestDepositTx(t)}, 1),
			output{
				utils.HexToHash("0xde3e0e2864dfae133ca62399c2e194e813ede8ff7723e43c9c4660841d32b144"),
				nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blk, out := tc.blk, tc.out

			blkHash, err := blk.Hash()
			if out.err != nil {
				assert.EqualError(t, err, out.err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, out.blkHash, blkHash)
			}
		})
	}
}

func TestBlock_Root(t *testing.T) {
	type output struct {
		blkRootHash common.Hash
		err         error
	}
	testCases := []struct {
		name string
		blk  *Block
		out  output
	}{
		{
			"null block",
			newTestNullBlock(t),
			output{
				utils.HexToHash("0xe026cc5a4aed3c22a58cbd3d2ac754c9352c5436f638042dca99034e83636516"),
				nil,
			},
		},
		{
			"deposit block",
			newTestBlock(t, []*Tx{newTestDepositTx(t)}, 1),
			output{
				utils.HexToHash("0xf88f3819a6a679a60f8d5070af717bdfb41a87ab9eceb631136273928fb30560"),
				nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blk, out := tc.blk, tc.out

			blkRootHash, err := blk.Root()
			if out.err != nil {
				assert.EqualError(t, err, out.err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, out.blkRootHash, blkRootHash)
			}
		})
	}
}

func TestBlock_Sign(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	signer := NewAccount(privKey)
	blk := newTestNullBlock(t)

	// sign
	require.NoError(t, blk.Sign(signer))

	// verify
	signerAddr, err := blk.SignerAddress()
	require.NoError(t, err)
	assert.Equal(t, signer.Address(), signerAddr)
}
