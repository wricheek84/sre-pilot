package main

import (
	
	"hash/fnv"
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)
type spatialEmbedding struct {
	vector  []float32
	x, y, z float32
}

type ResilienceCache struct {
	lru       *lru.Cache[uint64, spatialEmbedding]
	threshold float64
	mu        sync.RWMutex
}


func NewResilienceCache(size int) *ResilienceCache {
	c, _ := lru.New[uint64, spatialEmbedding](size)
	return &ResilienceCache{
		lru:       c,
		threshold: 0.90, 
	}
}


func (c *ResilienceCache) getFuzzyHash(text string) uint64 {

	clean := strings.ToLower(text)
	words := strings.Fields(clean)
	var signature uint64
	
	
	h := fnv.New64a()
	for i := 0; i < len(words); i++ {
		
		isPod := strings.Contains(words[i], "pod-")
		isIP := strings.Contains(words[i], "127.0.0.1")
		isTime := strings.Contains(words[i], "time=")

		if !isPod && !isIP && !isTime {
			h.Write([]byte(words[i]))
		}
	}
	signature = h.Sum64()
	return signature
}


func (c *ResilienceCache) GetEmbedding(text string) ([]float32,float32,float32,float32, bool) {
	sig := c.getFuzzyHash(text)
	
	if val, ok := c.lru.Get(sig); ok {
		return val.vector,val.x,val.y,val.z, true 
	}
	
	return nil, 0, 0, 0, false 
}


func (c *ResilienceCache) Add(text string, embedding []float32,x, y, z float32) {
	sig := c.getFuzzyHash(text)
	c.lru.Add(sig, spatialEmbedding{
		vector: embedding,
		x:      x,
		y:      y,
		z:      z,		
	})
}