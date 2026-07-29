package model

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const RegistrationIPAccountLimit = 3

var (
	ErrRegistrationIPBlocked       = errors.New("registration IP is blocked")
	ErrRegistrationIPLimitExceeded = errors.New("registration IP account limit exceeded")
)

type RegistrationIPState struct {
	Id                int    `json:"id"`
	IP                string `json:"ip" gorm:"type:varchar(45);not null;uniqueIndex"`
	CurrentCycle      int    `json:"current_cycle" gorm:"not null"`
	RegistrationCount int    `json:"registration_count" gorm:"not null"`
	BlockedAt         int64  `json:"blocked_at" gorm:"not null;index"`
	Allowlisted       bool   `json:"allowlisted" gorm:"not null;index"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

type RegistrationIPAccount struct {
	Id                int    `json:"id"`
	UserId            int    `json:"user_id" gorm:"not null;uniqueIndex"`
	RegistrationIP    string `json:"registration_ip" gorm:"type:varchar(45);not null;index"`
	RegistrationCycle int    `json:"registration_cycle" gorm:"not null;index"`
	AutoDisabledAt    int64  `json:"auto_disabled_at" gorm:"not null"`
	RestoreEligible   bool   `json:"restore_eligible" gorm:"not null"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

type SelfServiceRegistrationResult struct {
	CanonicalIP     string
	TriggeredBlock  bool
	AffectedUserIDs []int
}

func NormalizeRegistrationIP(raw string) (string, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid registration IP: %w", err)
	}
	if addr.Zone() != "" {
		addr = addr.WithZone("")
	}
	return addr.Unmap().String(), nil
}

func RegisterSelfServiceUser(
	user *User,
	inviterID int,
	rawIP string,
	afterCreate func(tx *gorm.DB) error,
) (*SelfServiceRegistrationResult, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	canonicalIP, err := NormalizeRegistrationIP(rawIP)
	if err != nil {
		return nil, err
	}

	result := SelfServiceRegistrationResult{CanonicalIP: canonicalIP}
	err = DB.Transaction(func(tx *gorm.DB) error {
		candidate := RegistrationIPState{
			IP:           canonicalIP,
			CurrentCycle: 1,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ip"}},
			DoNothing: true,
		}).Create(&candidate).Error; err != nil {
			return err
		}

		var state RegistrationIPState
		if err := lockForUpdate(tx).Where("ip = ?", canonicalIP).First(&state).Error; err != nil {
			return err
		}
		if state.BlockedAt > 0 && !state.Allowlisted {
			return ErrRegistrationIPBlocked
		}

		if err := user.InsertWithTx(tx, inviterID); err != nil {
			return err
		}
		if afterCreate != nil {
			if err := afterCreate(tx); err != nil {
				return err
			}
		}

		account := RegistrationIPAccount{
			UserId:            user.Id,
			RegistrationIP:    canonicalIP,
			RegistrationCycle: state.CurrentCycle,
		}
		if err := tx.Create(&account).Error; err != nil {
			return err
		}
		if state.Allowlisted {
			return nil
		}

		registrationCount := state.RegistrationCount + 1
		if err := tx.Model(&state).Update("registration_count", registrationCount).Error; err != nil {
			return err
		}
		if registrationCount <= RegistrationIPAccountLimit {
			return nil
		}

		var cycleUserIDs []int
		if err := tx.Model(&RegistrationIPAccount{}).
			Where("registration_ip = ? AND registration_cycle = ?", canonicalIP, state.CurrentCycle).
			Pluck("user_id", &cycleUserIDs).Error; err != nil {
			return err
		}
		var enabledUserIDs []int
		if err := tx.Model(&User{}).
			Where("id IN ? AND status = ?", cycleUserIDs, common.UserStatusEnabled).
			Pluck("id", &enabledUserIDs).Error; err != nil {
			return err
		}
		sort.Ints(enabledUserIDs)
		now := time.Now().Unix()
		if len(enabledUserIDs) > 0 {
			if err := tx.Model(&User{}).
				Where("id IN ?", enabledUserIDs).
				Update("status", common.UserStatusDisabled).Error; err != nil {
				return err
			}
			if err := tx.Model(&RegistrationIPAccount{}).
				Where("user_id IN ?", enabledUserIDs).
				Updates(map[string]interface{}{
					"auto_disabled_at": now,
					"restore_eligible": true,
				}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&state).Update("blocked_at", now).Error; err != nil {
			return err
		}

		result.TriggeredBlock = true
		result.AffectedUserIDs = enabledUserIDs
		for _, userID := range enabledUserIDs {
			if userID == user.Id {
				user.Status = common.UserStatusDisabled
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	user.FinishInsert(inviterID)
	for _, userID := range result.AffectedUserIDs {
		if err := InvalidateUserCache(userID); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate user cache after registration IP block for user %d: %s", userID, err.Error()))
		}
		if err := InvalidateUserTokensCache(userID); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate token cache after registration IP block for user %d: %s", userID, err.Error()))
		}
	}
	if result.TriggeredBlock {
		logger.LogWarn(
			context.Background(),
			fmt.Sprintf("registration IP threshold triggered: ip=%s affected_user_ids=%v", canonicalIP, result.AffectedUserIDs),
		)
	}
	return &result, nil
}
