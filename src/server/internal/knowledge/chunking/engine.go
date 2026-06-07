package chunking

import (
	"strings"
	"unicode"

	"oblivious/server/internal/knowledge"
)

const (
	defaultChunkSize = 500
	defaultOverlap   = 50
	minChunkSize     = 100
	maxChunkSize     = 4000
)

// Engine splits document text into chunks according to the configured strategy.
// It is safe for concurrent use (no mutable state).
type Engine struct{}

// NewEngine returns a new chunking Engine.
func NewEngine() *Engine { return &Engine{}}

// Chunk splits content into EngineDocumentChunks preserving page metadata
// when available. The pages slice may be nil for single-page documents.
func (e *Engine) Chunk(content string, pages []knowledge.DocumentPage, config knowledge.ChunkingEngineConfig) []knowledge.EngineDocumentChunk {
	config = normalizeConfig(config)
	switch config.Strategy {
	case knowledge.ChunkingEngineStrategyFixedSize:
		return e.fixedSize(content, pages, config.ChunkSize, config.Overlap)
	case knowledge.ChunkingEngineStrategySemantic:
		return e.semantic(content, pages, config.ChunkSize)
	case knowledge.ChunkingEngineStrategyQASplit:
		return e.qaSplit(content, pages, config.ChunkSize)
	case knowledge.ChunkingEngineStrategyTemplate:
		return e.template(content, pages, config.ChunkSize)
	default:
		return e.fixedSize(content, pages, config.ChunkSize, config.Overlap)
	}
}

func normalizeConfig(cfg knowledge.ChunkingEngineConfig) knowledge.ChunkingEngineConfig {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = defaultChunkSize
	}
	if cfg.ChunkSize < minChunkSize {
		cfg.ChunkSize = minChunkSize
	}
	if cfg.ChunkSize > maxChunkSize {
		cfg.ChunkSize = maxChunkSize
	}
	if cfg.Overlap < 0 {
		cfg.Overlap = 0
	}
	if cfg.Overlap >= cfg.ChunkSize {
		cfg.Overlap = cfg.ChunkSize - 1
	}
	if cfg.Overlap == 0 && cfg.ChunkSize == defaultChunkSize {
		cfg.Overlap = defaultOverlap
	}
	cfg.Strategy = strings.TrimSpace(cfg.Strategy)
	return cfg
}

// ---------------------------------------------------------------------------
// fixed_size: sliding window with overlap
// ---------------------------------------------------------------------------

func (e *Engine) fixedSize(content string, pages []knowledge.DocumentPage, chunkSize, overlap int) []knowledge.EngineDocumentChunk {
	runes := []rune(strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n")))
	if len(runes) == 0 {
		return nil
	}
	pageMap := buildPageMap(pages)

	chunks := make([]knowledge.EngineDocumentChunk, 0, len(runes)/chunkSize+1)
	idx := 0
	for start := 0; start < len(runes); {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		text := strings.TrimSpace(string(runes[start:end]))
		if text != "" {
			chunks = append(chunks, knowledge.EngineDocumentChunk{
				Index:      idx,
				Content:    text,
				PageNumber: lookupPage(pageMap, start),
				StartRune:  start,
				EndRune:    end,
			})
			idx++
		}
		if end == len(runes) {
			break
		}
		start = end - overlap
	}
	return chunks
}

// ---------------------------------------------------------------------------
// semantic: split by paragraphs/sections/headings
// ---------------------------------------------------------------------------

func (e *Engine) semantic(content string, pages []knowledge.DocumentPage, maxChunkSize int) []knowledge.EngineDocumentChunk {
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if normalized == "" {
		return nil
	}
	pageMap := buildPageMap(pages)

	paragraphs := splitSemanticParagraphs(normalized)
	var chunks []knowledge.EngineDocumentChunk
	idx := 0
	runeOffset := 0

	for _, para := range paragraphs {
		cleaned := strings.Join(strings.Fields(para), " ")
		if cleaned == "" {
			runeOffset += len([]rune(para)) + 2
			continue
		}
		subChunks := splitText(cleaned, maxChunkSize)
		for _, sc := range subChunks {
			if sc == "" {
				continue
			}
			chunks = append(chunks, knowledge.EngineDocumentChunk{
				Index:      idx,
				Content:    sc,
				PageNumber: lookupPage(pageMap, runeOffset),
				StartRune:  runeOffset,
				EndRune:    runeOffset + len([]rune(sc)),
			})
			idx++
		}
		runeOffset += len([]rune(para)) + 2
	}
	return chunks
}

func splitSemanticParagraphs(text string) []string {
	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) == 1 {
		paragraphs = strings.Split(text, "\n")
	}
	return paragraphs
}

// ---------------------------------------------------------------------------
// qa_split: group Q:/A: pairs into single chunks
// ---------------------------------------------------------------------------

func (e *Engine) qaSplit(content string, pages []knowledge.DocumentPage, maxChunkSize int) []knowledge.EngineDocumentChunk {
	semanticChunks := e.semantic(content, pages, maxChunkSize)
	if len(semanticChunks) == 0 {
		return nil
	}

	var result []knowledge.EngineDocumentChunk
	var group []knowledge.EngineDocumentChunk
	flush := func() {
		if len(group) == 0 {
			return
		}
		texts := make([]string, len(group))
		for i, c := range group {
			texts[i] = c.Content
		}
		result = append(result, knowledge.EngineDocumentChunk{
			Index:      len(result),
			Content:    strings.Join(texts, " "),
			PageNumber: group[0].PageNumber,
			StartRune:  group[0].StartRune,
			EndRune:    group[len(group)-1].EndRune,
		})
		group = nil
	}

	for _, chunk := range semanticChunks {
		lower := strings.TrimSpace(strings.ToLower(chunk.Content))
		if strings.HasPrefix(lower, "q:") {
			flush()
		}
		group = append(group, chunk)
		if strings.HasPrefix(lower, "a:") {
			flush()
		}
	}
	flush()
	return result
}

// ---------------------------------------------------------------------------
// template: ragflow-style structured chunking (split on headings)
// ---------------------------------------------------------------------------

func (e *Engine) template(content string, pages []knowledge.DocumentPage, maxChunkSize int) []knowledge.EngineDocumentChunk {
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if normalized == "" {
		return nil
	}
	pageMap := buildPageMap(pages)

	sections := splitByHeadings(normalized)
	var chunks []knowledge.EngineDocumentChunk
	idx := 0
	runeOffset := 0

	for _, section := range sections {
		text := strings.TrimSpace(section)
		if text == "" {
			runeOffset += len([]rune(section)) + 1
			continue
		}
		runeLen := len([]rune(text))
		if runeLen <= maxChunkSize {
			chunks = append(chunks, knowledge.EngineDocumentChunk{
				Index:      idx,
				Content:    text,
				PageNumber: lookupPage(pageMap, runeOffset),
				StartRune:  runeOffset,
				EndRune:    runeOffset + runeLen,
			})
			idx++
		} else {
			paras := splitSemanticParagraphs(text)
			for _, para := range paras {
				para = strings.TrimSpace(para)
				if para == "" {
					runeOffset++
					continue
				}
				for _, sc := range splitText(para, maxChunkSize) {
					if sc == "" {
						continue
					}
					chunks = append(chunks, knowledge.EngineDocumentChunk{
						Index:      idx,
						Content:    sc,
						PageNumber: lookupPage(pageMap, runeOffset),
						StartRune:  runeOffset,
						EndRune:    runeOffset + len([]rune(sc)),
					})
					idx++
				}
				runeOffset += len([]rune(para)) + 1
			}
		}
		runeOffset += len([]rune(section)) + 1
	}
	return chunks
}

func splitByHeadings(text string) []string {
	lines := strings.Split(text, "\n")
	var sections []string
	var current strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && len(trimmed) > 1 {
			if current.Len() > 0 {
				sections = append(sections, current.String())
				current.Reset()
			}
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		sections = append(sections, current.String())
	}
	return sections
}

// ---------------------------------------------------------------------------
// Shared utilities
// ---------------------------------------------------------------------------

func splitText(content string, maxLen int) []string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil
	}
	if len(runes) <= maxLen {
		return []string{string(runes)}
	}
	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + maxLen
		if end >= len(runes) {
			chunks = append(chunks, strings.TrimSpace(string(runes[start:])))
			break
		}
		splitAt := end
		for splitAt > start && !unicode.IsSpace(runes[splitAt-1]) {
			splitAt--
		}
		if splitAt == start {
			splitAt = end
		}
		chunks = append(chunks, strings.TrimSpace(string(runes[start:splitAt])))
		start = splitAt
		for start < len(runes) && unicode.IsSpace(runes[start]) {
			start++
		}
	}
	return chunks
}

type pageMapEntry struct {
	startRune int
	pageNum   int
}

func buildPageMap(pages []knowledge.DocumentPage) []pageMapEntry {
	if len(pages) == 0 {
		return nil
	}
	entries := make([]pageMapEntry, 0, len(pages))
	offset := 0
	for _, p := range pages {
		entries = append(entries, pageMapEntry{startRune: offset, pageNum: p.PageNumber})
		offset += len([]rune(p.Content)) + 1
	}
	return entries
}

func lookupPage(entries []pageMapEntry, offset int) int {
	if len(entries) == 0 {
		return 0
	}
	page := entries[0].pageNum
	for _, e := range entries {
		if offset >= e.startRune {
			page = e.pageNum
		}
	}
	return page
}
