package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"gorm.io/gorm"
)

type userSettingRecord struct {
	dto.UserSetting
	QuotaWarningEmailSent bool `json:"quota_warning_email_sent,omitempty"`
}

func decodeUserSettingRecord(raw string) (userSettingRecord, error) {
	var record userSettingRecord
	if raw == "" {
		return record, nil
	}
	if err := common.Unmarshal([]byte(raw), &record); err != nil {
		return userSettingRecord{}, err
	}
	return record, nil
}

func updateQuotaWarningEmailState(userId int, sent bool) (bool, error) {
	changed := false
	settingValue := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).
			Select("id", "setting").
			Where("id = ?", userId).
			First(&user).Error; err != nil {
			return err
		}

		record, err := decodeUserSettingRecord(user.Setting)
		if err != nil {
			return err
		}
		if record.QuotaWarningEmailSent == sent {
			return nil
		}
		record.QuotaWarningEmailSent = sent
		settingBytes, err := common.Marshal(record)
		if err != nil {
			return err
		}
		settingValue = string(settingBytes)
		if err := tx.Model(&User{}).
			Where("id = ?", userId).
			Update("setting", settingValue).Error; err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if changed {
		if err := updateUserSettingCache(userId, settingValue); err != nil {
			common.SysLog("failed to update quota warning setting cache: " + err.Error())
		}
	}
	return changed, nil
}

func TryClaimQuotaWarningEmail(userId int) (bool, error) {
	return updateQuotaWarningEmailState(userId, true)
}

func ReleaseQuotaWarningEmail(userId int) error {
	_, err := updateQuotaWarningEmailState(userId, false)
	return err
}
