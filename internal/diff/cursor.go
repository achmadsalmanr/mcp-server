package diff

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type Cursor struct {
	FileIndex  int    `json:"file_index"`
	HunkIndex  int    `json:"hunk_index"`
	LineIndex  int    `json:"line_index"`
	FilterHash string `json:"filter_hash"`
}

type DiffPage struct {
	Files            []FileDiffPage `json:"files"`
	NextCursor       string         `json:"next_cursor,omitempty"`
	Truncated        bool           `json:"truncated"`
	TruncationReason string         `json:"truncation_reason,omitempty"`
}

type FileDiffPage struct {
	Path      string     `json:"path"`
	OldPath   string     `json:"old_path,omitempty"`
	IsNew     bool       `json:"is_new,omitempty"`
	IsDeleted bool       `json:"is_deleted,omitempty"`
	IsRenamed bool       `json:"is_renamed,omitempty"`
	Hunks     []HunkPage `json:"hunks"`
}

type HunkPage struct {
	Header string     `json:"header"`
	Lines  []LinePage `json:"lines"`
}

type LinePage struct {
	DiffPosition int    `json:"diff_position"`
	OldLineNo    int    `json:"old_line_no,omitempty"`
	NewLineNo    int    `json:"new_line_no,omitempty"`
	Type         string `json:"type"`
	Content      string `json:"content"`
}

type PageInput struct {
	PRURL      string
	FileFilter string
	Cursor     string
	MaxLines   int
	MaxFiles   int
}

func FilterHash(prURL, fileFilter string, maxLines, maxFiles int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d", prURL, fileFilter, maxLines, maxFiles)))
	return fmt.Sprintf("%x", h[:8])
}

func EncodeCursor(c Cursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encoding cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeCursor(s string) (Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("decoding cursor: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, fmt.Errorf("parsing cursor: %w", err)
	}
	return c, nil
}

func (pd *ParsedDiff) GetPage(input PageInput) (*DiffPage, error) {
	fh := FilterHash(input.PRURL, input.FileFilter, input.MaxLines, input.MaxFiles)
	var start Cursor
	if input.Cursor != "" {
		var err error
		start, err = DecodeCursor(input.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		if start.FilterHash != fh {
			return nil, fmt.Errorf("cursor was generated for different request parameters")
		}
	} else {
		start = Cursor{FilterHash: fh}
	}

	page := &DiffPage{}
	lineCount := 0
	fileCount := 0
	done := false

	for fi, f := range pd.Files {
		if fi < start.FileIndex {
			continue
		}
		if input.FileFilter != "" && f.Path != input.FileFilter {
			continue
		}
		if input.MaxFiles > 0 && input.Cursor == "" && fileCount >= input.MaxFiles {
			next, err := EncodeCursor(Cursor{FileIndex: fi, FilterHash: fh})
			if err != nil {
				return nil, err
			}
			page.NextCursor = next
			page.Truncated = true
			page.TruncationReason = "max_files"
			break
		}

		var fp *FileDiffPage
		for hi, h := range f.Hunks {
			if fi == start.FileIndex && hi < start.HunkIndex {
				continue
			}
			var hp *HunkPage
			for li, l := range h.Lines {
				if fi == start.FileIndex && hi == start.HunkIndex && li < start.LineIndex {
					continue
				}
				if input.MaxLines > 0 && lineCount >= input.MaxLines {
					next, err := EncodeCursor(Cursor{
						FileIndex:  fi,
						HunkIndex:  hi,
						LineIndex:  li,
						FilterHash: fh,
					})
					if err != nil {
						return nil, err
					}
					page.NextCursor = next
					page.Truncated = true
					page.TruncationReason = "max_lines"
					done = true
					break
				}
				if fp == nil {
					fp = &FileDiffPage{
						Path:      f.Path,
						OldPath:   f.OldPath,
						IsNew:     f.IsNew,
						IsDeleted: f.IsDeleted,
						IsRenamed: f.IsRenamed,
					}
				}
				if hp == nil {
					hp = &HunkPage{Header: h.Header}
				}
				hp.Lines = append(hp.Lines, toLinePage(l))
				lineCount++
			}
			if hp != nil {
				fp.Hunks = append(fp.Hunks, *hp)
			}
			if done {
				break
			}
		}
		if fp != nil {
			page.Files = append(page.Files, *fp)
			fileCount++
		}
		if done {
			break
		}
	}
	return page, nil
}

func toLinePage(l DiffLine) LinePage {
	t := "context"
	switch l.Type {
	case LineAdded:
		t = "added"
	case LineRemoved:
		t = "removed"
	}
	return LinePage{
		DiffPosition: l.DiffPosition,
		OldLineNo:    l.OldLineNo,
		NewLineNo:    l.NewLineNo,
		Type:         t,
		Content:      l.Content,
	}
}
