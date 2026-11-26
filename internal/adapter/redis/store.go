package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	indexPrefix = "search:index:"
	cachePrefix = "search:cache:"
	cacheTTL    = time.Hour
)

type Store struct {
	client *goredis.Client
}

func Open(ctx context.Context, addr string) (*Store, error) {
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return &Store{client: client}, nil
}

func (s *Store) Close() error {
	return s.client.Close()
}

// ReplaceIndex clears the old index and response cache, then writes the new index.
func (s *Store) ReplaceIndex(ctx context.Context, index map[string][]int) error {
	if err := s.deleteByPattern(ctx, indexPrefix+"*"); err != nil {
		return err
	}
	if err := s.deleteByPattern(ctx, cachePrefix+"*"); err != nil {
		return err
	}

	if len(index) == 0 {
		return nil
	}

	pipe := s.client.Pipeline()
	for word, ids := range index {
		members := make([]interface{}, len(ids))
		for i, id := range ids {
			members[i] = id
		}
		pipe.SAdd(ctx, indexPrefix+word, members...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("write search index: %w", err)
	}
	return nil
}

func (s *Store) FindIDs(ctx context.Context, words []string) ([]int, error) {
	if len(words) == 0 {
		return []int{}, nil
	}

	keys := make([]string, len(words))
	for i, word := range words {
		keys[i] = indexPrefix + word
	}

	var values []string
	var err error
	if len(keys) == 1 {
		values, err = s.client.SMembers(ctx, keys[0]).Result()
	} else {
		values, err = s.client.SInter(ctx, keys...).Result()
	}
	if err != nil {
		return nil, fmt.Errorf("read search index: %w", err)
	}

	ids := make([]int, 0, len(values))
	for _, value := range values {
		id, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid movie id %q in search index: %w", value, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Store) GetCached(ctx context.Context, query string) (string, bool, error) {
	value, err := s.client.Get(ctx, cachePrefix+query).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read search cache: %w", err)
	}
	return value, true, nil
}

func (s *Store) SetCached(ctx context.Context, query, value string) error {
	if err := s.client.Set(ctx, cachePrefix+query, value, cacheTTL).Err(); err != nil {
		return fmt.Errorf("write search cache: %w", err)
	}
	return nil
}

func (s *Store) deleteByPattern(ctx context.Context, pattern string) error {
	keys := make([]string, 0)
	iter := s.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan Redis keys %q: %w", pattern, err)
	}
	if len(keys) == 0 {
		return nil
	}
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("delete Redis keys %q: %w", pattern, err)
	}
	return nil
}
