package startup

import (
	"testing"

	"sentioxyz/sentio-core/common/log"
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
