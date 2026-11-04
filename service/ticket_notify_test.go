package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
)

func TestTicketNoticesFollowEachAdministratorsSwitch(t *testing.T) {
	on, off := true, false
	for _, tc := range []struct {
		name     string
		role     int
		switched *bool
		want     bool
	}{
		{"root who never chose", common.RoleRootUser, nil, true},
		{"root who turned them off", common.RoleRootUser, &off, false},
		{"admin who never chose", common.RoleAdminUser, nil, false},
		{"admin who turned them on", common.RoleAdminUser, &on, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, receivesTicketNotices(tc.role, dto.UserSetting{TicketNotifyEnabled: tc.switched}))
		})
	}
}
