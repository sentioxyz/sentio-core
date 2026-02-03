package state

import (
	"context"
	"os"

	"gopkg.in/yaml.v3"
)

type FileStore struct {
	filename string
}

func NewFileStore(filename string) *FileStore {
	return &FileStore{filename: filename}
}

func (s *FileStore) Load(ctx context.Context) (*PlainState, error) {
	state := PlainState{
		ProcessorAllocations: map[string]map[uint64]ProcessorAllocation{},
		ProcessorInfos:       map[string]ProcessorInfo{},
		IndexerInfos:         map[uint64]IndexerInfo{},
		HostedProcessors:     map[string]bool{},
	}
	data, err := os.ReadFile(s.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &state, nil
		}
		return nil, err
	}
	err = yaml.Unmarshal(data, &state)
	if err != nil {
		return nil, err
	}
	if state.ProcessorAllocations == nil {
		state.ProcessorAllocations = map[string]map[uint64]ProcessorAllocation{}
	}
	if state.ProcessorInfos == nil {
		state.ProcessorInfos = map[string]ProcessorInfo{}
	}
	if state.IndexerInfos == nil {
		state.IndexerInfos = map[uint64]IndexerInfo{}
	}
	return &state, nil
}

func (s *FileStore) Save(ctx context.Context, state State) error {
	plainState := &PlainState{
		LastBlock:            state.GetLastBlock(),
		ProcessorAllocations: state.GetProcessorAllocations(),
		ProcessorInfos:       state.GetProcessorInfos(),
		IndexerInfos:         state.GetIndexerInfos(),
		HostedProcessors:     state.GetHostedProcessors(),
	}
	data, err := yaml.Marshal(plainState)
	if err != nil {
		return err
	}
	return os.WriteFile(s.filename, data, 0644)
}

func (s *FileStore) Close() error {
	return nil
}
