package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	SemanticCacheScopeGlobal       SemanticCacheScope = "global"
	SemanticCacheScopeOrganization SemanticCacheScope = "org"

	DefaultSemanticCacheGlobalTTL       = 24 * time.Hour
	DefaultSemanticCacheOrganizationTTL = time.Hour
)

var (
	semanticCacheEmailPattern = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	semanticCachePhonePattern = regexp.MustCompile(`(?:\+?\d[\d\s().-]{7,}\d)`)
)

type SemanticCacheScope string

type SemanticCacheRequest struct {
	OrganizationID   string
	OrganizationName string
	UserID           string
	UserName         string
	Model            string
	Query            string
	QueryEmbedding   []float32
	ChannelID        string
	UserScoped       bool
}

type SemanticCacheOptions struct {
	Now             func() time.Time
	GlobalTTL       time.Duration
	OrganizationTTL time.Duration
}

type SemanticCacheKey struct {
	Scope          SemanticCacheScope
	OrganizationID string
	Model          string
	QueryHash      string
}

func (k SemanticCacheKey) Namespace() string {
	if k.Scope == SemanticCacheScopeOrganization {
		return "org:" + k.OrganizationID + ":cache:" + k.Model + ":" + k.QueryHash
	}
	return "global:cache:" + k.Model + ":" + k.QueryHash
}

type SemanticCachePolicy struct {
	Scope     SemanticCacheScope
	TTL       time.Duration
	Key       SemanticCacheKey
	Sensitive bool
}

type SemanticCacheEntry struct {
	Key            SemanticCacheKey
	Query          string
	QueryEmbedding []float32
	Response       json.RawMessage
	HitCount       int
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

type SemanticCacheHit = SemanticCacheEntry

type SemanticCacheStore interface {
	Get(ctx context.Context, key SemanticCacheKey) (*SemanticCacheEntry, error)
	Put(ctx context.Context, entry SemanticCacheEntry) error
	IncrementHit(ctx context.Context, key SemanticCacheKey) error
}

type SemanticCacheSimilarStore interface {
	FindSimilar(ctx context.Context, key SemanticCacheKey, query string) (*SemanticCacheEntry, error)
}

type SemanticCacheVectorSimilarStore interface {
	FindSimilarByEmbedding(ctx context.Context, key SemanticCacheKey, queryEmbedding []float32) (*SemanticCacheEntry, error)
}

type SemanticCache struct {
	store           SemanticCacheStore
	now             func() time.Time
	globalTTL       time.Duration
	organizationTTL time.Duration
}

func NewSemanticCache(store SemanticCacheStore, opts SemanticCacheOptions) *SemanticCache {
	if store == nil {
		store = NewInMemorySemanticCacheStore()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	globalTTL := opts.GlobalTTL
	if globalTTL <= 0 {
		globalTTL = DefaultSemanticCacheGlobalTTL
	}
	organizationTTL := opts.OrganizationTTL
	if organizationTTL <= 0 {
		organizationTTL = DefaultSemanticCacheOrganizationTTL
	}
	return &SemanticCache{
		store:           store,
		now:             now,
		globalTTL:       globalTTL,
		organizationTTL: organizationTTL,
	}
}

func (c *SemanticCache) PolicyFor(req SemanticCacheRequest) SemanticCachePolicy {
	sensitive := isSensitiveSemanticCacheQuery(req)
	scope := SemanticCacheScopeGlobal
	ttl := c.globalTTL
	organizationID := ""
	if sensitive {
		scope = SemanticCacheScopeOrganization
		ttl = c.organizationTTL
		organizationID = req.OrganizationID
	}

	key := SemanticCacheKey{
		Scope:          scope,
		OrganizationID: organizationID,
		Model:          strings.TrimSpace(req.Model),
		QueryHash:      SemanticCacheQueryHash(req.Model, req.Query),
	}
	return SemanticCachePolicy{
		Scope:     scope,
		TTL:       ttl,
		Key:       key,
		Sensitive: sensitive,
	}
}

func (c *SemanticCache) Lookup(ctx context.Context, req SemanticCacheRequest) (*SemanticCacheHit, error) {
	policy := c.PolicyFor(req)
	keys := []SemanticCacheKey{policy.Key}
	if policy.Scope == SemanticCacheScopeGlobal && req.OrganizationID != "" {
		orgKey := policy.Key
		orgKey.Scope = SemanticCacheScopeOrganization
		orgKey.OrganizationID = req.OrganizationID
		keys = append(keys, orgKey)
	}

	now := c.now()
	for _, key := range keys {
		entry, err := c.store.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if entry == nil || !entry.ExpiresAt.After(now) {
			continue
		}
		if err := c.store.IncrementHit(ctx, key); err != nil {
			return nil, err
		}
		hit := cloneSemanticCacheEntry(*entry)
		hit.HitCount++
		return &hit, nil
	}

	if len(req.QueryEmbedding) > 0 {
		vectorStore, ok := c.store.(SemanticCacheVectorSimilarStore)
		if ok {
			for _, key := range keys {
				entry, err := vectorStore.FindSimilarByEmbedding(ctx, key, req.QueryEmbedding)
				if err != nil {
					return nil, err
				}
				if entry == nil || !entry.ExpiresAt.After(now) {
					continue
				}
				if err := c.store.IncrementHit(ctx, entry.Key); err != nil {
					return nil, err
				}
				hit := cloneSemanticCacheEntry(*entry)
				hit.HitCount++
				return &hit, nil
			}
		}
	}

	similarStore, ok := c.store.(SemanticCacheSimilarStore)
	if !ok {
		return nil, nil
	}
	for _, key := range keys {
		entry, err := similarStore.FindSimilar(ctx, key, req.Query)
		if err != nil {
			return nil, err
		}
		if entry == nil || !entry.ExpiresAt.After(now) {
			continue
		}
		if err := c.store.IncrementHit(ctx, entry.Key); err != nil {
			return nil, err
		}
		hit := cloneSemanticCacheEntry(*entry)
		hit.HitCount++
		return &hit, nil
	}
	return nil, nil
}

func (c *SemanticCache) Store(ctx context.Context, req SemanticCacheRequest, response json.RawMessage) (SemanticCacheEntry, error) {
	policy := c.PolicyFor(req)
	now := c.now()
	entry := SemanticCacheEntry{
		Key:            policy.Key,
		Query:          req.Query,
		QueryEmbedding: cloneFloat32Slice(req.QueryEmbedding),
		Response:       cloneRawMessage(response),
		CreatedAt:      now,
		ExpiresAt:      now.Add(policy.TTL),
	}
	if err := c.store.Put(ctx, entry); err != nil {
		return SemanticCacheEntry{}, err
	}
	return cloneSemanticCacheEntry(entry), nil
}

func IsSensitiveSemanticCacheRequest(req SemanticCacheRequest) bool {
	return isSensitiveSemanticCacheQuery(req)
}

func SemanticCacheQueryHash(model, query string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(model) + "\x00" + strings.TrimSpace(query)))
	return hex.EncodeToString(sum[:])
}

type InMemorySemanticCacheStore struct {
	mu      sync.RWMutex
	entries map[SemanticCacheKey]SemanticCacheEntry
}

func NewInMemorySemanticCacheStore() *InMemorySemanticCacheStore {
	return &InMemorySemanticCacheStore{entries: make(map[SemanticCacheKey]SemanticCacheEntry)}
}

func (s *InMemorySemanticCacheStore) Get(ctx context.Context, key SemanticCacheKey) (*SemanticCacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[key]
	if !ok {
		return nil, nil
	}
	cloned := cloneSemanticCacheEntry(entry)
	return &cloned, nil
}

func (s *InMemorySemanticCacheStore) Put(ctx context.Context, entry SemanticCacheEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.Key] = cloneSemanticCacheEntry(entry)
	return nil
}

func (s *InMemorySemanticCacheStore) IncrementHit(ctx context.Context, key SemanticCacheKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return nil
	}
	entry.HitCount++
	s.entries[key] = entry
	return nil
}

func (s *InMemorySemanticCacheStore) FindSimilar(ctx context.Context, key SemanticCacheKey, query string) (*SemanticCacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var best *SemanticCacheEntry
	bestScore := 0.0
	for _, entry := range s.entries {
		if entry.Key.Scope != key.Scope || entry.Key.OrganizationID != key.OrganizationID || entry.Key.Model != key.Model {
			continue
		}
		score := semanticCacheQuerySimilarity(query, entry.Query)
		if score < semanticCacheSimilarityThreshold {
			continue
		}
		if best == nil || score > bestScore || (score == bestScore && entry.CreatedAt.After(best.CreatedAt)) {
			cloned := cloneSemanticCacheEntry(entry)
			best = &cloned
			bestScore = score
		}
	}
	return best, nil
}

func (s *InMemorySemanticCacheStore) FindSimilarByEmbedding(ctx context.Context, key SemanticCacheKey, queryEmbedding []float32) (*SemanticCacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var best *SemanticCacheEntry
	bestScore := 0.0
	for _, entry := range s.entries {
		if entry.Key.Scope != key.Scope || entry.Key.OrganizationID != key.OrganizationID || entry.Key.Model != key.Model || entry.Key.QueryHash == key.QueryHash {
			continue
		}
		score := semanticCacheEmbeddingSimilarity(queryEmbedding, entry.QueryEmbedding)
		if score < semanticCacheEmbeddingSimilarityThreshold {
			continue
		}
		if best == nil || score > bestScore || (score == bestScore && entry.CreatedAt.After(best.CreatedAt)) {
			cloned := cloneSemanticCacheEntry(entry)
			best = &cloned
			bestScore = score
		}
	}
	return best, nil
}

func isSensitiveSemanticCacheQuery(req SemanticCacheRequest) bool {
	if req.UserScoped {
		return true
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return true
	}
	if semanticCacheEmailPattern.MatchString(query) || semanticCachePhonePattern.MatchString(query) {
		return true
	}

	lowerQuery := strings.ToLower(query)
	if containsFoldedValue(lowerQuery, req.OrganizationName) || containsFoldedValue(lowerQuery, req.UserName) || containsFoldedValue(lowerQuery, req.UserID) {
		return true
	}

	sensitivePhrases := []string{
		"我的", "我们公司", "我們公司", "我司", "本公司", "我的账户", "我的帳戶",
		"my account", "my company", "our company", "our organization", "our org",
	}
	for _, phrase := range sensitivePhrases {
		if strings.Contains(lowerQuery, phrase) {
			return true
		}
	}
	return false
}

const (
	semanticCacheSimilarityThreshold          = 0.7
	semanticCacheEmbeddingSimilarityThreshold = 0.8
)

var semanticCacheTokenPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

func semanticCacheQuerySimilarity(a, b string) float64 {
	aTokens := semanticCacheTokenSet(a)
	bTokens := semanticCacheTokenSet(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}

	intersection := 0
	for token := range aTokens {
		if _, ok := bTokens[token]; ok {
			intersection++
		}
	}
	union := len(aTokens) + len(bTokens) - intersection
	if union == 0 {
		return 0
	}
	jaccard := float64(intersection) / float64(union)
	containment := float64(intersection) / float64(min(len(aTokens), len(bTokens)))
	if containment == 1 && intersection >= 3 {
		return 1
	}
	return jaccard
}

func semanticCacheEmbeddingSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, aNorm, bNorm float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		aNorm += av * av
		bNorm += bv * bv
	}
	if aNorm == 0 || bNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(aNorm) * math.Sqrt(bNorm))
}

func semanticCacheTokenSet(query string) map[string]struct{} {
	tokens := semanticCacheTokenPattern.FindAllString(strings.ToLower(query), -1)
	set := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" || semanticCacheStopWords[token] {
			continue
		}
		set[token] = struct{}{}
	}
	return set
}

var semanticCacheStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "for": true, "how": true,
	"in": true, "is": true, "of": true, "on": true, "or": true, "please": true,
	"the": true, "to": true, "what": true,
}

func containsFoldedValue(lowerHaystack, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return strings.Contains(lowerHaystack, strings.ToLower(value))
}

func cloneSemanticCacheEntry(entry SemanticCacheEntry) SemanticCacheEntry {
	entry.QueryEmbedding = cloneFloat32Slice(entry.QueryEmbedding)
	entry.Response = cloneRawMessage(entry.Response)
	return entry
}

func cloneFloat32Slice(values []float32) []float32 {
	if len(values) == 0 {
		return nil
	}
	copied := make([]float32, len(values))
	copy(copied, values)
	return copied
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	copied := make([]byte, len(raw))
	copy(copied, raw)
	return copied
}
