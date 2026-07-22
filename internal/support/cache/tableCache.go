package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"errors"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
)

const tableCacheTTL = 1 * time.Hour 

const (
	keyTableByID = "table:id:%d" 
	keyTableByToken = "table:token:%s"
)

type TableCache struct {
	rdb *config.RedisClient
}

func NewTableCache(rdb *config.RedisClient) *TableCache {
	return &TableCache{rdb: rdb}
}

// get by id
func (c *TableCache) GetByID(ctx context.Context, id int64) (*entity.Table, error){
	key := fmt.Sprintf(keyTableByID, id)

	raw, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, config.RedisNil){
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("cache get by id: %w", err)
	}

	var t entity.Table
	if err := json.Unmarshal(raw, &t); err != nil {
		_ = c.rdb.Del(ctx, key).Err()
		return nil, nil
	}

	return &t, nil
}

func (c *TableCache) SetByID(ctx context.Context, t *entity.Table) error {
	key := fmt.Sprintf(keyTableByID, t.ID)
	payload, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	return c.rdb.Set(ctx, key, payload, tableCacheTTL).Err()
}


