package model

import "gorm.io/gorm"

// migrateQuotaDataCacheTokens moves the cache read totals this fork recorded
// in quota_data.cache_tokens into upstream's cache_read_tokens column. It adds
// rather than copies, so rows another node already counted in the new column
// keep that count, and it zeroes cache_tokens, so later startups update nothing.
func migrateQuotaDataCacheTokens() error {
	if !DB.Migrator().HasColumn(&QuotaData{}, "cache_tokens") {
		return nil
	}
	return DB.Table("quota_data").Where("cache_tokens > ?", 0).Updates(map[string]any{
		"cache_read_tokens": gorm.Expr("cache_read_tokens + cache_tokens"),
		"cache_tokens":      0,
	}).Error
}
