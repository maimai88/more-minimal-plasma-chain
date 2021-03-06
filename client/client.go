package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/m0t0k1ch1/more-minimal-plasma-chain/app"
)

type Client struct {
	*http.Client
	baseURI string
}

func New(baseURI string) *Client {
	return &Client{
		Client:  http.DefaultClient,
		baseURI: baseURI,
	}
}

func (c *Client) doAPI(ctx context.Context, method, uri string, params url.Values, res interface{}) error {
	u, err := url.Parse(c.baseURI)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, uri)

	var body io.Reader
	switch method {
	case http.MethodGet:
		u.RawQuery = params.Encode()
	default:
		body = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return err
	}
	req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var resErr ErrorResponse
	if err := json.Unmarshal(b, &resErr); err != nil {
		return err
	}
	if resErr.State == app.ResponseStateError {
		return fmt.Errorf(
			"API error : %s [%d]",
			resErr.Result.Message, resErr.Result.Code,
		)
	}

	return json.Unmarshal(b, &res)
}
