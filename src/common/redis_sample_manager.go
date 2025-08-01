package common

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis keys for sample data
	RedisSampleKeyPrefix = "sample_data:"
	RedisSampleCountKey  = "sample_count:"
	RedisSampleHashKey   = "sample_hash:"

	// Configuration constants
	DefaultSampleTTL        = 24 * time.Hour // 24 hours TTL
	DefaultMaxSamplesPerKey = 100            // Maximum 100 samples per project-sampler combination
	DefaultCleanupInterval  = 1 * time.Hour  // Cleanup expired data every hour
)

// RedisSampleData represents sample data stored in Redis
type RedisSampleData struct {
	Data                interface{} `json:"data"`
	Timestamp           time.Time   `json:"timestamp"`
	ProjectNodeSequence string      `json:"project_node_sequence"`
	SamplerName         string      `json:"sampler_name"`
	Score               float64     `json:"score"` // Used for sorting by timestamp
}

// RedisSampleManager manages sample data in Redis
type RedisSampleManager struct {
	ttl              time.Duration
	maxSamplesPerKey int
	cleanupTicker    *time.Ticker
	stopChan         chan struct{}
	batchChannel     chan SampleData // Channel for batch processing
	batchTicker      *time.Ticker    // Ticker for batch processing
}

// NewRedisSampleManager creates a new Redis Sample Manager
func NewRedisSampleManager() *RedisSampleManager {
	rsm := &RedisSampleManager{
		ttl:              DefaultSampleTTL,
		maxSamplesPerKey: DefaultMaxSamplesPerKey,
		cleanupTicker:    time.NewTicker(DefaultCleanupInterval),
		stopChan:         make(chan struct{}),
		batchChannel:     make(chan SampleData, 5000),            // Large buffer for batch processing
		batchTicker:      time.NewTicker(200 * time.Millisecond), // Batch every 200ms
	}

	// Start cleanup goroutine
	go rsm.startCleanup()

	// Start batch processing goroutine
	go rsm.startBatchProcessor()

	return rsm
}

// StoreSample stores a sample in Redis with TTL and size limits
func (rsm *RedisSampleManager) StoreSample(samplerName string, sample SampleData) error {
	if rdb == nil {
		return fmt.Errorf("Redis client not available")
	}

	ctx := context.Background()
	// Simplified key structure: sample_data:samplerName:projectNodeSequence
	key := fmt.Sprintf("%s%s:%s", RedisSampleKeyPrefix, samplerName, sample.ProjectNodeSequence)

	// Create Redis sample data
	redisSample := RedisSampleData{
		Data:                sample.Data,
		Timestamp:           sample.Timestamp,
		ProjectNodeSequence: sample.ProjectNodeSequence,
		SamplerName:         samplerName,
		Score:               float64(sample.Timestamp.Unix()),
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(redisSample)
	if err != nil {
		return fmt.Errorf("failed to serialize sample data: %w", err)
	}

	// Deduplication: compute hash only from business content (sequence + data), exclude timestamp
	hashInputBytes, _ := json.Marshal(struct {
		Seq  string      `json:"seq"`
		Data interface{} `json:"data"`
	}{Seq: sample.ProjectNodeSequence, Data: sample.Data})
	hashVal := xxhash.Sum64(hashInputBytes)
	// Simplified hash key: sample_hash:samplerName:projectNodeSequence
	hashKey := fmt.Sprintf("%s%s:%s", RedisSampleHashKey, samplerName, sample.ProjectNodeSequence)

	// SAdd returns 1 if new, 0 if already exists -> skip duplicate sample
	added, err := rdb.SAdd(ctx, hashKey, hashVal).Result()
	if err != nil {
		return fmt.Errorf("failed to add hash set: %w", err)
	}
	// set TTL for hash set key
	rdb.Expire(ctx, hashKey, rsm.ttl)
	if added == 0 {
		// duplicate, do not store
		return nil
	}

	// Use Redis transaction to ensure atomicity
	pipe := rdb.TxPipeline()

	// Add to sorted set (sorted by timestamp)
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  redisSample.Score,
		Member: jsonData,
	})

	// Set TTL on the key
	pipe.Expire(ctx, key, rsm.ttl)

	// Keep only the most recent N samples
	pipe.ZRemRangeByRank(ctx, key, 0, -int64(rsm.maxSamplesPerKey+1))

	// Update sample count with simplified key
	countKey := fmt.Sprintf("%s%s:%s", RedisSampleCountKey, samplerName, sample.ProjectNodeSequence)
	pipe.Incr(ctx, countKey)
	pipe.Expire(ctx, countKey, rsm.ttl)

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store sample in Redis: %w", err)
	}

	return nil
}

// GetSamples retrieves all samples for a specific sampler
func (rsm *RedisSampleManager) GetSamples(samplerName string) (map[string][]SampleData, error) {
	if rdb == nil {
		return nil, fmt.Errorf("Redis client not available")
	}

	ctx := context.Background()
	pattern := fmt.Sprintf("%s%s:*", RedisSampleKeyPrefix, samplerName)

	// Get all keys matching the pattern
	keys, err := rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get sample keys: %w", err)
	}

	result := make(map[string][]SampleData)

	for _, key := range keys {
		// Extract project node sequence from key (fixed extraction logic)
		// Key format: sample_data:samplerName:projectNodeSequence
		prefix := fmt.Sprintf("%s%s:", RedisSampleKeyPrefix, samplerName)
		projectNodeSequence := key[len(prefix):]

		// Get samples from sorted set (latest first)
		samples, err := rsm.getSamplesFromKey(ctx, key)
		if err != nil {
			continue // Skip this key if error
		}

		if len(samples) > 0 {
			result[projectNodeSequence] = samples
		}
	}

	return result, nil
}

// getSamplesFromKey retrieves samples from a specific Redis key
func (rsm *RedisSampleManager) getSamplesFromKey(ctx context.Context, key string) ([]SampleData, error) {
	// Get all samples from sorted set (latest first)
	members, err := rdb.ZRevRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get samples from key %s: %w", key, err)
	}

	samples := make([]SampleData, 0, len(members))

	for _, member := range members {
		var redisSample RedisSampleData
		err := json.Unmarshal([]byte(member), &redisSample)
		if err != nil {
			continue // Skip invalid data
		}

		sample := SampleData{
			Data:                redisSample.Data,
			Timestamp:           redisSample.Timestamp,
			ProjectNodeSequence: redisSample.ProjectNodeSequence,
		}
		samples = append(samples, sample)
	}

	return samples, nil
}

// GetStats retrieves statistics for a sampler
func (rsm *RedisSampleManager) GetStats(samplerName string) (map[string]int64, error) {
	if rdb == nil {
		return nil, fmt.Errorf("Redis client not available")
	}

	ctx := context.Background()
	pattern := fmt.Sprintf("%s%s:*", RedisSampleCountKey, samplerName)

	// Get all count keys
	keys, err := rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get count keys: %w", err)
	}

	result := make(map[string]int64)

	for _, key := range keys {
		// Extract project node sequence from key
		projectNodeSequence := key[len(fmt.Sprintf("%s%s:", RedisSampleCountKey, samplerName)):]

		// Get count
		count, err := rdb.Get(ctx, key).Int64()
		if err != nil {
			continue // Skip this key if error
		}

		result[projectNodeSequence] = count
	}

	return result, nil
}

// Reset clears all samples for a sampler
func (rsm *RedisSampleManager) Reset(samplerName string) error {
	if rdb == nil {
		return fmt.Errorf("Redis client not available")
	}

	ctx := context.Background()

	// Delete all sample data keys
	pattern := fmt.Sprintf("%s%s:*", RedisSampleKeyPrefix, samplerName)
	keys, err := rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get sample keys: %w", err)
	}

	if len(keys) > 0 {
		err = rdb.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("failed to delete sample keys: %w", err)
		}
	}

	// Delete all count keys
	pattern = fmt.Sprintf("%s%s:*", RedisSampleCountKey, samplerName)
	keys, err = rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get count keys: %w", err)
	}

	if len(keys) > 0 {
		err = rdb.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("failed to delete count keys: %w", err)
		}
	}

	return nil
}

// SetTTL sets the TTL for sample data
func (rsm *RedisSampleManager) SetTTL(ttl time.Duration) {
	rsm.ttl = ttl
}

// SetMaxSamplesPerKey sets the maximum number of samples per key
func (rsm *RedisSampleManager) SetMaxSamplesPerKey(max int) {
	rsm.maxSamplesPerKey = max
}

// startCleanup starts the cleanup routine
func (rsm *RedisSampleManager) startCleanup() {
	for {
		select {
		case <-rsm.cleanupTicker.C:
			rsm.cleanupExpiredData()
		case <-rsm.stopChan:
			return
		}
	}
}

// cleanupExpiredData removes expired data
func (rsm *RedisSampleManager) cleanupExpiredData() {
	if rdb == nil {
		return
	}

	ctx := context.Background()

	// Get all sample keys
	pattern := fmt.Sprintf("%s*", RedisSampleKeyPrefix)
	keys, err := rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return
	}

	// Remove expired samples based on timestamp
	cutoffTime := time.Now().Add(-rsm.ttl)
	cutoffScore := float64(cutoffTime.Unix())

	for _, key := range keys {
		// Remove samples older than TTL
		rdb.ZRemRangeByScore(ctx, key, "0", strconv.FormatFloat(cutoffScore, 'f', -1, 64))
	}
}

// Close stops the cleanup routine
func (rsm *RedisSampleManager) Close() {
	if rsm.cleanupTicker != nil {
		rsm.cleanupTicker.Stop()
	}
	close(rsm.stopChan)
}

// startBatchProcessor processes samples in batches to improve Redis performance
func (rsm *RedisSampleManager) startBatchProcessor() {
	batch := make([]SampleData, 0, 200) // Batch size of 200

	for {
		select {
		case sample, ok := <-rsm.batchChannel:
			if !ok {
				// Channel closed, process remaining batch and exit
				if len(batch) > 0 {
					rsm.processBatch(batch)
				}
				return
			}

			batch = append(batch, sample)

			// Process batch when it reaches capacity
			if len(batch) >= 200 {
				rsm.processBatch(batch)
				batch = batch[:0] // Reset batch
			}

		case <-rsm.batchTicker.C:
			// Process batch periodically even if not full
			if len(batch) > 0 {
				rsm.processBatch(batch)
				batch = batch[:0] // Reset batch
			}

		case <-rsm.stopChan:
			// Process remaining batch and exit
			if len(batch) > 0 {
				rsm.processBatch(batch)
			}
			return
		}
	}
}

// processBatch processes a batch of samples efficiently using Redis pipeline
func (rsm *RedisSampleManager) processBatch(batch []SampleData) {
	if rdb == nil || len(batch) == 0 {
		return
	}

	ctx := context.Background()
	pipe := rdb.TxPipeline()

	// Group samples by key to optimize Redis operations
	samplesByKey := make(map[string][]SampleData)
	for _, sample := range batch {
		key := fmt.Sprintf("%s%s:%s", RedisSampleKeyPrefix, sample.ProjectNodeSequence, "unknown") // We need sampler name
		samplesByKey[key] = append(samplesByKey[key], sample)
	}

	// Process each key group
	for key, samples := range samplesByKey {
		for _, sample := range samples {
			// Create Redis sample data
			redisSample := RedisSampleData{
				Data:                sample.Data,
				Timestamp:           sample.Timestamp,
				ProjectNodeSequence: sample.ProjectNodeSequence,
				SamplerName:         "unknown", // We need to pass sampler name
				Score:               float64(sample.Timestamp.Unix()),
			}

			// Serialize to JSON
			jsonData, err := json.Marshal(redisSample)
			if err != nil {
				continue
			}

			// Add to sorted set (sorted by timestamp)
			pipe.ZAdd(ctx, key, redis.Z{
				Score:  redisSample.Score,
				Member: jsonData,
			})

			// Set TTL on the key
			pipe.Expire(ctx, key, rsm.ttl)

			// Keep only the most recent N samples
			pipe.ZRemRangeByRank(ctx, key, 0, -int64(rsm.maxSamplesPerKey+1))
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		// Log error but don't fail completely
		fmt.Printf("Failed to execute batch Redis operations: %v\n", err)
	}
}

// StoreSampleAsync stores a sample asynchronously via batch processing
func (rsm *RedisSampleManager) StoreSampleAsync(samplerName string, sample SampleData) error {
	if rsm.batchChannel == nil {
		// Fall back to synchronous storage
		return rsm.StoreSample(samplerName, sample)
	}

	// Add sampler name to sample for batch processing
	// We need to extend SampleData to include sampler name, or modify the approach

	// For now, send to batch channel (non-blocking)
	select {
	case rsm.batchChannel <- sample:
		return nil
	default:
		// Channel full, fall back to synchronous storage
		return rsm.StoreSample(samplerName, sample)
	}
}

// Global Redis sample manager instance
var globalRedisSampleManager *RedisSampleManager

// InitRedisSampleManager initializes the global Redis sample manager
func InitRedisSampleManager() {
	globalRedisSampleManager = NewRedisSampleManager()
}

// GetRedisSampleManager returns the global Redis sample manager
func GetRedisSampleManager() *RedisSampleManager {
	return globalRedisSampleManager
}
