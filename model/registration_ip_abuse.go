package model

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const RegistrationIPAccountLimit = 3

var (
	ErrInvalidRegistrationIP       = errors.New("invalid registration IP")
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

type RegistrationIPMutationResult struct {
	CanonicalIP          string `json:"ip"`
	AffectedUserIDs      []int  `json:"affected_user_ids"`
	AffectedAccountCount int    `json:"affected_account_count"`
	Allowlisted          bool   `json:"allowlisted"`
}

type RegistrationIPAccountListItem struct {
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Status          int    `json:"status"`
	UserCreatedAt   int64  `json:"user_created_at"`
	RegistrationAt  int64  `json:"registration_at"`
	Deleted         bool   `json:"deleted"`
	RestoreEligible bool   `json:"restore_eligible"`
	AutoDisabledAt  int64  `json:"auto_disabled_at"`
}

type BlockedRegistrationIPListItem struct {
	IP                     string                          `json:"ip"`
	CurrentCycle           int                             `json:"current_cycle"`
	RegistrationCount      int                             `json:"registration_count"`
	BlockedAt              int64                           `json:"blocked_at"`
	AssociatedAccountCount int                             `json:"associated_account_count"`
	Accounts               []RegistrationIPAccountListItem `json:"accounts"`
}

type RegistrationIPAllowlistItem struct {
	IP                string `json:"ip"`
	CurrentCycle      int    `json:"current_cycle"`
	RegistrationCount int    `json:"registration_count"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

func NormalizeRegistrationIP(raw string) (string, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidRegistrationIP, err)
	}
	if addr.Zone() != "" {
		addr = addr.WithZone("")
	}
	return addr.Unmap().String(), nil
}

func registrationIPStateForUpdate(tx *gorm.DB, canonicalIP string) (*RegistrationIPState, error) {
	candidate := RegistrationIPState{
		IP:           canonicalIP,
		CurrentCycle: 1,
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ip"}},
		DoNothing: true,
	}).Create(&candidate).Error; err != nil {
		return nil, err
	}

	var state RegistrationIPState
	if err := lockForUpdate(tx).Where("ip = ?", canonicalIP).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
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
		state, err := registrationIPStateForUpdate(tx, canonicalIP)
		if err != nil {
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

func UnblockRegistrationIP(rawIP string) (*RegistrationIPMutationResult, error) {
	return resetRegistrationIPState(rawIP, nil)
}

func AddRegistrationIPAllowlist(rawIP string) (*RegistrationIPMutationResult, error) {
	allowlisted := true
	return resetRegistrationIPState(rawIP, &allowlisted)
}

func RemoveRegistrationIPAllowlist(rawIP string) (*RegistrationIPMutationResult, error) {
	allowlisted := false
	return resetRegistrationIPState(rawIP, &allowlisted)
}

func resetRegistrationIPState(rawIP string, allowlisted *bool) (*RegistrationIPMutationResult, error) {
	canonicalIP, err := NormalizeRegistrationIP(rawIP)
	if err != nil {
		return nil, err
	}
	result := RegistrationIPMutationResult{CanonicalIP: canonicalIP}
	err = DB.Transaction(func(tx *gorm.DB) error {
		state, err := registrationIPStateForUpdate(tx, canonicalIP)
		if err != nil {
			return err
		}

		if allowlisted == nil && state.BlockedAt == 0 {
			result.Allowlisted = state.Allowlisted
			return nil
		}
		if allowlisted != nil && *allowlisted == state.Allowlisted && state.BlockedAt == 0 {
			result.Allowlisted = state.Allowlisted
			return nil
		}

		if allowlisted == nil || *allowlisted {
			var eligibleUserIDs []int
			if err := tx.Model(&RegistrationIPAccount{}).
				Where(
					"registration_ip = ? AND registration_cycle = ? AND restore_eligible = ?",
					canonicalIP,
					state.CurrentCycle,
					true,
				).
				Pluck("user_id", &eligibleUserIDs).Error; err != nil {
				return err
			}
			if len(eligibleUserIDs) > 0 {
				if err := tx.Model(&User{}).
					Where("id IN ? AND status = ?", eligibleUserIDs, common.UserStatusDisabled).
					Pluck("id", &result.AffectedUserIDs).Error; err != nil {
					return err
				}
				sort.Ints(result.AffectedUserIDs)
				if len(result.AffectedUserIDs) > 0 {
					if err := tx.Model(&User{}).
						Where("id IN ?", result.AffectedUserIDs).
						Update("status", common.UserStatusEnabled).Error; err != nil {
						return err
					}
				}
				if err := tx.Model(&RegistrationIPAccount{}).
					Where(
						"registration_ip = ? AND registration_cycle = ? AND restore_eligible = ?",
						canonicalIP,
						state.CurrentCycle,
						true,
					).
					Updates(map[string]interface{}{
						"auto_disabled_at": 0,
						"restore_eligible": false,
					}).Error; err != nil {
					return err
				}
			}
		}

		resultingAllowlisted := state.Allowlisted
		if allowlisted != nil {
			resultingAllowlisted = *allowlisted
		}
		if err := tx.Model(state).Updates(map[string]interface{}{
			"allowlisted":        resultingAllowlisted,
			"blocked_at":         0,
			"current_cycle":      state.CurrentCycle + 1,
			"registration_count": 0,
		}).Error; err != nil {
			return err
		}
		result.Allowlisted = resultingAllowlisted
		result.AffectedAccountCount = len(result.AffectedUserIDs)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, userID := range result.AffectedUserIDs {
		invalidateRegistrationIPUserCaches(userID)
	}
	return &result, nil
}

func SetUserStatusByAdmin(userID int, status int) error {
	if userID <= 0 {
		return errors.New("user ID must be positive")
	}
	if status != common.UserStatusEnabled && status != common.UserStatusDisabled {
		return errors.New("invalid user status")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if err := tx.Model(&user).Update("status", status).Error; err != nil {
			return err
		}
		return tx.Model(&RegistrationIPAccount{}).
			Where("user_id = ?", userID).
			Updates(map[string]interface{}{
				"auto_disabled_at": 0,
				"restore_eligible": false,
			}).Error
	})
	if err != nil {
		return err
	}
	invalidateRegistrationIPUserCaches(userID)
	return nil
}

func invalidateRegistrationIPUserCaches(userID int) {
	if err := InvalidateUserCache(userID); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate registration IP user cache for user %d: %s", userID, err.Error()))
	}
	if err := InvalidateUserTokensCache(userID); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate registration IP token cache for user %d: %s", userID, err.Error()))
	}
}

func ListBlockedRegistrationIPs(
	keyword string,
	pageInfo *common.PageInfo,
) ([]*BlockedRegistrationIPListItem, int64, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize <= 0 {
		pageInfo.PageSize = common.ItemsPerPage
	}
	if pageInfo.PageSize > 100 {
		pageInfo.PageSize = 100
	}

	query := DB.Model(&RegistrationIPState{}).Where("blocked_at > ?", 0)
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		pattern := "%" + keyword + "%"
		var matchingIPs []string
		accountQuery := DB.Unscoped().Table("registration_ip_accounts").
			Select("DISTINCT registration_ip_accounts.registration_ip").
			Joins("LEFT JOIN users ON users.id = registration_ip_accounts.user_id")
		if userID, parseErr := strconv.Atoi(keyword); parseErr == nil && userID > 0 {
			accountQuery = accountQuery.Where(
				"registration_ip_accounts.user_id = ? OR users.username LIKE ? OR users.display_name LIKE ?",
				userID,
				pattern,
				pattern,
			)
		} else {
			accountQuery = accountQuery.Where(
				"users.username LIKE ? OR users.display_name LIKE ?",
				pattern,
				pattern,
			)
		}
		if err := accountQuery.Pluck("registration_ip_accounts.registration_ip", &matchingIPs).Error; err != nil {
			return nil, 0, err
		}
		if canonicalIP, normalizeErr := NormalizeRegistrationIP(keyword); normalizeErr == nil {
			query = query.Where("ip = ? OR ip IN ?", canonicalIP, matchingIPs)
		} else if len(matchingIPs) > 0 {
			query = query.Where("ip IN ?", matchingIPs)
		} else {
			return []*BlockedRegistrationIPListItem{}, 0, nil
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	states := make([]RegistrationIPState, 0)
	if err := query.
		Order("blocked_at DESC, id DESC").
		Limit(pageInfo.PageSize).
		Offset((pageInfo.Page - 1) * pageInfo.PageSize).
		Find(&states).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*BlockedRegistrationIPListItem, 0, len(states))
	for _, state := range states {
		type accountRow struct {
			UserId          int
			JoinedUserId    int
			Username        string
			DisplayName     string
			Status          int
			UserCreatedAt   int64
			RegistrationAt  int64
			DeletedAt       gorm.DeletedAt
			RestoreEligible bool
			AutoDisabledAt  int64
		}
		rows := make([]accountRow, 0)
		if err := DB.Unscoped().Table("registration_ip_accounts").
			Select(
				"registration_ip_accounts.user_id, users.id AS joined_user_id, users.username, "+
					"users.display_name, users.status, users.created_at AS user_created_at, "+
					"registration_ip_accounts.created_at AS registration_at, users.deleted_at, "+
					"registration_ip_accounts.restore_eligible, registration_ip_accounts.auto_disabled_at",
			).
			Joins("LEFT JOIN users ON users.id = registration_ip_accounts.user_id").
			Where(
				"registration_ip_accounts.registration_ip = ? AND registration_ip_accounts.registration_cycle = ?",
				state.IP,
				state.CurrentCycle,
			).
			Order("registration_ip_accounts.id ASC").
			Scan(&rows).Error; err != nil {
			return nil, 0, err
		}
		accounts := make([]RegistrationIPAccountListItem, 0, len(rows))
		for _, row := range rows {
			accounts = append(accounts, RegistrationIPAccountListItem{
				UserId:          row.UserId,
				Username:        row.Username,
				DisplayName:     row.DisplayName,
				Status:          row.Status,
				UserCreatedAt:   row.UserCreatedAt,
				RegistrationAt:  row.RegistrationAt,
				Deleted:         row.JoinedUserId == 0 || row.DeletedAt.Valid,
				RestoreEligible: row.RestoreEligible,
				AutoDisabledAt:  row.AutoDisabledAt,
			})
		}
		items = append(items, &BlockedRegistrationIPListItem{
			IP:                     state.IP,
			CurrentCycle:           state.CurrentCycle,
			RegistrationCount:      state.RegistrationCount,
			BlockedAt:              state.BlockedAt,
			AssociatedAccountCount: len(accounts),
			Accounts:               accounts,
		})
	}
	return items, total, nil
}

func ListRegistrationIPAllowlist(
	pageInfo *common.PageInfo,
) ([]*RegistrationIPAllowlistItem, int64, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize <= 0 {
		pageInfo.PageSize = common.ItemsPerPage
	}
	if pageInfo.PageSize > 100 {
		pageInfo.PageSize = 100
	}

	query := DB.Model(&RegistrationIPState{}).Where("allowlisted = ?", true)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*RegistrationIPAllowlistItem, 0)
	if err := query.Select(
		"ip, current_cycle, registration_count, created_at, updated_at",
	).
		Order("updated_at DESC, id DESC").
		Limit(pageInfo.PageSize).
		Offset((pageInfo.Page - 1) * pageInfo.PageSize).
		Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
