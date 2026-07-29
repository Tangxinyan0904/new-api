package model

import (
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

func setupRegistrationIPAbuseTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()

	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(
		sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"),
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
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
	})
	return db
}
