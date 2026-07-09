package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

// Document 索引文档结构
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	Source    string    `json:"source,omitempty"`
	Path      string    `json:"path,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// SearchResult 搜索结果条目
type SearchResult struct {
	ID      string   `json:"id"`
	Score   float64  `json:"score"`
	Title   string   `json:"title,omitempty"`
	Content string   `json:"content,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Source  string   `json:"source,omitempty"`
	Path    string   `json:"path,omitempty"`
}

// SearchResponse 搜索结果响应
type SearchResponse struct {
	Total uint64         `json:"total"` // 匹配总数
	Took  time.Duration  `json:"took"`  // 搜索耗时
	Hits  []SearchResult `json:"hits"`  // 命中结果列表
}

// Engine 全文检索引擎
type Engine struct {
	mu    sync.RWMutex
	index bleve.Index
	path  string
}

// NewEngine 创建基于磁盘的检索引擎
func NewEngine(dataDir string) (*Engine, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("search: create data dir failed: %w", err)
	}

	indexPath := filepath.Join(dataDir, "search.bleve")
	index, err := openOrCreateIndex(indexPath)
	if err != nil {
		return nil, fmt.Errorf("search: init index failed: %w", err)
	}

	return &Engine{index: index, path: indexPath}, nil
}

// NewMemoryEngine 创建纯内存检索引擎
func NewMemoryEngine() (*Engine, error) {
	mapping := buildMapping()
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, fmt.Errorf("search: create memory index failed: %w", err)
	}
	return &Engine{index: index, path: ":memory:"}, nil
}

// openOrCreateIndex 打开已有索引或创建新索引
func openOrCreateIndex(path string) (bleve.Index, error) {
	if _, err := os.Stat(path); err == nil {
		index, err := bleve.Open(path)
		if err != nil {
			return nil, fmt.Errorf("search: open index %s failed: %w", path, err)
		}
		return index, nil
	}
	mapping := buildMapping()
	index, err := bleve.New(path, mapping)
	if err != nil {
		return nil, fmt.Errorf("search: create index %s failed: %w", path, err)
	}
	return index, nil
}

// buildMapping 构建索引字段映射配置
func buildMapping() *mapping.IndexMappingImpl {
	docMapping := bleve.NewDocumentMapping()

	textMapping := bleve.NewTextFieldMapping()

	keywordMapping := bleve.NewTextFieldMapping()
	keywordMapping.Analyzer = "keyword"

	docMapping.AddFieldMappingsAt("title", textMapping)
	docMapping.AddFieldMappingsAt("content", textMapping)
	docMapping.AddFieldMappingsAt("tags", keywordMapping)
	docMapping.AddFieldMappingsAt("source", keywordMapping)
	docMapping.AddFieldMappingsAt("path", keywordMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddDocumentMapping("doc", docMapping)
	indexMapping.TypeField = "type"
	indexMapping.DefaultAnalyzer = "standard"

	return indexMapping
}

// Index 索引单个文档
func (e *Engine) Index(ctx context.Context, doc Document) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = time.Now()
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}
	if doc.ID == "" {
		return fmt.Errorf("search: document ID is required")
	}
	return e.index.Index(doc.ID, doc)
}

// BatchIndex 批量索引文档
func (e *Engine) BatchIndex(ctx context.Context, docs []Document) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	batch := e.index.NewBatch()
	for _, doc := range docs {
		if doc.ID == "" {
			continue
		}
		if doc.UpdatedAt.IsZero() {
			doc.UpdatedAt = time.Now()
		}
		if doc.CreatedAt.IsZero() {
			doc.CreatedAt = time.Now()
		}
		if err := batch.Index(doc.ID, doc); err != nil {
			return fmt.Errorf("search: batch index failed for %s: %w", doc.ID, err)
		}
	}
	return e.index.Batch(batch)
}

// Delete 从索引中删除文档
func (e *Engine) Delete(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.index.Delete(id)
}

// Search 执行全文搜索
func (e *Engine) Search(ctx context.Context, q string, opts ...SearchOption) (*SearchResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var o searchOptions
	for _, opt := range opts {
		opt(&o)
	}

	searchQuery := buildQuery(q, o)
	searchRequest := bleve.NewSearchRequest(searchQuery)
	searchRequest.Size = o.size()
	searchRequest.From = o.from()
	searchRequest.Fields = o.fields
	if o.highlight {
		searchRequest.Highlight = bleve.NewHighlight()
	}

	start := time.Now()
	result, err := e.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search: query failed: %w", err)
	}

	hits := make([]SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		sr := SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}
		if fields, ok := hit.Fields["title"]; ok {
			sr.Title = fmt.Sprintf("%v", fields)
		}
		if fields, ok := hit.Fields["content"]; ok {
			sr.Content = fmt.Sprintf("%v", fields)
		}
		if fields, ok := hit.Fields["source"]; ok {
			sr.Source = fmt.Sprintf("%v", fields)
		}
		if fields, ok := hit.Fields["path"]; ok {
			sr.Path = fmt.Sprintf("%v", fields)
		}
		if fields, ok := hit.Fields["tags"]; ok {
			if tagList, ok := fields.([]any); ok {
				for _, t := range tagList {
					sr.Tags = append(sr.Tags, fmt.Sprintf("%v", t))
				}
			}
		}
		hits = append(hits, sr)
	}

	return &SearchResponse{
		Total: result.Total,
		Took:  time.Since(start),
		Hits:  hits,
	}, nil
}

// Count 获取索引文档总数
func (e *Engine) Count(ctx context.Context) (uint64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	count, err := e.index.DocCount()
	if err != nil {
		return 0, fmt.Errorf("search: count failed: %w", err)
	}
	return count, nil
}

// Close 关闭检索引擎
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.index.Close()
}

// searchOptions 搜索选项配置
type searchOptions struct {
	limit     int      // 返回条数上限
	offset    int      // 分页偏移量
	fields    []string // 返回字段列表
	highlight bool     // 是否启用高亮
	tags      []string // 标签过滤
	tagsOr    bool     // 标签逻辑 OR
	source    string   // 来源过滤
}

func (o searchOptions) size() int {
	if o.limit <= 0 {
		return 10
	}
	return o.limit
}

func (o searchOptions) from() int {
	if o.offset < 0 {
		return 0
	}
	return o.offset
}

// SearchOption 搜索选项函数类型
type SearchOption func(*searchOptions)

// WithLimit 设置返回结果数量上限
func WithLimit(limit int) SearchOption {
	return func(o *searchOptions) { o.limit = limit }
}

// WithOffset 设置分页偏移量
func WithOffset(offset int) SearchOption {
	return func(o *searchOptions) { o.offset = offset }
}

// WithFields 设置返回字段白名单
func WithFields(fields ...string) SearchOption {
	return func(o *searchOptions) { o.fields = fields }
}

// WithHighlight 启用搜索结果高亮
func WithHighlight() SearchOption {
	return func(o *searchOptions) { o.highlight = true }
}

// WithTags 按标签过滤（AND 逻辑）
func WithTags(tags ...string) SearchOption {
	return func(o *searchOptions) { o.tags = tags; o.tagsOr = false }
}

// WithTagsOR 按标签过滤（OR 逻辑）
func WithTagsOR(tags ...string) SearchOption {
	return func(o *searchOptions) { o.tags = tags; o.tagsOr = true }
}

// WithSource 按来源过滤
func WithSource(source string) SearchOption {
	return func(o *searchOptions) { o.source = source }
}

// buildQuery 根据查询字符串和选项构建 bleve 查询
func buildQuery(q string, o searchOptions) query.Query {
	var queries []query.Query

	if q != "" {
		fuzzyQ := bleve.NewFuzzyQuery(q)
		fuzzyQ.Fuzziness = 2
		queries = append(queries, fuzzyQ)
	}

	if o.source != "" {
		sourceQ := bleve.NewTermQuery(o.source)
		sourceQ.SetField("source")
		queries = append(queries, sourceQ)
	}

	if len(o.tags) > 0 {
		tagQs := make([]query.Query, 0, len(o.tags))
		for _, tag := range o.tags {
			tagQ := bleve.NewTermQuery(tag)
			tagQ.SetField("tags")
			tagQs = append(tagQs, tagQ)
		}
		if o.tagsOr && len(tagQs) > 0 {
			queries = append(queries, bleve.NewDisjunctionQuery(tagQs...))
		} else {
			queries = append(queries, tagQs...)
		}
	}

	if len(queries) == 0 {
		return bleve.NewMatchAllQuery()
	}
	if len(queries) == 1 {
		return queries[0]
	}
	return bleve.NewConjunctionQuery(queries...)
}