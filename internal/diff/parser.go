package diff

import (
	"fmt"
	"strconv"
	"strings"
)

type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

type DiffLine struct {
	DiffPosition int
	OldLineNo    int
	NewLineNo    int
	Type         LineType
	Content      string
}

type Hunk struct {
	Header   string
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

type FileDiff struct {
	Path      string
	OldPath   string
	IsNew     bool
	IsDeleted bool
	IsRenamed bool
	Hunks     []Hunk
}

type ParsedDiff struct {
	Files []FileDiff
}

func Parse(raw string) (*ParsedDiff, error) {
	lines := strings.Split(raw, "\n")
	pd := &ParsedDiff{}

	var cur *FileDiff
	var curHunk *Hunk
	diffPos := 0

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")

		switch {
		case strings.HasPrefix(line, "diff --git "):
			if cur != nil {
				pd.Files = append(pd.Files, *cur)
			}
			cur = &FileDiff{}
			curHunk = nil
			diffPos = 0
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				cur.OldPath = cleanPath(parts[2])
				cur.Path = cleanPath(parts[3])
			}
		case strings.HasPrefix(line, "new file mode"):
			if cur != nil {
				cur.IsNew = true
			}
		case strings.HasPrefix(line, "deleted file mode"):
			if cur != nil {
				cur.IsDeleted = true
			}
		case strings.HasPrefix(line, "rename from "):
			if cur != nil {
				cur.IsRenamed = true
				cur.OldPath = strings.TrimPrefix(line, "rename from ")
			}
		case strings.HasPrefix(line, "rename to "):
			if cur != nil {
				cur.IsRenamed = true
				cur.Path = strings.TrimPrefix(line, "rename to ")
			}
		case strings.HasPrefix(line, "--- "):
			if cur != nil {
				p := cleanPath(strings.TrimPrefix(line, "--- "))
				if p != "/dev/null" && p != "" {
					cur.OldPath = p
				}
			}
		case strings.HasPrefix(line, "+++ "):
			if cur != nil {
				p := cleanPath(strings.TrimPrefix(line, "+++ "))
				if p != "/dev/null" && p != "" {
					cur.Path = p
				}
			}
		case strings.HasPrefix(line, "@@ "):
			if cur == nil {
				continue
			}
			diffPos++
			hunk, err := parseHunkHeader(line)
			if err != nil {
				return nil, fmt.Errorf("file %q: %w", cur.Path, err)
			}
			hunk.Header = line
			cur.Hunks = append(cur.Hunks, *hunk)
			curHunk = &cur.Hunks[len(cur.Hunks)-1]
		case curHunk != nil && len(line) > 0:
			switch line[0] {
			case '+', '-', ' ':
			default:
				continue
			}
			diffPos++
			dl := DiffLine{DiffPosition: diffPos, Content: line[1:]}
			switch line[0] {
			case '+':
				dl.Type = LineAdded
				curHunk.NewStart++
				dl.NewLineNo = curHunk.NewStart
			case '-':
				dl.Type = LineRemoved
				curHunk.OldStart++
				dl.OldLineNo = curHunk.OldStart
			case ' ':
				dl.Type = LineContext
				curHunk.OldStart++
				curHunk.NewStart++
				dl.OldLineNo = curHunk.OldStart
				dl.NewLineNo = curHunk.NewStart
			}
			curHunk.Lines = append(curHunk.Lines, dl)
		}
	}

	if cur != nil {
		pd.Files = append(pd.Files, *cur)
	}
	return pd, nil
}

func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "a/")
	p = strings.TrimPrefix(p, "b/")
	return p
}

func parseHunkHeader(header string) (*Hunk, error) {
	inner := strings.TrimPrefix(header, "@@ ")
	if idx := strings.Index(inner, " @@"); idx >= 0 {
		inner = inner[:idx]
	}
	parts := strings.Fields(inner)
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed hunk header: %q", header)
	}
	oldStart, oldCount, err := parseRange(parts[0])
	if err != nil {
		return nil, err
	}
	newStart, newCount, err := parseRange(parts[1])
	if err != nil {
		return nil, err
	}
	return &Hunk{
		OldStart: oldStart - 1,
		OldCount: oldCount,
		NewStart: newStart - 1,
		NewCount: newCount,
	}, nil
}

func parseRange(s string) (start, count int, err error) {
	s = strings.TrimLeft(s, "+-")
	parts := strings.SplitN(s, ",", 2)
	start, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("bad range start %q: %w", s, err)
	}
	if len(parts) == 2 {
		count, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("bad range count %q: %w", s, err)
		}
	} else {
		count = 1
	}
	return start, count, nil
}

func (pd *ParsedDiff) FindLine(filePath string, newLineNo int) (DiffLine, error) {
	for _, f := range pd.Files {
		if f.Path != filePath {
			continue
		}
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				if l.NewLineNo == newLineNo {
					return l, nil
				}
			}
		}
		return DiffLine{}, fmt.Errorf("line %d in %q is not part of any diff hunk", newLineNo, filePath)
	}
	return DiffLine{}, fmt.Errorf("file %q not found in diff", filePath)
}

func (pd *ParsedDiff) OldPathFor(filePath string) string {
	for _, f := range pd.Files {
		if f.Path == filePath {
			if f.OldPath != "" {
				return f.OldPath
			}
			return f.Path
		}
	}
	return filePath
}
