package document

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type fileStore struct {
	root string
}

func newFileStore(root string) (*fileStore, error) {
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("resolve library root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(absRoot, "notes"), 0o700); err != nil {
		return nil, fmt.Errorf("create library notes directory: %w", err)
	}
	return &fileStore{root: absRoot}, nil
}

func (s *fileStore) contentPath(categoryPath, slug string) (string, error) {
	if !isSafePathSlug(slug) {
		return "", fmt.Errorf("invalid slug")
	}
	parts := []string{"notes"}
	if strings.TrimSpace(categoryPath) != "" {
		for _, part := range strings.Split(categoryPath, "/") {
			if !isSafePathSlug(part) {
				return "", fmt.Errorf("invalid category path")
			}
			parts = append(parts, part)
		}
	}
	parts = append(parts, slug+".md")
	return filepath.ToSlash(filepath.Join(parts...)), nil
}

func (s *fileStore) writeDocument(path string, doc Document, categoryPath string, tagNames []string, content string) error {
	body := renderMarkdown(doc, categoryPath, tagNames, content)
	return s.writeBytes(path, []byte(body))
}

func (s *fileStore) writeBytes(path string, data []byte) error {
	absPath, err := s.absolutePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return fmt.Errorf("create document directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(absPath), ".tmp-*.md")
	if err != nil {
		return fmt.Errorf("create document temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write document temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close document temp file: %w", err)
	}
	var backupPath string
	if _, err := os.Stat(absPath); err == nil {
		backupPath, err = tempSiblingPath(filepath.Dir(absPath), ".replace-*.md")
		if err != nil {
			return err
		}
		if err := os.Rename(absPath, backupPath); err != nil {
			return fmt.Errorf("stage existing document file: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat document file: %w", err)
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, absPath)
		}
		return fmt.Errorf("replace document file: %w", err)
	}
	cleanup = false
	if backupPath != "" {
		_ = os.Remove(backupPath)
	}
	return nil
}

func (s *fileStore) readContent(path string) (string, []byte, error) {
	absPath, err := s.absolutePath(path)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", nil, fmt.Errorf("read document file: %w", err)
	}
	return stripFrontmatter(string(data)), data, nil
}

func (s *fileStore) exists(path string) (bool, error) {
	absPath, err := s.absolutePath(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(absPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat document file: %w", err)
}

func (s *fileStore) remove(path string) error {
	absPath, err := s.absolutePath(path)
	if err != nil {
		return err
	}
	if err := os.Remove(absPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove document file: %w", err)
	}
	return nil
}

// stageDelete moves a document file aside and returns restore/finalize callbacks.
// stageDelete 先暂存文档文件，并返回恢复/最终清理回调。
func (s *fileStore) stageDelete(path string) (func() error, func() error, error) {
	absPath, err := s.absolutePath(path)
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(absPath); errors.Is(err, fs.ErrNotExist) {
		return noopFileOp, noopFileOp, nil
	} else if err != nil {
		return nil, nil, fmt.Errorf("stat document file: %w", err)
	}
	stagedPath, err := tempSiblingPath(filepath.Dir(absPath), ".delete-*.md")
	if err != nil {
		return nil, nil, err
	}
	if err := os.Rename(absPath, stagedPath); err != nil {
		return nil, nil, fmt.Errorf("stage document file deletion: %w", err)
	}
	restore := func() error {
		if _, err := os.Stat(stagedPath); errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return fmt.Errorf("stat staged document file: %w", err)
		}
		if _, err := os.Stat(absPath); err == nil {
			return fmt.Errorf("restore document file: target already exists")
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("stat restore target: %w", err)
		}
		if err := os.Rename(stagedPath, absPath); err != nil {
			return fmt.Errorf("restore document file: %w", err)
		}
		return nil
	}
	finalize := func() error {
		if err := os.Remove(stagedPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove staged document file: %w", err)
		}
		return nil
	}
	return restore, finalize, nil
}

func (s *fileStore) absolutePath(path string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", fmt.Errorf("unsafe document path")
	}
	absPath := filepath.Join(s.root, cleaned)
	rel, err := filepath.Rel(s.root, absPath)
	if err != nil {
		return "", fmt.Errorf("resolve document path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("document path escapes library root")
	}
	return absPath, nil
}

func tempSiblingPath(dir, pattern string) (string, error) {
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("create sibling temp file: %w", err)
	}
	path := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close sibling temp file: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("remove sibling temp placeholder: %w", err)
	}
	return path, nil
}

func noopFileOp() error {
	return nil
}

func renderMarkdown(doc Document, categoryPath string, tagNames []string, content string) string {
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString("title: " + yamlQuote(doc.Title) + "\n")
	builder.WriteString("created: " + yamlQuote(doc.CreatedAt.Format(time.RFC3339Nano)) + "\n")
	builder.WriteString("updated: " + yamlQuote(doc.UpdatedAt.Format(time.RFC3339Nano)) + "\n")
	builder.WriteString("tags: " + yamlList(tagNames) + "\n")
	builder.WriteString("category: " + yamlQuote(categoryPath) + "\n")
	builder.WriteString("source: " + yamlQuote(doc.Source) + "\n")
	builder.WriteString("summary: " + yamlQuote(doc.Summary) + "\n")
	builder.WriteString("confidence: " + strconv.FormatFloat(doc.Confidence, 'f', -1, 64) + "\n")
	builder.WriteString("status: " + yamlQuote(doc.Status) + "\n")
	if doc.CoverURL != "" {
		builder.WriteString("cover_url: " + yamlQuote(doc.CoverURL) + "\n")
	}
	if doc.PublishedAt != nil {
		builder.WriteString("published: " + yamlQuote(doc.PublishedAt.Format(time.RFC3339Nano)) + "\n")
	}
	builder.WriteString("---\n\n")
	builder.WriteString(strings.TrimLeft(content, "\r\n"))
	return builder.String()
}

func stripFrontmatter(markdown string) string {
	markdown = strings.TrimPrefix(markdown, "\uFEFF")
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return markdown
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return markdown
	}
	return strings.TrimLeft(normalized[4+end+5:], "\n")
}

func yamlQuote(value string) string {
	return strconv.Quote(value)
}

func yamlList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, yamlQuote(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func isSafePathSlug(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}
