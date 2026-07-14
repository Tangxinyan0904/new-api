package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetPageQueryNormalizesPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		target   string
		expected int
	}{
		{
			name:     "negative page falls back to first page",
			target:   "/?p=-1",
			expected: 1,
		},
		{
			name:     "zero page falls back to first page",
			target:   "/?p=0",
			expected: 1,
		},
		{
			name:     "missing page falls back to first page",
			target:   "/",
			expected: 1,
		},
		{
			name:     "invalid page falls back to first page",
			target:   "/?p=invalid",
			expected: 1,
		},
		{
			name:     "positive page is preserved",
			target:   "/?p=3",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, tt.target, nil)

			pageInfo := GetPageQuery(ctx)

			assert.Equal(t, tt.expected, pageInfo.Page)
		})
	}
}

func TestGetPageQueryNormalizesPageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		target   string
		expected int
	}{
		{
			name:     "negative page_size falls back to default",
			target:   "/?page_size=-1",
			expected: ItemsPerPage,
		},
		{
			name:     "negative page_size uses positive ps alias",
			target:   "/?page_size=-1&ps=25",
			expected: 25,
		},
		{
			name:     "negative ps uses positive size alias",
			target:   "/?ps=-1&size=30",
			expected: 30,
		},
		{
			name:     "positive page_size is preserved",
			target:   "/?page_size=40",
			expected: 40,
		},
		{
			name:     "page_size above limit is capped",
			target:   "/?page_size=101",
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, tt.target, nil)

			pageInfo := GetPageQuery(ctx)

			assert.Equal(t, tt.expected, pageInfo.PageSize)
		})
	}
}
