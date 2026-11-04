package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateQuotaDataCacheTokensMovesForkTotalsOnce(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("ALTER TABLE quota_data ADD COLUMN cache_tokens integer DEFAULT 0").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM quota_data")
		require.NoError(t, DB.Exec("ALTER TABLE quota_data DROP COLUMN cache_tokens").Error)
	})

	rows := []struct {
		modelName  string
		forkCache  int
		cacheReads int64
		want       int64
	}{
		{"recorded-before-upgrade", 800, 0, 800},
		// Another node already counted part of this hour in the new column.
		{"recorded-by-both-nodes", 100, 50, 150},
		{"recorded-after-upgrade", 0, 30, 30},
	}
	for _, row := range rows {
		quotaData := QuotaData{ModelName: row.modelName, CreatedAt: 3600, CacheReadTokens: row.cacheReads}
		require.NoError(t, DB.Create(&quotaData).Error)
		require.NoError(t, DB.Exec("UPDATE quota_data SET cache_tokens = ? WHERE id = ?", row.forkCache, quotaData.Id).Error)
	}

	// A second startup must not count the totals again.
	require.NoError(t, migrateQuotaDataCacheTokens())
	require.NoError(t, migrateQuotaDataCacheTokens())

	for _, row := range rows {
		var got struct {
			CacheReadTokens int64
			CacheTokens     int
		}
		require.NoError(t, DB.Table("quota_data").Select("cache_read_tokens, cache_tokens").Where("model_name = ?", row.modelName).Scan(&got).Error)
		assert.Equal(t, row.want, got.CacheReadTokens, row.modelName)
		assert.Zero(t, got.CacheTokens, row.modelName)
	}
}

func TestMigrateQuotaDataCacheTokensSkipsTablesWithoutTheForkColumn(t *testing.T) {
	require.False(t, DB.Migrator().HasColumn(&QuotaData{}, "cache_tokens"))
	assert.NoError(t, migrateQuotaDataCacheTokens())
}
