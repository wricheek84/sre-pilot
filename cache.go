package main

import (
	
	"hash/fnv"
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

type ResilienceCache struct {
	lru       *lru.Cache[uint64, []float32]
	threshold float64
	mu        sync.RWMutex
}


func NewResilienceCache(size int) *ResilienceCache {
	c, _ := lru.New[uint64, []float32](size)
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


func (c *ResilienceCache) GetEmbedding(text string) ([]float32, bool) {
	sig := c.getFuzzyHash(text)
	
	if val, ok := c.lru.Get(sig); ok {
		return val, true 
	}
	
	return nil, false 
}


func (c *ResilienceCache) Add(text string, embedding []float32) {
	sig := c.getFuzzyHash(text)
	c.lru.Add(sig, embedding)
}