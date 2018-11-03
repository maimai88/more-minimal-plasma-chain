package app

import (
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/m0t0k1ch1/more-minimal-plasma-chain/core"
	"github.com/m0t0k1ch1/more-minimal-plasma-chain/utils"
)

func (cc *ChildChain) PostBlockHandler(c *Context) error {
	c.Request().ParseForm()

	blkNum, err := cc.rootChain.CurrentPlasmaBlockNumber()
	if err != nil {
		return c.JSONError(err)
	}
	if blkNum.Uint64() != cc.blockchain.CurrentBlockNumber() {
		return c.JSONError(ErrRootChainNotSynchronized)
	}

	blkHash, err := cc.blockchain.AddBlock(cc.operator)
	if err != nil {
		if err == core.ErrEmptyBlock {
			return c.JSONError(ErrEmptyBlock)
		}
		return c.JSONError(err)
	}

	blk, err := cc.blockchain.GetBlock(blkHash)
	if err != nil {
		if err == core.ErrBlockNotFound {
			return c.JSONError(ErrBlockNotFound)
		}
		return c.JSONError(err)
	}

	rootHash, err := blk.Root()
	if err != nil {
		return c.JSONError(err)
	}

	if _, err := cc.rootChain.CommitPlasmaBlockRoot(cc.operator, rootHash); err != nil {
		return c.JSONError(err)
	}
	cc.Logger().Infof("[COMMIT] root: %s", utils.EncodeToHex(rootHash[:]))

	return c.JSONSuccess(map[string]interface{}{
		"blkhash": utils.EncodeToHex(blkHash.Bytes()),
	})
}

func (cc *ChildChain) GetBlockHandler(c *Context) error {
	blkHash, err := c.GetBlockHashFromPath()
	if err != nil {
		return c.JSONError(err)
	}

	blk, err := cc.blockchain.GetBlock(blkHash)
	if err != nil {
		if err == core.ErrBlockNotFound {
			return c.JSONError(ErrBlockNotFound)
		}
		return c.JSONError(err)
	}

	blkBytes, err := rlp.EncodeToBytes(blk)
	if err != nil {
		return c.JSONError(err)
	}

	return c.JSONSuccess(map[string]interface{}{
		"blk": utils.EncodeToHex(blkBytes),
	})
}
