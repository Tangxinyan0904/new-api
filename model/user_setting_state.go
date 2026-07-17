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

func updateQuotaWarningEmailState(userId int, sent bool, requireRecoveredBalance bool) (bool, error) {
	changed := false
	settingValue := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).
			Select("id", "quota", "setting").
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
		if requireRecoveredBalance {
			threshold := common.QuotaRemindThreshold
			if record.QuotaWarningThreshold != 0 {
				threshold = int(record.QuotaWarningThreshold)
			}
			if user.Quota < threshold {
				return nil
			}
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
	return updateQuotaWarningEmailState(userId, true, false)
}

func ReleaseQuotaWarningEmail(userId int) error {
	_, err := updateQuotaWarningEmailState(userId, false, false)
	return err
}

func RearmQuotaWarningEmail(userId int) error {
	_, err := updateQuotaWarningEmailState(userId, false, false)
	return err
}

func RearmQuotaWarningEmailIfRecovered(userId int) error {
	_, err := updateQuotaWarningEmailState(userId, false, true)
	return err
}

func rearmQuotaWarningEmailAfterCredit(userId int) {
	if err := RearmQuotaWarningEmailIfRecovered(userId); err != nil {
		common.SysLog("failed to rearm quota warning email: " + err.Error())
	}
}
