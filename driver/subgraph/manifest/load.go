package manifest

import (
	"errors"
	"fmt"
	"io"

	shell "github.com/ipfs/go-ipfs-api"
	"gopkg.in/yaml.v2"
)

func load(r io.Reader) (*Manifest, error) {
	var mf Manifest
	return &mf, yaml.NewDecoder(r).Decode(&mf)
}

var ErrInvalidManifest = errors.New("invalid manifest")

func LoadFromIpfs(ipfsShell *shell.Shell, hash string, verify bool) (*Manifest, error) {
	r, err := ipfsShell.Cat(hash)
	if err != nil {
		return nil, fmt.Errorf("cat subgraph manifest file with ipfs hash %q failed: %w", hash, err)
	}
	defer r.Close()
	mf, err := load(r)
	if err != nil {
		return nil, fmt.Errorf("%w: load subgraph manifest with ipfs hash %q failed: %v", ErrInvalidManifest, hash, err)
	}
	if verify {
		if err = mf.verify(); err != nil {
			if errors.Is(err, ErrInvalidCustomizedEndpoint) {
				return nil, err
			}
			return nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
		}
	}
	if err = mf.loadIpfsFiles(ipfsShell); err != nil {
		return nil, err
	}
	if err = mf.loadContractABI(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	if err = mf.loadEventHandlerTopic0(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	if err = mf.loadCallHandlerInfo(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	return mf, nil
}
