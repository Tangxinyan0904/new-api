package controller

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// relayActivityRpmWindow is the window "RPM" counts finished requests in.
const relayActivityRpmWindow = time.Minute

type relayActivityStat struct {
	// Concurrency is the requests being served right now.
	Concurrency int64 `json:"concurrency"`
	// Rpm is the requests that finished (consume logs) in the last minute.
	Rpm int64 `json:"rpm"`
}

type tokenRelayActivityStat struct {
	TokenId int `json:"token_id"`
	relayActivityStat
}

// GetAllRelayActivity returns the site-wide requests in flight and RPM.
func GetAllRelayActivity(c *gin.Context) {
	respondRelayActivity(c, 0)
}

// GetSelfRelayActivity returns the current user's requests in flight and RPM
// across all their API keys.
func GetSelfRelayActivity(c *gin.Context) {
	respondRelayActivity(c, c.GetInt("id"))
}

func respondRelayActivity(c *gin.Context, userId int) {
	activity, err := service.ClusterRelayActivity()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rpm, err := model.CountRecentConsumeLogs(userId, time.Now().Add(-relayActivityRpmWindow).Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	stat := relayActivityStat{Concurrency: activity.Total, Rpm: rpm}
	if userId != 0 {
		stat.Concurrency = activity.Users[userId]
	}
	common.ApiSuccess(c, stat)
}

// GetTokenRelayActivity returns the requests in flight and RPM of each of the
// current user's API keys that has any.
func GetTokenRelayActivity(c *gin.Context) {
	userId := c.GetInt("id")
	activity, err := service.ClusterRelayActivity()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rpmByToken, err := model.CountRecentConsumeLogsByToken(userId, time.Now().Add(-relayActivityRpmWindow).Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var tokenIds []int
	if err := model.DB.Model(&model.Token{}).Where("user_id = ?", userId).Pluck("id", &tokenIds).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	stats := make([]tokenRelayActivityStat, 0)
	for _, tokenId := range tokenIds {
		stat := relayActivityStat{Concurrency: activity.Tokens[tokenId], Rpm: rpmByToken[tokenId]}
		if stat.Concurrency != 0 || stat.Rpm != 0 {
			stats = append(stats, tokenRelayActivityStat{TokenId: tokenId, relayActivityStat: stat})
		}
	}
	common.ApiSuccess(c, stats)
}
