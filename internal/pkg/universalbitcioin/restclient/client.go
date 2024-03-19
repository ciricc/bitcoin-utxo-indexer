package restclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
)

type RESTClient struct {
	nodeRestURL *url.URL
	c           *http.Client
}

func New(nodeRestURL *url.URL) (*RESTClient, error) {
	if nodeRestURL == nil {
		return nil, ErrNodeHostNotSpecified
	}

	return &RESTClient{
		nodeRestURL: nodeRestURL,
		c:           http.DefaultClient,
	}, nil
}

func (r *RESTClient) GetBlockHeader(
	ctx context.Context,
	hash Hash,
) (*blockchain.BlockHeader, error) {
	var header blockchain.BlockHeader

	res, err := r.callRest(
		ctx,
		buildJsonMethodPath("block", "notxdetails", hash.String()),
		&header,
	)
	if err != nil && res == nil {
		return nil, fmt.Errorf("failed to get block header: %w", err)
	}

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrBlockNotFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, newBadStatusCodeError(res.StatusCode)
	}

	return &header, nil
}

func (r *RESTClient) GetBlock(
	ctx context.Context,
	hash Hash,
) (*blockchain.Block, error) {
	var block blockchain.Block

	res, err := r.callRest(ctx, buildJsonMethodPath("block", hash.String()), &block)
	if err != nil && res == nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrBlockNotFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, newBadStatusCodeError(res.StatusCode)
	}

	return &block, nil
}

func (r *RESTClient) callRest(
	ctx context.Context,
	method string,
	unmarshalTo interface{},
) (*http.Response, error) {
	path, err := url.JoinPath("/", strings.TrimPrefix(r.nodeRestURL.Path, "/"), method)
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	url := fmt.Sprintf("%s://%s%s", r.nodeRestURL.Scheme, r.nodeRestURL.Host, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	res, err := r.c.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.Body != nil && res.Header.Get("content-type") == "application/json" {
		resBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		err = json.Unmarshal(resBytes, unmarshalTo)
		if err != nil {
			return res, fmt.Errorf("failed to unmarshal json response body: %w", err)
		}

		return res, nil
	}

	return res, ErrUnknownResponseType
}

func buildJsonMethodPath(elem ...string) string {
	return fmt.Sprintf("%s.json", path.Join(elem...))
}
