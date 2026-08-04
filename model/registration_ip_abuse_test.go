package model

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

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
				require.ErrorIs(t, err, ErrInvalidRegistrationIP)
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

func TestRegisterSelfServiceUserBlockAdvancesAuthStateAndRevokesSessions(t *testing.T) {
	setupRegistrationIPAbuseTestDB(t)
	useUserCacheMiniRedis(t)
	const registrationIP = "203.0.113.23"
	users := make([]*User, 0, RegistrationIPAccountLimit+1)

	for index := 1; index <= RegistrationIPAccountLimit; index++ {
		user := newRegistrationIPTestUser(index)
		_, err := RegisterSelfServiceUser(user, 0, registrationIP, nil)
		require.NoError(t, err)
		require.NoError(t, populateUserCache(*user))
		require.NoError(t, CreateUserSession(&UserSession{
			SID:             fmt.Sprintf("registration-ip-block-%d", index),
			UserID:          user.Id,
			UserAuthVersion: user.AuthVersion,
			RefreshHash:     fmt.Sprintf("registration-ip-refresh-%d", index),
			ExpiresAt:       time.Now().Add(time.Hour).Unix(),
		}))
		users = append(users, user)
	}

	fourth := newRegistrationIPTestUser(RegistrationIPAccountLimit + 1)
	result, err := RegisterSelfServiceUser(fourth, 0, registrationIP, nil)
	require.NoError(t, err)
	require.True(t, result.TriggeredBlock)
	users = append(users, fourth)

	for index, user := range users {
		var stored User
		require.NoError(t, DB.First(&stored, user.Id).Error)
		assert.EqualValues(t, 2, stored.AuthVersion)
		assert.Equal(t, common.UserStatusDisabled, stored.Status)

		cached, cacheErr := GetUserCache(user.Id)
		require.NoError(t, cacheErr)
		assert.EqualValues(t, 2, cached.AuthVersion)
		assert.Equal(t, common.UserStatusDisabled, cached.Status)

		if index < RegistrationIPAccountLimit {
			var session UserSession
			require.NoError(t, DB.First(&session, "sid = ?", fmt.Sprintf("registration-ip-block-%d", index+1)).Error)
			assert.Equal(t, UserSessionStatusRevoked, session.Status)
			assert.Equal(t, "user_security_changed", session.RevokedReason)
		}
	}
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

func TestRegisterSelfServiceUserDoesNotRecreateRestorationAfterConcurrentAdminDisable(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	const registrationIP = "203.0.113.24"
	var existingUserIDs []int
	for index := 1; index <= RegistrationIPAccountLimit; index++ {
		user := newRegistrationIPTestUser(index)
		_, err := RegisterSelfServiceUser(user, 0, registrationIP, nil)
		require.NoError(t, err)
		existingUserIDs = append(existingUserIDs, user.Id)
	}

	interleaved := installRegistrationIPAdminDisableInterleaving(t, db, existingUserIDs[0])
	fourth := newRegistrationIPTestUser(RegistrationIPAccountLimit + 1)
	result, err := RegisterSelfServiceUser(fourth, 0, registrationIP, nil)
	require.NoError(t, err)
	require.True(t, *interleaved)
	assert.NotContains(t, result.AffectedUserIDs, existingUserIDs[0])

	var user User
	require.NoError(t, db.First(&user, existingUserIDs[0]).Error)
	assert.Equal(t, common.UserStatusDisabled, user.Status)
	var account RegistrationIPAccount
	require.NoError(t, db.Where("user_id = ?", existingUserIDs[0]).First(&account).Error)
	assert.False(t, account.RestoreEligible)
	assert.Zero(t, account.AutoDisabledAt)
}

func TestUnblockRegistrationIPRestoresOnlyEligibleNonDeletedUsers(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	const registrationIP = "203.0.113.30"
	userIDs := createBlockedRegistrationIPFixture(t, registrationIP)

	require.NoError(t, SetUserStatusByAdmin(userIDs[0], common.UserStatusDisabled))
	require.NoError(t, db.Delete(&User{}, userIDs[1]).Error)

	result, err := UnblockRegistrationIP(registrationIP)
	require.NoError(t, err)
	assert.Equal(t, registrationIP, result.CanonicalIP)
	assert.ElementsMatch(t, userIDs[2:], result.AffectedUserIDs)
	assert.Equal(t, 2, result.AffectedAccountCount)
	assert.False(t, result.Allowlisted)

	var state RegistrationIPState
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.Equal(t, 2, state.CurrentCycle)
	assert.Zero(t, state.RegistrationCount)
	assert.Zero(t, state.BlockedAt)

	var users []User
	require.NoError(t, db.Unscoped().Where("id IN ?", userIDs).Order("id ASC").Find(&users).Error)
	require.Len(t, users, 4)
	assert.Equal(t, common.UserStatusDisabled, users[0].Status)
	assert.Equal(t, common.UserStatusDisabled, users[1].Status)
	assert.True(t, users[1].DeletedAt.Valid)
	assert.Equal(t, common.UserStatusEnabled, users[2].Status)
	assert.Equal(t, common.UserStatusEnabled, users[3].Status)

	var oldCycleAccounts []RegistrationIPAccount
	require.NoError(t, db.Where("registration_ip = ? AND registration_cycle = ?", registrationIP, 1).Find(&oldCycleAccounts).Error)
	for _, account := range oldCycleAccounts {
		assert.False(t, account.RestoreEligible)
		assert.Zero(t, account.AutoDisabledAt)
	}

	idempotent, err := UnblockRegistrationIP(registrationIP)
	require.NoError(t, err)
	assert.Empty(t, idempotent.AffectedUserIDs)
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.Equal(t, 2, state.CurrentCycle)
}

func TestUnblockRegistrationIPAdvancesAuthStateAndRevokesSessions(t *testing.T) {
	setupRegistrationIPAbuseTestDB(t)
	useUserCacheMiniRedis(t)
	const registrationIP = "203.0.113.34"
	userIDs := createBlockedRegistrationIPFixture(t, registrationIP)

	for index, userID := range userIDs {
		var user User
		require.NoError(t, DB.First(&user, userID).Error)
		require.NoError(t, populateUserCache(user))
		require.NoError(t, CreateUserSession(&UserSession{
			SID:             fmt.Sprintf("registration-ip-restore-%d", index),
			UserID:          userID,
			UserAuthVersion: user.AuthVersion,
			RefreshHash:     fmt.Sprintf("registration-ip-restore-refresh-%d", index),
			ExpiresAt:       time.Now().Add(time.Hour).Unix(),
		}))
	}

	result, err := UnblockRegistrationIP(registrationIP)
	require.NoError(t, err)
	assert.ElementsMatch(t, userIDs, result.AffectedUserIDs)

	for index, userID := range userIDs {
		var stored User
		require.NoError(t, DB.First(&stored, userID).Error)
		assert.EqualValues(t, 3, stored.AuthVersion)
		assert.Equal(t, common.UserStatusEnabled, stored.Status)

		cached, cacheErr := GetUserCache(userID)
		require.NoError(t, cacheErr)
		assert.EqualValues(t, 3, cached.AuthVersion)
		assert.Equal(t, common.UserStatusEnabled, cached.Status)

		var session UserSession
		require.NoError(t, DB.First(&session, "sid = ?", fmt.Sprintf("registration-ip-restore-%d", index)).Error)
		assert.Equal(t, UserSessionStatusRevoked, session.Status)
		assert.Equal(t, "user_security_changed", session.RevokedReason)
	}
}

func TestUnblockRegistrationIPDoesNotOverrideConcurrentAdminDisable(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	const registrationIP = "203.0.113.35"
	userIDs := createBlockedRegistrationIPFixture(t, registrationIP)

	interleaved := installRegistrationIPAdminDisableInterleaving(t, db, userIDs[0])
	result, err := UnblockRegistrationIP(registrationIP)
	require.NoError(t, err)
	require.True(t, *interleaved)
	assert.NotContains(t, result.AffectedUserIDs, userIDs[0])

	var user User
	require.NoError(t, db.First(&user, userIDs[0]).Error)
	assert.Equal(t, common.UserStatusDisabled, user.Status)
	var account RegistrationIPAccount
	require.NoError(t, db.Where("user_id = ?", userIDs[0]).First(&account).Error)
	assert.False(t, account.RestoreEligible)
	assert.Zero(t, account.AutoDisabledAt)
}

func TestSetUserStatusByAdminClearsAutomaticRestorationMarker(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	const registrationIP = "203.0.113.31"
	userIDs := createBlockedRegistrationIPFixture(t, registrationIP)

	require.NoError(t, SetUserStatusByAdmin(userIDs[0], common.UserStatusEnabled))
	var account RegistrationIPAccount
	require.NoError(t, db.Where("user_id = ?", userIDs[0]).First(&account).Error)
	assert.False(t, account.RestoreEligible)
	assert.Zero(t, account.AutoDisabledAt)
	var user User
	require.NoError(t, db.First(&user, userIDs[0]).Error)
	assert.Equal(t, common.UserStatusEnabled, user.Status)

	require.NoError(t, SetUserStatusByAdmin(userIDs[1], common.UserStatusDisabled))
	account = RegistrationIPAccount{}
	require.NoError(t, db.Where("user_id = ?", userIDs[1]).First(&account).Error)
	assert.False(t, account.RestoreEligible)
	assert.Zero(t, account.AutoDisabledAt)

	result, err := UnblockRegistrationIP(registrationIP)
	require.NoError(t, err)
	assert.NotContains(t, result.AffectedUserIDs, userIDs[0])
	assert.NotContains(t, result.AffectedUserIDs, userIDs[1])
}

func TestRegistrationIPAllowlistRestoresAndStartsFreshCycles(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	const registrationIP = "203.0.113.32"
	blockedUserIDs := createBlockedRegistrationIPFixture(t, registrationIP)

	added, err := AddRegistrationIPAllowlist(registrationIP)
	require.NoError(t, err)
	assert.True(t, added.Allowlisted)
	assert.ElementsMatch(t, blockedUserIDs, added.AffectedUserIDs)

	var state RegistrationIPState
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.True(t, state.Allowlisted)
	assert.Equal(t, 2, state.CurrentCycle)
	assert.Zero(t, state.RegistrationCount)
	assert.Zero(t, state.BlockedAt)

	duplicateAdd, err := AddRegistrationIPAllowlist(registrationIP)
	require.NoError(t, err)
	assert.Empty(t, duplicateAdd.AffectedUserIDs)
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.Equal(t, 2, state.CurrentCycle)

	allowlistedUser := newRegistrationIPTestUser(10)
	registrationResult, err := RegisterSelfServiceUser(allowlistedUser, 0, registrationIP, nil)
	require.NoError(t, err)
	assert.False(t, registrationResult.TriggeredBlock)
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.Zero(t, state.RegistrationCount)

	removed, err := RemoveRegistrationIPAllowlist(registrationIP)
	require.NoError(t, err)
	assert.False(t, removed.Allowlisted)
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.False(t, state.Allowlisted)
	assert.Equal(t, 3, state.CurrentCycle)
	assert.Zero(t, state.RegistrationCount)

	duplicateRemove, err := RemoveRegistrationIPAllowlist(registrationIP)
	require.NoError(t, err)
	assert.Empty(t, duplicateRemove.AffectedUserIDs)
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.Equal(t, 3, state.CurrentCycle)

	newCycleUser := newRegistrationIPTestUser(11)
	_, err = RegisterSelfServiceUser(newCycleUser, 0, registrationIP, nil)
	require.NoError(t, err)
	require.NoError(t, db.Where("ip = ?", registrationIP).First(&state).Error)
	assert.Equal(t, 1, state.RegistrationCount)

	items, total, err := ListRegistrationIPAllowlist(&common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, items)
}

func TestListBlockedRegistrationIPsSearchesAssociatedUsersAndIncludesDeleted(t *testing.T) {
	db := setupRegistrationIPAbuseTestDB(t)
	const registrationIP = "203.0.113.33"
	userIDs := createBlockedRegistrationIPFixture(t, registrationIP)
	require.NoError(t, db.Model(&User{}).Where("id = ?", userIDs[0]).Updates(map[string]interface{}{
		"username":     "needle-user",
		"display_name": "Needle Display",
	}).Error)
	require.NoError(t, db.Delete(&User{}, userIDs[1]).Error)

	keywords := []string{
		registrationIP,
		"::ffff:" + registrationIP,
		strconv.Itoa(userIDs[0]),
		"needle-user",
		"Needle Display",
	}
	for _, keyword := range keywords {
		t.Run(keyword, func(t *testing.T) {
			items, total, err := ListBlockedRegistrationIPs(
				keyword,
				&common.PageInfo{Page: 1, PageSize: 10},
			)
			require.NoError(t, err)
			assert.EqualValues(t, 1, total)
			require.Len(t, items, 1)
			assert.Equal(t, registrationIP, items[0].IP)
			assert.Equal(t, RegistrationIPAccountLimit+1, items[0].RegistrationCount)
			assert.Equal(t, RegistrationIPAccountLimit+1, items[0].AssociatedAccountCount)
			require.Len(t, items[0].Accounts, RegistrationIPAccountLimit+1)
			assert.NotZero(t, items[0].BlockedAt)
		})
	}

	items, _, err := ListBlockedRegistrationIPs(
		registrationIP,
		&common.PageInfo{Page: 1, PageSize: 10},
	)
	require.NoError(t, err)
	deletedFound := false
	for _, account := range items[0].Accounts {
		if account.UserId == userIDs[1] {
			deletedFound = true
			assert.True(t, account.Deleted)
			assert.Equal(t, common.UserStatusDisabled, account.Status)
		}
	}
	assert.True(t, deletedFound)
}

func createBlockedRegistrationIPFixture(t *testing.T, registrationIP string) []int {
	t.Helper()
	userIDs := make([]int, 0, RegistrationIPAccountLimit+1)
	for index := 1; index <= RegistrationIPAccountLimit+1; index++ {
		user := newRegistrationIPTestUser(index)
		_, err := RegisterSelfServiceUser(user, 0, registrationIP, nil)
		require.NoError(t, err)
		userIDs = append(userIDs, user.Id)
	}
	return userIDs
}

func newRegistrationIPTestUser(index int) *User {
	return &User{
		Username:    fmt.Sprintf("registration-ip-%d", index),
		DisplayName: fmt.Sprintf("Registration IP %d", index),
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
}

func installRegistrationIPAdminDisableInterleaving(t *testing.T, db *gorm.DB, userID int) *bool {
	t.Helper()
	const beforeCallback = "registration_ip_test:admin_disable_before_user_read"
	const afterCallback = "registration_ip_test:admin_disable_after_user_read"
	armed := false
	mutateAfterUserRead := false
	interleaved := false

	applyAdminDisable := func(tx *gorm.DB) {
		mutationDB := tx.Session(&gorm.Session{NewDB: true})
		if err := mutationDB.Model(&User{}).
			Where("id = ?", userID).
			Update("status", common.UserStatusDisabled).Error; err != nil {
			tx.AddError(err)
			return
		}
		if err := mutationDB.Model(&RegistrationIPAccount{}).
			Where("user_id = ?", userID).
			Updates(map[string]interface{}{
				"auto_disabled_at": 0,
				"restore_eligible": false,
			}).Error; err != nil {
			tx.AddError(err)
			return
		}
		interleaved = true
	}

	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(beforeCallback, func(tx *gorm.DB) {
		if !armed || tx.Statement.Table != "users" {
			return
		}
		switch tx.Statement.Dest.(type) {
		case *User:
			armed = false
			applyAdminDisable(tx)
		case *[]int:
			mutateAfterUserRead = true
		}
	}))
	require.NoError(t, db.Callback().Query().After("gorm:after_query").Register(afterCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "registration_ip_accounts" {
			if _, ok := tx.Statement.Dest.(*[]int); ok {
				armed = true
			}
			return
		}
		if !armed || !mutateAfterUserRead || tx.Statement.Table != "users" {
			return
		}
		if _, ok := tx.Statement.Dest.(*[]int); !ok {
			return
		}
		armed = false
		mutateAfterUserRead = false
		applyAdminDisable(tx)
	}))
	return &interleaved
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
		&UserSession{},
		&Token{},
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
