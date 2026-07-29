package model

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
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
