// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/nosql"

	"github.com/go-redis/redis/v7"
)

// RedisQueueType is the type for redis queue
const RedisQueueType Type = "redis"

// RedisQueueConfiguration is the configuration for the redis queue
type RedisQueueConfiguration struct {
	ByteFIFOQueueConfiguration
	RedisByteFIFOConfiguration
}

// RedisQueue redis queue
type RedisQueue struct {
	*ByteFIFOQueue
}

// NewRedisQueue creates single redis or cluster redis queue
func NewRedisQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(RedisQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(RedisQueueConfiguration)

	byteFIFO, err := NewRedisByteFIFO(config.RedisByteFIFOConfiguration)
	if err != nil {
		return nil, err
	}

	byteFIFOQueue, err := NewByteFIFOQueue(RedisQueueType, byteFIFO, handle, config.ByteFIFOQueueConfiguration, exemplar)
	if err != nil {
		return nil, err
	}

	queue := &RedisQueue{
		ByteFIFOQueue: byteFIFOQueue,
	}

	queue.qid = GetManager().Add(queue, RedisQueueType, config, exemplar)

	return queue, nil
}

type redisClient interface {
	RPush(key string, args ...interface{}) *redis.IntCmd
	LPop(key string) *redis.StringCmd
	LLen(key string) *redis.IntCmd
	SAdd(key string, members ...interface{}) *redis.IntCmd
	SRem(key string, members ...interface{}) *redis.IntCmd
	SIsMember(key string, member interface{}) *redis.BoolCmd
	Ping() *redis.StatusCmd
	Close() error
}

var _ (ByteFIFO) = &RedisByteFIFO{}

// RedisByteFIFO represents a ByteFIFO formed from a redisClient
type RedisByteFIFO struct {
	client    redisClient
	queueName string
}

// RedisByteFIFOConfiguration is the configuration for the RedisByteFIFO
type RedisByteFIFOConfiguration struct {
	ConnectionString string
	QueueName        string
}

// NewRedisByteFIFO creates a ByteFIFO formed from a redisClient
func NewRedisByteFIFO(config RedisByteFIFOConfiguration) (*RedisByteFIFO, error) {
	fifo := &RedisByteFIFO{
		queueName: config.QueueName,
	}
	fifo.client = nosql.GetManager().GetRedisClient(config.ConnectionString)
	if err := fifo.client.Ping().Err(); err != nil {
		return nil, err
	}
	return fifo, nil
}

// PushFunc pushes data to the end of the fifo and calls the callback if it is added
func (fifo *RedisByteFIFO) PushFunc(data []byte, fn func() error) error {
	if fn != nil {
		if err := fn(); err != nil {
			return err
		}
	}
	return fifo.client.RPush(fifo.queueName, data).Err()
}

// Pop pops data from the start of the fifo
func (fifo *RedisByteFIFO) Pop() ([]byte, error) {
	data, err := fifo.client.LPop(fifo.queueName).Bytes()
	if err == nil || err == redis.Nil {
		return data, nil
	}
	return data, err
}

// Close this fifo
func (fifo *RedisByteFIFO) Close() error {
	return fifo.client.Close()
}

// Len returns the length of the fifo
func (fifo *RedisByteFIFO) Len() int64 {
	val, err := fifo.client.LLen(fifo.queueName).Result()
	if err != nil {
		log.Error("Error whilst getting length of redis queue %s: Error: %v", fifo.queueName, err)
		return -1
	}
	return val
}

func init() {
	queuesMap[RedisQueueType] = NewRedisQueue
}
