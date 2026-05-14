package memory

import (
	"strings"
	"unicode"
)

// TextChunker 文本分块器
type TextChunker struct {
	chunkSize    int // 每个分块的最大字符数
	chunkOverlap int // 分块之间的重叠字符数
}

// ChunkerConfig 分块配置
type ChunkerConfig struct {
	ChunkSize    int
	ChunkOverlap int
}

// NewTextChunker 创建文本分块器
func NewTextChunker(config ChunkerConfig) *TextChunker {
	chunkSize := config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 512 // 默认 512 字符
	}

	chunkOverlap := config.ChunkOverlap
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 4 // 重叠不超过分块大小的 1/4
	}

	return &TextChunker{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// Chunk 将文本分割成多个块
func (c *TextChunker) Chunk(text string) []string {
	// 标准化文本
	normalized := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if normalized == "" {
		return nil
	}

	// 先按段落分割
	paragraphs := strings.Split(normalized, "\n\n")

	var chunks []string
	var currentChunk strings.Builder
	currentLength := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// 如果段落本身超过分块大小，需要进一步分割
		if len(para) > c.chunkSize {
			// 先保存当前累积的内容
			if currentLength > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
				currentLength = 0
			}

			// 分割大段落
			subChunks := c.splitLongText(para)
			chunks = append(chunks, subChunks...)
			continue
		}

		// 检查是否需要开始新的分块
		if currentLength+len(para)+2 > c.chunkSize {
			if currentLength > 0 {
				chunks = append(chunks, currentChunk.String())
			}

			// 处理重叠
			if c.chunkOverlap > 0 && currentLength > c.chunkOverlap {
				overlap := c.getOverlap(currentChunk.String())
				currentChunk.Reset()
				currentChunk.WriteString(overlap)
				currentLength = len(overlap)
			} else {
				currentChunk.Reset()
				currentLength = 0
			}
		}

		if currentLength > 0 {
			currentChunk.WriteString("\n\n")
			currentLength += 2
		}
		currentChunk.WriteString(para)
		currentLength += len(para)
	}

	// 保存最后一个分块
	if currentLength > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// splitLongText 分割过长的文本
func (c *TextChunker) splitLongText(text string) []string {
	runes := []rune(text)
	if len(runes) <= c.chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(runes) {
		end := start + c.chunkSize
		if end >= len(runes) {
			chunks = append(chunks, strings.TrimSpace(string(runes[start:])))
			break
		}

		// 尝试在句子边界分割
		splitPos := c.findSentenceBoundary(runes, start, end)
		if splitPos <= start {
			splitPos = end
		}

		chunk := strings.TrimSpace(string(runes[start:splitPos]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		// 下一块的起始位置（考虑重叠）
		nextStart := splitPos - c.chunkOverlap
		if nextStart <= start {
			nextStart = splitPos
		}
		start = nextStart
	}

	return chunks
}

// findSentenceBoundary 在指定范围内查找句子边界
func (c *TextChunker) findSentenceBoundary(runes []rune, start, end int) int {
	// 从 end 向前查找句子结束符
	for i := end; i > start; i-- {
		r := runes[i-1]
		if r == '.' || r == '!' || r == '?' || r == '。' || r == '！' || r == '？' {
			// 检查后面是否是空格或换行
			if i < len(runes) && (unicode.IsSpace(runes[i]) || runes[i] == '\n') {
				return i
			}
			// 如果是句末
			if i == len(runes) {
				return i
			}
		}
	}

	// 没有找到句子边界，尝试在单词边界分割
	for i := end; i > start; i-- {
		if unicode.IsSpace(runes[i-1]) {
			return i
		}
	}

	return end
}

// getOverlap 获取分块末尾的重叠部分
func (c *TextChunker) getOverlap(text string) string {
	runes := []rune(text)
	if len(runes) <= c.chunkOverlap {
		return text
	}

	// 从末尾向前查找合适的分割点
	start := len(runes) - c.chunkOverlap
	for i := start; i < len(runes); i++ {
		if unicode.IsSpace(runes[i]) {
			start = i + 1
			break
		}
	}

	return string(runes[start:])
}

// DefaultChunker 默认分块器
func DefaultChunker() *TextChunker {
	return NewTextChunker(ChunkerConfig{
		ChunkSize:    512,
		ChunkOverlap: 64,
	})
}
