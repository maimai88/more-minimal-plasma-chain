package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/m0t0k1ch1/more-minimal-plasma-chain/core"
	"github.com/m0t0k1ch1/more-minimal-plasma-chain/core/types"
	"github.com/m0t0k1ch1/more-minimal-plasma-chain/utils"
)

type HandlerFunc func(*Context) error

type Plasma struct {
	config     *Config
	server     *echo.Echo
	operator   *types.Account
	rootChain  *core.RootChain
	childChain *core.ChildChain
}

func NewPlasma(conf *Config) (*Plasma, error) {
	p := &Plasma{
		config: conf,
	}

	p.initServer()
	if err := p.initRootChain(); err != nil {
		return nil, err
	}
	if err := p.initOperator(); err != nil {
		return nil, err
	}
	p.initChildChain()

	return p, nil
}

func (p *Plasma) initServer() {
	p.server = echo.New()
	p.server.Use(middleware.Logger())
	p.server.Use(middleware.Recover())
	p.server.Use(func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return h(&Context{c})
		}
	})
	p.server.HTTPErrorHandler = p.httpErrorHandler
	p.server.Logger.SetLevel(log.INFO)
	p.initRoutes()
}

func (p *Plasma) initRoutes() {
	p.GET("/ping", p.PingHandler)
	p.GET("/chain/:blkNum", p.GetChainHandler)
	p.POST("/blocks", p.PostBlockHandler)
	p.GET("/blocks/:blkHash", p.GetBlockHandler)
	p.POST("/txes", p.PostTxHandler)
	p.GET("/txes/:txHash", p.GetTxHandler)
	p.GET("/txes/:txHash/index", p.GetTxIndexHandler)
	p.GET("/txes/:txHash/proof", p.GetTxProofHandler)
	p.PUT("/txes/:txHash", p.PutTxHandler)
}

func (p *Plasma) initRootChain() error {
	rc, err := core.NewRootChain(p.config.RootChain)
	if err != nil {
		return err
	}
	p.rootChain = rc
	return nil
}

func (p *Plasma) initOperator() error {
	privKey, err := crypto.HexToECDSA(p.config.Operator.PrivateKey)
	if err != nil {
		return err
	}
	p.operator = types.NewAccount(privKey)
	return nil
}

func (p *Plasma) initChildChain() error {
	cc, err := core.NewChildChain()
	if err != nil {
		return err
	}
	p.childChain = cc
	return nil
}

func (p *Plasma) GET(path string, h HandlerFunc, m ...echo.MiddlewareFunc) {
	p.Add(http.MethodGet, path, h, m...)
}

func (p *Plasma) POST(path string, h HandlerFunc, m ...echo.MiddlewareFunc) {
	p.Add(http.MethodPost, path, h, m...)
}

func (p *Plasma) PUT(path string, h HandlerFunc, m ...echo.MiddlewareFunc) {
	p.Add(http.MethodPut, path, h, m...)
}

func (p *Plasma) Add(method, path string, h HandlerFunc, m ...echo.MiddlewareFunc) {
	p.server.Add(method, path, func(c echo.Context) error {
		return h(NewContext(c))
	})
}

func (p *Plasma) Logger() echo.Logger {
	return p.server.Logger
}

func (p *Plasma) Start() error {
	if err := p.watchRootChain(); err != nil {
		return err
	}

	return p.server.Start(fmt.Sprintf(":%d", p.config.Port))
}

func (p *Plasma) watchRootChain() error {
	sink := make(chan *core.RootChainDepositCreated)
	sub, err := p.rootChain.WatchDepositCreated(context.Background(), sink)
	if err != nil {
		return err
	}

	go func() {
		defer sub.Unsubscribe()
		for log := range sink {
			blkHash, err := p.childChain.AddDepositBlock(log.Owner, log.Amount, p.operator)
			if err != nil {
				p.Logger().Error(err)
			} else {
				p.Logger().Infof(
					"[DEPOSIT] blkhash: %s, owner: %s: amount: %s",
					utils.HashToHex(blkHash),
					utils.AddressToHex(log.Owner),
					log.Amount.String(),
				)
			}
		}
	}()

	return nil
}

func (p *Plasma) httpErrorHandler(err error, c echo.Context) {
	p.Logger().Error(err)

	code := http.StatusInternalServerError
	msg := http.StatusText(code)

	if httpErr, ok := err.(*echo.HTTPError); ok {
		code = httpErr.Code
		msg = fmt.Sprintf("%v", httpErr.Message)
	}

	appErr := NewError(code, msg)

	if err := c.JSON(appErr.Code, NewErrorResponse(appErr)); err != nil {
		p.Logger().Error(err)
	}
}
