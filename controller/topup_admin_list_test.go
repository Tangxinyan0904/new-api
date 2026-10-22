package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllTopUpsFiltersByStatusAndUser(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}))
	require.NoError(t, db.Create(&[]model.User{
		{Id: 1, Username: "alice", AffCode: "a1"},
		{Id: 2, Username: "bob", AffCode: "b2"},
		// A numeric username: "1" is both user 1's ID and user 3's name.
		{Id: 3, Username: "1", AffCode: "c3"},
	}).Error)
	require.NoError(t, db.Create(&[]model.TopUp{
		{UserId: 1, TradeNo: "A1", Status: common.TopUpStatusPending, CreateTime: 1},
		{UserId: 1, TradeNo: "A2", Status: common.TopUpStatusSuccess, CreateTime: 2},
		{UserId: 2, TradeNo: "B1", Status: common.TopUpStatusSuccess, CreateTime: 3},
		{UserId: 3, TradeNo: "C1", Status: common.TopUpStatusPending, CreateTime: 4},
		{UserId: 3, TradeNo: "C2", Status: common.TopUpStatusFailed, CreateTime: 5},
	}).Error)
	// A deleted user's orders stay listed and findable by name.
	require.NoError(t, db.Delete(&model.User{}, 2).Error)

	testCases := []struct {
		name      string
		query     string
		wantTrade []string
	}{
		{name: "no filter lists everything newest first", query: "", wantTrade: []string{"C2", "C1", "B1", "A2", "A1"}},
		{name: "status", query: "status=pending", wantTrade: []string{"C1", "A1"}},
		{name: "username", query: "user=alice", wantTrade: []string{"A2", "A1"}},
		{name: "deleted user's username", query: "user=bob", wantTrade: []string{"B1"}},
		{name: "number matches that ID and that username", query: "user=1", wantTrade: []string{"C2", "C1", "A2", "A1"}},
		{name: "status narrows the user match", query: "user=+1+&status=pending", wantTrade: []string{"C1", "A1"}},
		{name: "unknown user", query: "user=nobody", wantTrade: []string{}},
		{name: "order number with status", query: "keyword=A1&status=success", wantTrade: []string{}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := callAdminTopUps(t, tc.query)
			require.True(t, response.Success, response.Message)
			trades := make([]string, 0, len(response.Data.Items))
			for _, item := range response.Data.Items {
				trades = append(trades, item.TradeNo)
			}
			assert.Equal(t, tc.wantTrade, trades)
			assert.Equal(t, len(tc.wantTrade), response.Data.Total)
		})
	}

	t.Run("rows carry the owner's username", func(t *testing.T) {
		response := callAdminTopUps(t, "")
		usernames := map[string]string{}
		for _, item := range response.Data.Items {
			usernames[item.TradeNo] = item.Username
		}
		assert.Equal(t, map[string]string{"A1": "alice", "A2": "alice", "B1": "bob", "C1": "1", "C2": "1"}, usernames)
	})

	t.Run("unknown status is rejected", func(t *testing.T) {
		response := callAdminTopUps(t, "status=paid")
		assert.False(t, response.Success)
	})
}

type adminTopUpsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int `json:"total"`
		Items []struct {
			TradeNo  string `json:"trade_no"`
			Username string `json:"username"`
		} `json:"items"`
	} `json:"data"`
}

func callAdminTopUps(t *testing.T, query string) adminTopUpsResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup?p=1&page_size=20&"+query, nil)
	GetAllTopUps(ctx)
	var response adminTopUpsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}
