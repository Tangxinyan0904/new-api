package model

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNormalizeRegistrationIP(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "IPv4", raw: "203.0.113.7", want: "203.0.113.7"},
		{name: "mapped IPv4", raw: "::ffff:203.0.113.7", want: "203.0.113.7"},
		{name: "IPv6", raw: "2001:0db8::1", want: "2001:db8::1"},
		{name: "IPv6 zone", raw: "fe80::1%eth0", want: "fe80::1"},
		{name: "surrounding spaces", raw: " 203.0.113.8 ", want: "203.0.113.8"},
		{name: "CIDR", raw: "203.0.113.0/24", wantErr: true},
		{name: "host and port", raw: "203.0.113.7:443", wantErr: true},
		{name: "hostname", raw: "example.com", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeRegistrationIP(test.raw)
			if test.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestRegistrationIPModelsMigrateWithPortableUniqueConstraints(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)

	state := RegistrationIPState{
		IP:           "203.0.113.10",
		CurrentCycle: 1,
	}
	require.NoError(t, db.Create(&state).Error)
	duplicateState := RegistrationIPState{
		IP:           state.IP,
		CurrentCycle: 1,
	}
	require.Error(t, db.Create(&duplicateState).Error)

	user := User{
		Username: "registration-ip-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "regip1",
	}
	require.NoError(t, db.Create(&user).Error)
	account := RegistrationIPAccount{
		UserId:            user.Id,
		RegistrationIP:    state.IP,
		RegistrationCycle: state.CurrentCycle,
	}
	require.NoError(t, db.Create(&account).Error)
	duplicateAccount := RegistrationIPAccount{
		UserId:            user.Id,
		RegistrationIP:    "203.0.113.11",
		RegistrationCycle: 1,
	}
	require.Error(t, db.Create(&duplicateAccount).Error)
}

func TestRegisterSelfServiceUserBlocksFourthAccountAndRejectsLaterAttempts(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	const registrationIP = "203.0.113.20"
	createdUserIDs := make([]int, 0, RegistrationIPAccountLimit+1)

	for index := 1; index <= RegistrationIPAccountLimit; index++ {
		user := newRegistrationIPTestUser(index)
		result, err := RegisterSelfServiceUser(user, 0, registrationIP, nil)
		require.NoError(t, err)
		assert.False(t, result.TriggeredBlock)
		assert.Equal(t, registrationIP, result.CanonicalIP)
		assert.Empty(t, result.AffectedUserIDs)
		createdUserIDs = append(createdUserIDs, user.Id)
	}

	fourthUser := newRegistrationIPTestUser(RegistrationIPAccountLimit + 1)
	result, err := RegisterSelfServiceUser(fourthUser, 0, registrationIP, nil)
	require.NoError(t, err)
	assert.True(t, result.TriggeredBlock)
	createdUserIDs = append(createdUserIDs, fourthUser.Id)
	assert.ElementsMatch(t, createdUserIDs, result.AffectedUserIDs)
	assert.Equal(t, common.UserStatusDisabled, fourthUser.Status)

	var state RegistrationIPState
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.Equal(t, 1, state.CurrentCycle)
	assert.Equal(t, RegistrationIPAccountLimit+1, state.RegistrationCount)
	assert.Positive(t, state.BlockedAt)
	assert.False(t, state.Allowlisted)

	var users []User
	require.NoError(t, db.Where("id IN ?", createdUserIDs).Find(&users).Error)
	require.Len(t, users, RegistrationIPAccountLimit+1)
	for _, user := range users {
		assert.Equal(t, common.UserStatusDisabled, user.Status)
	}

	var accounts []RegistrationIPAccount
	require.NoError(t, db.Where("registration_ip = ? AND registration_cycle = ?", registrationIP, 1).Find(&accounts).Error)
	require.Len(t, accounts, RegistrationIPAccountLimit+1)
	for _, account := range accounts {
		assert.True(t, account.RestoreEligible)
		assert.Positive(t, account.AutoDisabledAt)
	}

	var usersBefore int64
	require.NoError(t, db.Model(&User{}).Count(&usersBefore).Error)
	fifthUser := newRegistrationIPTestUser(RegistrationIPAccountLimit + 2)
	blockedResult, err := RegisterSelfServiceUser(fifthUser, 0, registrationIP, nil)
	require.ErrorIs(t, err, ErrRegistrationIPBlocked)
	assert.Nil(t, blockedResult)
	assert.Zero(t, fifthUser.Id)
	var usersAfter int64
	require.NoError(t, db.Model(&User{}).Count(&usersAfter).Error)
	assert.Equal(t, usersBefore, usersAfter)
}

func TestRegisterSelfServiceUserRollsBackProviderCallbackFailure(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	callbackErr := errors.New("provider binding failed")
	user := newRegistrationIPTestUser(1)

	result, err := RegisterSelfServiceUser(
		user,
		0,
		"203.0.113.21",
		func(tx *gorm.DB) error {
			require.NotNil(t, tx)
			return callbackErr
		},
	)

	require.ErrorIs(t, err, callbackErr)
	assert.Nil(t, result)
	var userCount int64
	require.NoError(t, db.Model(&User{}).Count(&userCount).Error)
	assert.Zero(t, userCount)
	var stateCount int64
	require.NoError(t, db.Model(&RegistrationIPState{}).Count(&stateCount).Error)
	assert.Zero(t, stateCount)
	var accountCount int64
	require.NoError(t, db.Model(&RegistrationIPAccount{}).Count(&accountCount).Error)
	assert.Zero(t, accountCount)
}

func TestRegisterSelfServiceUserSerializesConcurrentThreshold(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	const registrationIP = "203.0.113.22"
	const registrations = RegistrationIPAccountLimit + 1
	start := make(chan struct{})
	errorsByAttempt := make([]error, registrations)
	users := make([]*User, registrations)
	var waitGroup sync.WaitGroup

	for index := range registrations {
		users[index] = newRegistrationIPTestUser(index + 1)
		waitGroup.Add(1)
		go func(attempt int) {
			defer waitGroup.Done()
			<-start
			_, errorsByAttempt[attempt] = RegisterSelfServiceUser(
				users[attempt],
				0,
				registrationIP,
				nil,
			)
		}(index)
	}
	close(start)
	waitGroup.Wait()

	for _, err := range errorsByAttempt {
		require.NoError(t, err)
	}
	var state RegistrationIPState
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.Equal(t, registrations, state.RegistrationCount)
	assert.Positive(t, state.BlockedAt)
	var enabledCount int64
	require.NoError(t, db.Model(&User{}).Where("status = ?", common.UserStatusEnabled).Count(&enabledCount).Error)
	assert.Zero(t, enabledCount)
	var accountCount int64
	require.NoError(t, db.Model(&RegistrationIPAccount{}).Where("registration_ip = ?", registrationIP).Count(&accountCount).Error)
	assert.EqualValues(t, registrations, accountCount)
}

func newRegistrationIPTestUser(index int) *User {
	return &User{
		Username:    fmt.Sprintf("registration-ip-%d", index),
		DisplayName: fmt.Sprintf("Registration IP %d", index),
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
}

func setupRegistrationIPAbuseTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()

	previousQuotaForNewUser := common.QuotaForNewUser
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(
		sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"),
		&gorm.Config{},
	)
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&User{},
		&RegistrationIPState{},
		&RegistrationIPAccount{},
	))

	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		DB = previousDB
		LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.QuotaForNewUser = previousQuotaForNewUser
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
	})
	return db
}
