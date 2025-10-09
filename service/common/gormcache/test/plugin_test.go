package test

//
//import (
//	"fmt"
//	"math/rand"
//	"os"
//	"sentioxyz/sentio-core/common/gonanoid"
//	"sentioxyz/sentio-core/common/log"
//	"sentioxyz/sentio-core/service/common/dbtest"
//	"sentioxyz/sentio-core/service/common/gormcache"
//	"sentioxyz/sentio-core/service/common/models"
//	"sentioxyz/sentio-core/service/common/protos"
//	commonrepo "sentioxyz/sentio-core/service/common/repository"
//	"testing"
//	"time"
//
//	"github.com/pkg/errors"
//
//	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//	"github.com/stretchr/testify/suite"
//	"gorm.io/gorm"
//)

//
//type GormCachePluginTestSuite struct {
//	suite.Suite
//	database *embeddedpostgres.EmbeddedPostgres
//	conn     *gorm.DB
//	dataPath string
//	cache    gormcache.CacheDB
//}
//
//func (s *GormCachePluginTestSuite) SetupSuite() {
//	var err error
//	require.NoError(s.T(), err)
//
//	testdir := os.Getenv("TEST_TMPDIR")
//	if testdir == "" {
//		testdir = os.TempDir()
//	}
//	dataPath := testdir + "pgdata"
//
//	db, err := s.SetupTestDB(dataPath)
//	require.NoError(s.T(), err)
//	s.dataPath = dataPath
//	s.conn = db.Begin()
//}
//
//func (s *GormCachePluginTestSuite) SetupTestDB(dataPath string) (*gorm.DB, error) {
//	rand.Seed(time.Now().UnixNano())
//	port := 20000 + rand.Intn(9999)
//
//	database, err := dbtest.PrepareTestPostgres(port, dataPath)
//	if err != nil {
//		return nil, err
//	}
//	err = database.Start()
//	if err != nil {
//		return nil, err
//	}
//	dbURL := fmt.Sprintf("postgres://postgres:postgres@localhost:%d/postgres", port)
//
//	conn, err := commonrepo.SetupDBWithoutCache(dbURL)
//	if err != nil {
//		return nil, err
//	}
//	s.cache = gormcache.NewLocalCacheDB(10 * time.Minute)
//	s.database = database
//	err = conn.Use(gormcache.NewGormCachePlugin(s.cache))
//	return conn, err
//}
//
//func (s *GormCachePluginTestSuite) SetupTest() {
//	s.conn.Begin()
//	s.conn.SavePoint("before_test")
//}
//
//func (s *GormCachePluginTestSuite) TearDownTest() {
//	s.conn.RollbackTo("before_test")
//}
//
//func (s *GormCachePluginTestSuite) TearDownSuite() {
//	err := s.database.Stop()
//	if err != nil {
//		log.Errore(err)
//	}
//	require.NoError(s.T(), err)
//	os.RemoveAll(s.dataPath)
//	log.Sync()
//}
//
//func TestGormCachePluginTestSuite(t *testing.T) {
//	suite.Run(t, new(GormCachePluginTestSuite))
//}
//
//func (s *GormCachePluginTestSuite) newUser(db *gorm.DB, username string) (*models.User, error) {
//	user := models.User{
//		ID:            "user-" + gonanoid.Must(8),
//		Email:         username + "@email.com",
//		EmailVerified: true,
//		FirstName:     username,
//		LastName:      username,
//		Username:      username,
//		AccountStatus: "ACTIVE",
//	}
//	owner := models.Owner{
//		Name: username,
//	}
//	err := s.conn.Save(&owner).Error
//	if err != nil {
//		return &user, err
//	}
//	err = s.conn.Create(&user).Error
//	if err != nil {
//		return nil, err
//	}
//	identity := models.Identity{
//		Sub:    "sub-" + user.ID,
//		UserID: user.ID,
//	}
//	err = db.Create(&identity).Error
//	return &user, err
//}
//
//func (s *GormCachePluginTestSuite) newOrg(name string) (*models.Organization, error) {
//	org := models.Organization{
//		ID:   "org-" + gonanoid.Must(8),
//		Name: name,
//	}
//	//owner := models.Owner{
//	//	Name: name,
//	//}
//	//err := s.conn.Save(&owner).Error
//	//if err != nil {
//	//	return nil, err
//	//}
//	err := s.conn.Create(&org).Error
//	return &org, err
//}
//
//func (s *GormCachePluginTestSuite) newProject(slug, owner string) (*models.Project, error) {
//	project := models.Project{
//		ID:          "project-" + gonanoid.Must(8),
//		Slug:        slug,
//		DisplayName: slug + " project",
//		Description: slug + " description",
//		OwnerID:     owner,
//		OwnerType:   "users",
//		OwnerName:   "tester",
//		Public:      false,
//	}
//	err := s.conn.Create(&project).Error
//	return &project, err
//}
//
//func (s *GormCachePluginTestSuite) Test_CRUD() {
//	s.cache.ResetCache()
//	user, err := s.newUser(s.conn, "tester")
//	assert.NoError(s.T(), err)
//
//	var u models.User
//	err = s.conn.Preload("Projects").First(&u, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(u.Projects))
//
//	project, err := s.newProject("test", user.ID)
//	assert.NoError(s.T(), err)
//	var u2 models.User
//	err = s.conn.Preload("Projects").First(&u2, "id = ?", user.ID).Error
//
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), project.ID, u2.Projects[0].ID)
//
//	err = s.conn.Delete(&project).Error
//	assert.NoError(s.T(), err)
//	var u3 models.User
//	err = s.conn.Preload("Projects").First(&u3, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(u3.Projects))
//}
//
//func (s *GormCachePluginTestSuite) Test_Query() {
//	s.cache.ResetCache()
//	user, err := s.newUser(s.conn, "tester")
//	assert.NoError(s.T(), err)
//
//	project, err := s.newProject("test", user.ID)
//	assert.NoError(s.T(), err)
//
//	var u models.User
//	err = s.conn.Preload("Projects").First(&u, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(u.Projects))
//	assert.Equal(s.T(), project.ID, u.Projects[0].ID)
//
//	// load once more to make sure it's cached
//	err = s.conn.Preload("Projects").First(&u, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//
//	hit, miss := s.cache.GetCacheCount()
//	runs := 10
//	for i := 0; i < runs; i++ {
//		err = s.conn.Preload("Projects").First(&u, "id = ?", user.ID).Error
//		assert.NoError(s.T(), err)
//	}
//	hit2, miss2 := s.cache.GetCacheCount()
//	assert.GreaterOrEqualf(s.T(), int(hit2-hit), runs, "should hit cache")
//	assert.Equal(s.T(), miss, miss2)
//}
//
//func (s *GormCachePluginTestSuite) Test_Query_Expiration() {
//	s.cache.ResetCache()
//	user, err := s.newUser(s.conn, "tester")
//	assert.NoError(s.T(), err)
//
//	_, err = s.newProject("test", user.ID)
//	assert.NoError(s.T(), err)
//
//	s.cache.SetExpiration(1 * time.Second)
//	s.cache.ResetCacheCount()
//
//	err = s.conn.Preload("Projects").First(&models.User{}, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	time.Sleep(2 * time.Second)
//	err = s.conn.Preload("Projects").First(&models.User{}, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//
//	_, miss := s.cache.GetCacheCount()
//	assert.Equal(s.T(), 2, miss)
//	// reset expiration
//	s.cache.SetExpiration(10 * time.Minute)
//}
//
//func (s *GormCachePluginTestSuite) Test_New_User() {
//	s.cache.ResetCache()
//	var user models.User
//	email := "test@email.com"
//	err := s.conn.Preload("Identities").Where(&models.User{Email: email}).First(&user).Error
//	assert.True(s.T(), errors.Is(err, gorm.ErrRecordNotFound))
//	var users []models.User
//
//	err = s.conn.Preload("Identities").Find(&users).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(users))
//
//	_, err = s.newUser(s.conn, "test")
//	assert.NoError(s.T(), err)
//	var user2 models.User
//	err = s.conn.Preload("Identities").Where(&models.User{Email: email}).First(&user2).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), email, user2.Email)
//
//	err = s.conn.Preload("Identities").Find(&users).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(users))
//
//	err = s.conn.Delete(&user2).Error
//	assert.NoError(s.T(), err)
//	err = s.conn.Preload("Identities").Find(&users).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(users))
//}
//
//func (s *GormCachePluginTestSuite) Test_Relation_Delete() {
//	s.cache.ResetCache()
//	user, err := s.newUser(s.conn, "tester")
//	assert.NoError(s.T(), err)
//	member, err := s.newUser(s.conn, "member")
//	assert.NoError(s.T(), err)
//
//	project, err := s.newProject("project", user.ID)
//	assert.NoError(s.T(), err)
//
//	// add member to project
//	project.Members = append(project.Members, member)
//	err = s.conn.Save(&project).Error
//	assert.NoError(s.T(), err)
//
//	// load user again
//	var u2 models.User
//	err = s.conn.Preload("Projects").Preload("Projects.Members").First(&u2, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(u2.Projects))
//	assert.Equal(s.T(), 1, len(u2.Projects[0].Members))
//
//	// delete member from project using association
//	err = s.conn.Model(&project).Association("Members").Delete(member)
//	assert.NoError(s.T(), err)
//	var u3 models.User
//	err = s.conn.Preload("Projects").Preload("Projects.Members").First(&u3, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(u3.Projects))
//	assert.Equal(s.T(), 0, len(u3.Projects[0].Members))
//}
//
//func (s *GormCachePluginTestSuite) Test_LoadMember() {
//	s.cache.ResetCache()
//	user, err := s.newUser(s.conn, "tester")
//	assert.NoError(s.T(), err)
//	project, err := s.newProject("test", user.ID)
//	assert.NoError(s.T(), err)
//	member, err := s.newUser(s.conn, "member")
//	assert.NoError(s.T(), err)
//
//	// add member to project
//	project.Members = append(project.Members, member)
//	err = s.conn.Save(&project).Error
//	assert.NoError(s.T(), err)
//
//	// first load project without preload
//	var p2 models.Project
//	err = s.conn.First(&p2, "id = ?", project.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(p2.Members))
//
//	var p models.Project
//	err = s.conn.Preload("NotificationChannels").Preload("Members").First(&p, "id = ?", project.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(p.Members))
//	assert.Equal(s.T(), member.ID, p.Members[0].ID)
//}
//
//func (s *GormCachePluginTestSuite) Test_Complex_Relation() {
//	s.cache.ResetCache()
//	user, err := s.newUser(s.conn, "tester")
//	assert.NoError(s.T(), err)
//	member, err := s.newUser(s.conn, "member")
//	assert.NoError(s.T(), err)
//
//	project, err := s.newProject("project", user.ID)
//	assert.NoError(s.T(), err)
//
//	var u models.User
//	err = s.conn.Preload("Projects").Preload("Projects.Members").First(&u, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(u.Projects))
//	// no members
//	assert.Equal(s.T(), 0, len(u.Projects[0].Members))
//
//	// add member to project
//	project.Members = append(project.Members, member)
//	err = s.conn.Save(&project).Error
//	assert.NoError(s.T(), err)
//
//	// load user again
//	var u2 models.User
//	err = s.conn.Preload("Projects").Preload("Projects.Members").First(&u2, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(u2.Projects))
//	assert.Equal(s.T(), 1, len(u2.Projects[0].Members))
//	var projects []models.Project
//	err = s.conn.Model(member).Association("SharedProjects").Find(&projects)
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(projects))
//
//	// delete member from project, but only operate on the join table
//	err = s.conn.Where("user_id =? and project_id =? ", member.ID, project.ID).Delete(&models.ProjectMember{}).Error
//	assert.NoError(s.T(), err)
//	var u3 models.User
//	err = s.conn.Preload("Projects").Preload("Projects.Members").First(&u3, "id = ?", user.ID).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(u3.Projects))
//	assert.Equal(s.T(), 0, len(u3.Projects[0].Members))
//
//	var projects2 []models.Project
//	err = s.conn.Model(member).Association("SharedProjects").Find(&projects2)
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(projects2))
//}
//
//func (s *GormCachePluginTestSuite) Test_NonKey_Query() {
//	s.cache.ResetCache()
//	_, err := s.newUser(s.conn, "tester")
//	assert.NoError(s.T(), err)
//
//	var users []models.User
//	// for this kind of query, the cached value should be invalidated when the table is updated
//	err = s.conn.Preload("Projects").Find(&users, "username like ? or first_name like ?", "test%", "test%").Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(users))
//
//	// this should invalidate the cache
//	_, err = s.newUser(s.conn, "tester2")
//	assert.NoError(s.T(), err)
//
//	err = s.conn.Preload("Projects").Find(&users, "username like ? or first_name like ?", "test%", "test%").Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 2, len(users))
//}
//
//func (s *GormCachePluginTestSuite) Test_EmptyValue() {
//	s.cache.ResetCache()
//	var users []models.User
//	err := s.conn.Find(&users, "username = ? ", "test").Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(users))
//	var user models.User
//	result := s.conn.Where(&models.User{Username: "test"}).First(&user)
//	assert.True(s.T(), errors.Is(result.Error, gorm.ErrRecordNotFound))
//
//	// no matter how many times we query, the empty value should be cached
//	for i := 0; i < 10; i++ {
//		err := s.conn.Find(&users, "username = ? ", "test").Error
//		assert.NoError(s.T(), err)
//		assert.Equal(s.T(), 0, len(users))
//
//		result := s.conn.Where(&models.User{Username: "test"}).First(&user)
//		assert.True(s.T(), errors.Is(result.Error, gorm.ErrRecordNotFound))
//	}
//	hit, miss := s.cache.GetCacheCount()
//	assert.Equal(s.T(), 20, hit)
//	assert.Equal(s.T(), 2, miss)
//
//	// now we add a user
//	u, err := s.newUser(s.conn, "test")
//	assert.NoError(s.T(), err)
//	// the cache should be invalidated immediately, and the query should return the new user
//	err = s.conn.Find(&users, "username = ? ", "test").Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(users))
//	err = s.conn.Where(&models.User{Username: "test"}).First(&user).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), user.ID, u.ID)
//}
//
//func (s *GormCachePluginTestSuite) Test_UserOrgTier() {
//	s.cache.ResetCache()
//	user1, err := s.newUser(s.conn, "user1")
//	assert.NoError(s.T(), err)
//	_, err = s.newUser(s.conn, "user2")
//	assert.NoError(s.T(), err)
//
//	// admin get all users, default all users are Free tier
//	var users []models.User
//	err = s.conn.Find(&users).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 2, len(users))
//	assert.Equal(s.T(), protos.Tier_FREE, protos.Tier(users[0].Tier))
//	assert.Equal(s.T(), protos.Tier_FREE, protos.Tier(users[1].Tier))
//
//	// admin update user1 to Pro tier
//	err = s.conn.Model(&models.Owner{Name: user1.Username}).Update("tier", protos.Tier_PRO).Error
//	assert.NoError(s.T(), err)
//
//	// admin get all users, user1 is Pro tier
//	users = nil
//	err = s.conn.Find(&users).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 2, len(users))
//
//	user := users[0]
//	assert.Equal(s.T(), protos.Tier_PRO, protos.Tier(user.Tier))
//	assert.Equal(s.T(), protos.Tier_FREE, protos.Tier(users[1].Tier))
//}
//
//func (s *GormCachePluginTestSuite) Test_UserOrg() {
//	s.cache.ResetCache()
//	user, err := s.newUser(s.conn, "user")
//	assert.NoError(s.T(), err)
//	org, err := s.newOrg("org")
//	assert.NoError(s.T(), err)
//
//	ret, err := s.getOrg("org")
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(ret.Members))
//
//	orgs, err := s.getOrgsByUser(user.ID)
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 0, len(orgs))
//
//	userOrg := models.UserOrganization{
//		UserID:         user.ID,
//		OrganizationID: org.ID,
//		Role:           string(models.OrgAdmin),
//	}
//	err = s.conn.Create(&userOrg).Error
//	assert.NoError(s.T(), err)
//
//	ret, err = s.getOrg("org")
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(ret.Members))
//
//	orgs, err = s.getOrgsByUser(user.ID)
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(orgs))
//
//	member, err := s.newUser(s.conn, "member")
//	assert.NoError(s.T(), err)
//	memOrg := models.UserOrganization{
//		UserID:         member.ID,
//		OrganizationID: org.ID,
//		Role:           string(models.OrgMember),
//	}
//
//	// add a member to the org
//	err = s.conn.Create(&memOrg).Error
//	assert.NoError(s.T(), err)
//
//	ret, err = s.getOrg("org")
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 2, len(ret.Members))
//
//	orgs, err = s.getOrgsByUser(user.ID)
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(orgs))
//	assert.Equal(s.T(), 2, len(orgs[0].Members))
//}
//
//func (s *GormCachePluginTestSuite) Test_Identity() {
//	identity := &models.Identity{Sub: "test-sub"}
//
//	err := s.conn.Preload("User").First(identity).Error
//	assert.True(s.T(), errors.Is(err, gorm.ErrRecordNotFound))
//
//	user, err := s.newUser(s.conn, "test")
//	assert.NoError(s.T(), err)
//	identity.UserID = user.ID
//	err = s.conn.Create(identity).Error
//	assert.NoError(s.T(), err)
//
//	err = s.conn.Preload("User").First(identity).Error
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), user.ID, identity.UserID)
//
//}
//
//func (s *GormCachePluginTestSuite) Test_UserOrg_Project() {
//	s.cache.ResetCache()
//	user, err := s.newUser(s.conn, "user")
//	assert.NoError(s.T(), err)
//	org, err := s.newOrg("org")
//	assert.NoError(s.T(), err)
//
//	userOrg := models.UserOrganization{
//		UserID:         user.ID,
//		OrganizationID: org.ID,
//		Role:           string(models.OrgAdmin),
//	}
//	err = s.conn.Create(&userOrg).Error
//	assert.NoError(s.T(), err)
//
//	ret, err := s.getOrgsByUser(user.ID)
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(ret[0].Members))
//	assert.Equal(s.T(), 0, len(ret[0].Projects))
//
//	// add a project to the org
//	prj, err := s.newOrgProject("test-project", org.ID)
//	assert.NoError(s.T(), err)
//
//	ret, err = s.getOrgsByUser(user.ID)
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(ret[0].Members))
//	assert.Equal(s.T(), 1, len(ret[0].Projects))
//	assert.Equal(s.T(), prj.ID, ret[0].Projects[0].ID)
//
//	s.conn.Delete(&prj)
//	ret, err = s.getOrgsByUser(user.ID)
//	assert.NoError(s.T(), err)
//	assert.Equal(s.T(), 1, len(ret[0].Members))
//	assert.Equal(s.T(), 0, len(ret[0].Projects))
//}
//
//func (s *GormCachePluginTestSuite) getOrgsByUser(userID string) ([]*models.Organization, error) {
//	var orgs []*models.Organization
//	err := s.conn.Joins("JOIN user_organizations ON organizations.id = user_organizations.organization_id").
//		Preload("Members").Preload("Members.User").Preload("Projects").
//		Where("user_organizations.user_id = ?", userID).Find(&orgs).Error
//	return orgs, err
//}
//
//func (s *GormCachePluginTestSuite) getOrg(name string) (*models.Organization, error) {
//	ret := &models.Organization{}
//	err := s.conn.Preload("Members").Preload("Members.User").Preload("Projects").Limit(1).
//		Where("id =? or LOWER(name) = LOWER(?)", name, name).Find(ret).Error
//	return ret, err
//}
//
//func (s *GormCachePluginTestSuite) newOrgProject(slug, orgID string) (*models.Project, error) {
//	project := models.Project{
//		ID:          "project-" + gonanoid.Must(8),
//		Slug:        slug,
//		DisplayName: slug + " project",
//		Description: slug + " description",
//		OwnerID:     orgID,
//		OwnerType:   "organizations",
//		Public:      false,
//	}
//	err := s.conn.Create(&project).Error
//	return &project, err
//}
