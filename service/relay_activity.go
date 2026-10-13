package service

import (
	"maps"
	"os"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// RelayActivityCounts is how many relay requests are being served right now,
// in total and per user and token. Users and tokens without a request in
// flight are left out.
type RelayActivityCounts struct {
	Total  int64         `json:"total"`
	Users  map[int]int64 `json:"users,omitempty"`
	Tokens map[int]int64 `json:"tokens,omitempty"`
}

var (
	relayActivityMu sync.Mutex
	relayActivity   = RelayActivityCounts{Users: map[int]int64{}, Tokens: map[int]int64{}}
)

// BeginRelayRequest counts a relay request as in flight on this node until
// the returned function is called.
func BeginRelayRequest(userId, tokenId int) (end func()) {
	relayActivityMu.Lock()
	relayActivity.Total++
	if userId > 0 {
		relayActivity.Users[userId]++
	}
	if tokenId > 0 {
		relayActivity.Tokens[tokenId]++
	}
	relayActivityMu.Unlock()

	return func() {
		relayActivityMu.Lock()
		defer relayActivityMu.Unlock()
		relayActivity.Total--
		if userId > 0 {
			releaseRelayActivity(relayActivity.Users, userId)
		}
		if tokenId > 0 {
			releaseRelayActivity(relayActivity.Tokens, tokenId)
		}
	}
}

func releaseRelayActivity(counts map[int]int64, id int) {
	if counts[id] <= 1 {
		delete(counts, id)
		return
	}
	counts[id]--
}

// LocalRelayActivity returns a copy of this node's requests in flight.
func LocalRelayActivity() RelayActivityCounts {
	relayActivityMu.Lock()
	defer relayActivityMu.Unlock()
	return RelayActivityCounts{
		Total:  relayActivity.Total,
		Users:  maps.Clone(relayActivity.Users),
		Tokens: maps.Clone(relayActivity.Tokens),
	}
}

// ClusterRelayActivity adds what the other live nodes reported in their last
// system instance report to this node's current requests in flight.
func ClusterRelayActivity() (RelayActivityCounts, error) {
	counts := LocalRelayActivity()
	instances, err := model.ListSystemInstances()
	if err != nil {
		return counts, err
	}
	self := currentSystemInstanceName()
	now := common.GetTimestamp()
	for _, instance := range instances {
		if instance.NodeName == self || now-instance.LastSeenAt > model.SystemInstanceStaleAfterSeconds {
			continue
		}
		var info struct {
			RelayActivity *RelayActivityCounts `json:"relay_activity"`
		}
		if common.UnmarshalJsonStr(instance.Info, &info) != nil || info.RelayActivity == nil {
			continue
		}
		counts.Total += info.RelayActivity.Total
		for id, n := range info.RelayActivity.Users {
			counts.Users[id] += n
		}
		for id, n := range info.RelayActivity.Tokens {
			counts.Tokens[id] += n
		}
	}
	return counts, nil
}

// currentSystemInstanceName is the node name this process reports its system
// instance under: the configured node name, or the hostname.
func currentSystemInstanceName() string {
	if name := strings.TrimSpace(common.GetNodeIdentity().Name); name != "" {
		return name
	}
	hostname, _ := os.Hostname()
	return strings.TrimSpace(hostname)
}
