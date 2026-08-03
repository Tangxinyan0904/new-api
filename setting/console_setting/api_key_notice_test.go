package console_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAPIKeyNotice(t *testing.T) {
	tests := []struct {
		name    string
		notice  string
		wantErr string
	}{
		{
			name:   "accepts empty notice",
			notice: "",
		},
		{
			name:   "accepts five hundred unicode characters",
			notice: strings.Repeat("密", 500),
		},
		{
			name:    "rejects more than five hundred unicode characters",
			notice:  strings.Repeat("密", 501),
			wantErr: "API 密钥公告不能超过 500 个字符",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKeyNotice(tt.notice)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}
