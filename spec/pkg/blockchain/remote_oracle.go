package blockchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type RemoteOracle struct {
	url string
}

func NewRemoteOracle(url string) *RemoteOracle {
	return &RemoteOracle{url: url}
}

// RegisterDidDoc 现在真正地通过 HTTP POST 注册 DID 文档到远程 Oracle
func (r *RemoteOracle) RegisterDidDoc(did string, doc []byte) {
	payload := struct {
		Did      string `json:"did"`
		Document []byte `json:"document"`
	}{
		Did:      did,
		Document: doc,
	}

	data, _ := json.Marshal(payload)
	resp, err := http.Post(fmt.Sprintf("%s/register", r.url), "application/json", bytes.NewBuffer(data))
	if err != nil {
		fmt.Printf("[RemoteOracle] Failed to register %s: %v\n", did, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("[RemoteOracle] Successfully registered/updated identity: %s\n", did)
	} else {
		fmt.Printf("[RemoteOracle] Registration for %s returned status: %s\n", did, resp.Status)
	}
}

func (r *RemoteOracle) ResolveDidDoc(did string) ([]byte, error) {
	resp, err := http.Get(fmt.Sprintf("%s/resolve?did=%s", r.url, did))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oracle returned status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var resolveResp struct {
		Document []byte `json:"document"`
		Error    string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &resolveResp); err != nil {
		return nil, err
	}

	if resolveResp.Error != "" {
		return nil, errors.New(resolveResp.Error)
	}

	return resolveResp.Document, nil
}
