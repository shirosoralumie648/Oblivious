package affinity

import (
	"sync"
	"time"

	"oblivious/server/internal/relay/types"
)

// ConversationAffinity 渠道亲和性管理器
// conversation_id -> channel_id 映射，支持故障自动切换和语义缓存跨渠道共享
type ConversationAffinity struct {
	mu             sync.RWMutex
	mappings       map[string]*types.AffinityMapping  // conversation_id -> mapping
	orgIndex       map[string][]string                 // org_id -> [conversation_ids]
	ttl            time.Duration
	maxFailovers   int
	onFailover     func(conversationID, oldChannelID, newChannelID string)
}

// NewConversationAffinity 创建渠道亲和性管理器
func NewConversationAffinity(ttl time.Duration, maxFailovers int) *ConversationAffinity {
	return &ConversationAffinity{
		mappings:     make(map[string]*types.AffinityMapping),
		orgIndex:     make(map[string][]string),
		ttl:          ttl,
		maxFailovers: maxFailovers,
	}
}

// SetFailoverCallback 设置故障切换回调
func (ca *ConversationAffinity) SetFailoverCallback(fn func(conversationID, oldChannelID, newChannelID string)) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.onFailover = fn
}

// GetChannel 获取会话绑定的渠道 ID
// 返回 channelID, true 如果有绑定；"", false 如果没有或已过期
func (ca *ConversationAffinity) GetChannel(conversationID string) (string, bool) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	mapping, ok := ca.mappings[conversationID]
	if !ok {
		return "", false
	}

	// 检查 TTL
	if ca.ttl > 0 && time.Since(mapping.LastUsedAt) > ca.ttl {
		return "", false
	}

	return mapping.ChannelID, true
}

// Bind 绑定会话到渠道
func (ca *ConversationAffinity) Bind(conversationID, channelID, orgID string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	now := time.Now()
	ca.mappings[conversationID] = &types.AffinityMapping{
		ConversationID: conversationID,
		ChannelID:      channelID,
		OrgID:          orgID,
		CreatedAt:      now,
		LastUsedAt:     now,
	}

	// 更新组织索引
	if orgID != "" {
		if _, ok := ca.orgIndex[orgID]; !ok {
			ca.orgIndex[orgID] = make([]string, 0)
		}
		// 避免重复
		found := false
		for _, cid := range ca.orgIndex[orgID] {
			if cid == conversationID {
				found = true
				break
			}
		}
		if !found {
			ca.orgIndex[orgID] = append(ca.orgIndex[orgID], conversationID)
		}
	}
}

// Failover 故障切换
// 当当前渠道不可用时，尝试切换到 newChannelID
// 如果超过最大切换次数，清除亲和性
func (ca *ConversationAffinity) Failover(conversationID, newChannelID string) bool {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	mapping, ok := ca.mappings[conversationID]
	if !ok {
		return false
	}

	if ca.maxFailovers > 0 && mapping.FailoverCount >= ca.maxFailovers {
		// 超过最大切换次数，清除亲和性
		delete(ca.mappings, conversationID)
		return false
	}

	oldChannelID := mapping.ChannelID
	mapping.ChannelID = newChannelID
	mapping.FailoverCount++
	mapping.LastUsedAt = time.Now()

	// 触发回调
	if ca.onFailover != nil {
		go ca.onFailover(conversationID, oldChannelID, newChannelID)
	}

	return true
}

// Touch 更新会话最后使用时间
func (ca *ConversationAffinity) Touch(conversationID string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if mapping, ok := ca.mappings[conversationID]; ok {
		mapping.LastUsedAt = time.Now()
	}
}

// Remove 移除会话绑定
func (ca *ConversationAffinity) Remove(conversationID string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.removeMapping(conversationID)
}

// RemoveByOrg 移除组织下所有会话绑定
func (ca *ConversationAffinity) RemoveByOrg(orgID string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	conversations, ok := ca.orgIndex[orgID]
	if !ok {
		return
	}

	for _, cid := range conversations {
		delete(ca.mappings, cid)
	}
	delete(ca.orgIndex, orgID)
}

// Cleanup 清理过期映射
func (ca *ConversationAffinity) Cleanup() int {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if ca.ttl <= 0 {
		return 0
	}

	removed := 0
	now := time.Now()
	for cid, mapping := range ca.mappings {
		if now.Sub(mapping.LastUsedAt) > ca.ttl {
			ca.removeMapping(cid)
			removed++
		}
	}

	return removed
}

// Stats 返回亲和性统计
func (ca *ConversationAffinity) Stats() AffinityStats {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	stats := AffinityStats{
		TotalMappings: len(ca.mappings),
		OrgCount:      len(ca.orgIndex),
	}

	for _, mapping := range ca.mappings {
		if mapping.FailoverCount > 0 {
			stats.FailoverMappings++
		}
	}

	return stats
}

// AffinityStats 亲和性统计
type AffinityStats struct {
	TotalMappings    int `json:"total_mappings"`
	FailoverMappings int `json:"failover_mappings"`
	OrgCount         int `json:"org_count"`
}

// IsCacheShareable 检查会话缓存是否可以跨渠道共享
// 语义缓存的公共缓存部分天然跨渠道共享
// 私有缓存通过 org_id 隔离，同组织不同渠道可共享
func (ca *ConversationAffinity) IsCacheShareable(conversationID1, conversationID2 string) bool {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	m1, ok1 := ca.mappings[conversationID1]
	m2, ok2 := ca.mappings[conversationID2]

	if !ok1 || !ok2 {
		return false
	}

	// 同组织即可共享
	return m1.OrgID != "" && m1.OrgID == m2.OrgID
}

func (ca *ConversationAffinity) removeMapping(conversationID string) {
	mapping, ok := ca.mappings[conversationID]
	if !ok {
		return
	}

	// 从组织索引中移除
	if mapping.OrgID != "" {
		if conversations, ok := ca.orgIndex[mapping.OrgID]; ok {
			for i, cid := range conversations {
				if cid == conversationID {
					ca.orgIndex[mapping.OrgID] = append(conversations[:i], conversations[i+1:]...)
					break
				}
			}
			if len(ca.orgIndex[mapping.OrgID]) == 0 {
				delete(ca.orgIndex, mapping.OrgID)
			}
		}
	}

	delete(ca.mappings, conversationID)
}
