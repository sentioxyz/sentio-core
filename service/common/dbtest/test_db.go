package dbtest

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/common/protos"
)

func PrepareTestPostgres(port int, dataPath string) (*embeddedpostgres.EmbeddedPostgres, error) {
	config := embeddedpostgres.DefaultConfig().
		Version(embeddedpostgres.V14).
		Port(uint32(port)).
		RuntimePath(dataPath). // different run should be the same
		//DataPath(dataPath). // datapath will be same as runtimepath if needed
		//BinariesPath(dataPath + "/postgres").
		Locale("en_US.UTF-8")

	database := embeddedpostgres.NewDatabase(config)
	return database, nil
}

type BasePGSuite struct {
	suite.Suite

	port     int
	dataPath string

	pg *embeddedpostgres.EmbeddedPostgres
}

func (s *BasePGSuite) SetupSuite() {
	var err error
	rand.Seed(time.Now().UnixNano())
	s.port = 20000 + rand.Intn(9999)
	s.dataPath, err = os.MkdirTemp("", "pg-unittest-*")
	require.NoError(s.T(), err, "MkdirTemp failed")
	s.pg, err = PrepareTestPostgres(s.port, s.dataPath)
	require.NoError(s.T(), err, "create pg instance failed, port: %d", s.port)
	err = s.pg.Start()
	require.NoError(s.T(), err, "start pg instance failed, port: %d", s.port)
}

func (s *BasePGSuite) TearDownSuite() {
	require.NoError(s.T(), s.pg.Stop(), "stop pg instance failed")
	require.NoError(s.T(), os.RemoveAll(s.dataPath), "remove data path %q failed", s.dataPath)
}

func (s *BasePGSuite) GetDBURL() string {
	return fmt.Sprintf("postgres://postgres:postgres@localhost:%d/postgres", s.port)
}

func (s *BasePGSuite) CreateProject(tx *gorm.DB, uid string, slug string) *models.Project {
	project := &models.Project{
		Slug:        slug,
		DisplayName: slug,
		OwnerID:     uid,
		OwnerType:   "users",
		Description: "test project " + slug,
	}

	require.NoError(s.T(), tx.Create(project).Error)
	return project
}

func (s *BasePGSuite) CreateUser(
	tx *gorm.DB,
	username string,
	sub string,
	tier protos.Tier,
) (*models.User, *models.Owner) {
	owner := &models.Owner{
		Name: username,
		Tier: protos.Tier_name[int32(tier)],
	}
	user := &models.User{
		ID:       gonanoid.Must(12),
		Email:    username + "@sentio.xyz",
		Username: username,
	}
	identity := &models.Identity{
		Sub:    sub,
		UserID: user.ID,
	}

	require.NoError(s.T(), tx.Create(owner).Error)
	require.NoError(s.T(), tx.Create(user).Error)
	require.NoError(s.T(), tx.Create(identity).Error)
	return user, owner
}
