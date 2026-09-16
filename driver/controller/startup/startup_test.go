package startup

import (
	"testing"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller/config"
	sentioerror "sentioxyz/sentio-core/service/common/errors"
	"sentioxyz/sentio-core/service/processor/models"

	"github.com/stretchr/testify/assert"
)

func Test_errorRecord(t *testing.T) {
	var er sentioerror.ErrorRecord
	pb := er.ToPB()
	log.Infof("pb.createdAt: %s", pb.GetCreatedAt().AsTime().String())
	var ner sentioerror.ErrorRecord
	ner.FromPB(pb)
	log.Infof("er.createdAt: %s", ner.CreatedAt.String())
	assert.True(t, ner.CreatedAt.IsZero())
}

func Test_buildProcessorUrls(t *testing.T) {
	var s standardStartupController

	s.config.ProcessorUrl = "aaa"
	s.processor = &models.Processor{NumWorkers: 1}
	urls, err := s.buildProcessorUrlList()
	assert.NoError(t, err)
	assert.Equal(t, []string{"aaa"}, urls)

	s.config.ProcessorUrl = "aaa"
	s.processor.NumWorkers = 3
	urls, err = s.buildProcessorUrlList()
	assert.NoError(t, err)
	assert.Equal(t, []string{"aaa", "aaa:81", "aaa:82"}, urls)

	s.config.ProcessorUrl = "aaa.bbb:9999"
	s.processor.NumWorkers = 3
	urls, err = s.buildProcessorUrlList()
	assert.NoError(t, err)
	assert.Equal(t, []string{"aaa.bbb:9999", "aaa.bbb:10000", "aaa.bbb:10001"}, urls)
}

func Test_splitSupportedChains(t *testing.T) {
	chainConfigs := map[string]*config.ChainConfig{"1": {}, "25": {}}

	supported, unsupported := splitSupportedChains([]string{"1", "388", "25"}, chainConfigs)
	assert.Equal(t, []string{"1", "25"}, supported)
	assert.Equal(t, []string{"388"}, unsupported)

	supported, unsupported = splitSupportedChains([]string{"388", "3776"}, chainConfigs)
	assert.Empty(t, supported)
	assert.Equal(t, []string{"388", "3776"}, unsupported)

	supported, unsupported = splitSupportedChains([]string{"1"}, chainConfigs)
	assert.Equal(t, []string{"1"}, supported)
	assert.Empty(t, unsupported)
}
