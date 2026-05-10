package cache

import (
	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type ProductCache struct {
	cache *config.RedisClient
}

const productCacheTTL = 1 * time.Hour

const (
	KeyProductID = "product:id:%d"
	// KeyProductToken = "product:token:%s"
)

func NewProductCache(cache *config.RedisClient) *ProductCache {
	return &ProductCache{cache: cache}
}

func (c *ProductCache) GetAll(ctx context.Context, filterstatus, categoryID string) ([]entity.Product, error) {
	key := fmt.Sprintf(KeyProductID, filterstatus, categoryID)

	raw, err := c.cache.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var product []entity.Product
	err = json.Unmarshal([]byte(raw), &product)
	if err != nil {
		return nil, fmt.Errorf("gagal unmarshall", err)
	}
	return product, nil
}

func (c *ProductCache) SetAll(ctx context.Context, filterstatus, categoryID string, product []entity.Product) error {
	key := fmt.Sprintf(KeyProductID, filterstatus, categoryID)
	productJSON, _ := json.Marshal(product)
	return c.cache.Set(ctx, key, productJSON, productCacheTTL).Err()
}

// func (c *ProductCache) GetByID(ctx context.Context, id int64) (*entity.Product, error){
// 	key := fmt.Sprintf(KeyProductID, id)

// 	raw, err := c.cache.Get(ctx, key).Bytes()
// 	if err != nil {
// 		return nil, err
// 	}

// 	var product entity.Product
// 	err = json.Unmarshal(raw, &product)
// 	if err != nil {
// 		return nil, fmt.Errorf("gagal unmarshal: %v", err)
// 	}
// 	return &product, nil
// }


// func (c *ProductCache) SetByID(ctx context.Context, product *entity.Product) error{
// 	key := fmt.Sprintf(KeyProductID, product.ID)
// 	productJSON, err := json.Marshal(product)
// 	if err != nil {
// 		return err
// 	}
// 	return c.cache.Set(ctx, key, productJSON, productCacheTTL).Err()
// }

func (c *ProductCache) InvaliDateByID(ctx context.Context, id int64) error {
	key := fmt.Sprintf(KeyProductID, id)
	return c.cache.Del(ctx, key).Err()
}

func (c *ProductCache) InvalidDateAll(ctx context.Context) error {
	iter := c.cache.Scan(ctx, 0, "product:status:*", 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		err := c.cache.Del(ctx, key).Err()
		if err != nil {
			log.Printf("gagal hapus cache dengan key %s: %v\n", key, err)
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}

	return nil
}
