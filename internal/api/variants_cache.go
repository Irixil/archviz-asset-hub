package api

import (
	"container/list"
	"sync"
)

type previewCacheEntry struct {
	key         string
	data        []byte
	contentType string
}

type LruPreviewCache struct {
	mu      sync.Mutex
	items   map[string]*list.Element
	order   *list.List
	maxSize int
}

func NewLRUPreviewCache(maxSize int) *LruPreviewCache {
	return &LruPreviewCache{
		items:   make(map[string]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
	}
}

func (c *LruPreviewCache) Get(key string) ([]byte, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, ""
	}
	c.order.MoveToFront(el)
	entry, ok := el.Value.(*previewCacheEntry)
	if !ok {
		return nil, ""
	}
	return entry.data, entry.contentType
}

func (c *LruPreviewCache) Set(key string, data []byte, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		entry, entryOk := el.Value.(*previewCacheEntry)
		if !entryOk {
			return
		}
		entry.data = data
		return
	}
	entry := &previewCacheEntry{key: key, data: data, contentType: contentType}
	el := c.order.PushFront(entry)
	c.items[key] = el

	for c.order.Len() > c.maxSize {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.order.Remove(back)
		backEntry, entryOk := back.Value.(*previewCacheEntry)
		if !entryOk {
			continue
		}
		delete(c.items, backEntry.key)
	}
}

// fiber:context-methods migrated
