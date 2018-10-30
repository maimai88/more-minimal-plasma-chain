package app

func (cc *ChildChain) GetBlockHandler(c *Context) error {
	blkNum, err := c.GetBlockNumberFromPath()
	if err != nil {
		return c.JSONError(err)
	}

	return cc.getBlockHandler(c, blkNum)
}

func (cc *ChildChain) PostBlockHandler(c *Context) error {
	c.Request().ParseForm()

	bt, err := c.GetBlockTypeFromForm()
	if err != nil {
		return c.JSONError(err)
	}

	if bt.IsDeposit() {
		return cc.postDepositBlockHandler(c)
	}
	return cc.postBlockHandler(c)
}

func (cc *ChildChain) postBlockHandler(c *Context) error {
	blkNum, err := cc.blockchain.AddBlock(cc.operator)
	if err != nil {
		return c.JSONError(err)
	}

	return cc.getBlockHandler(c, blkNum)
}

func (cc *ChildChain) postDepositBlockHandler(c *Context) error {
	ownerAddr, err := c.GetOwnerFromForm()
	if err != nil {
		return c.JSONError(err)
	}
	amount, err := c.GetAmountFromForm()
	if err != nil {
		return c.JSONError(err)
	}

	blkNum, err := cc.blockchain.AddDepositBlock(ownerAddr, amount, cc.operator)
	if err != nil {
		return c.JSONError(err)
	}

	return cc.getBlockHandler(c, blkNum)
}

func (cc *ChildChain) getBlockHandler(c *Context, blkNum uint64) error {
	blk := cc.blockchain.GetBlock(blkNum)
	if blk == nil {
		return c.JSONError(ErrBlockNotFound)
	}

	blkSummary, err := blk.Summary()
	if err != nil {
		return c.JSONError(err)
	}

	return c.JSONSuccess(blkSummary)
}
