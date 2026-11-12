package main

// Symbol extents: where a definition ends, not just where it starts. The flow "read as code"
// view needs the whole body of each function a step names. Like the rest of code
// intelligence this is patterns, not parsers: brace matching (skipping strings and comments)
// for C-like languages, indentation for Python and GDScript.

import (
	"path/filepath"
	"strings"
)

// maxSpan caps a block so a mismatched brace can't turn one function into the rest of the file.
const maxSpan = 400

var indentLangs = set("py", "gd", "pyi")

// fileLines returns a repo file's lines (cached like resolveSymbol), or nil.
func fileLines(file string) []string {
	var text string
	if v, ok := fileCache.Load(file); ok {
		text = v.(string)
	} else {
		t, err := readText(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return nil
		}
		text = t
		fileCache.Store(file, t)
	}
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

// symbolSpan is the [from, to] lines (1-based, inclusive) of a symbol's definition, with any
// doc comment or decorators directly above it. ok is false when the symbol was only found as
// a mention, not a definition, so there's no body to show.
func symbolSpan(file, symbol string) (from, to int, ok bool) {
	line, def := locateSymbol(file, symbol)
	if line == 0 || !def {
		return 0, 0, false
	}
	ls := fileLines(file)
	if ls == nil {
		return 0, 0, false
	}
	end := blockEnd(ls, line, extOf(file))
	return leadStart(ls, line, extOf(file)), end, true
}

// lineSpan is the extent of the statement or block that starts at a line (a `file:N` ref).
func lineSpan(file string, line int) (from, to int) {
	ls := fileLines(file)
	if ls == nil || line < 1 || line > len(ls) {
		return 0, 0
	}
	return line, blockEnd(ls, line, extOf(file))
}

// leadStart walks up over doc comments, attributes and decorators directly above a definition.
func leadStart(ls []string, line int, ext string) int {
	i := line - 1 // 0-based index of the definition
	inBlock := false
	for i > 0 {
		t := strings.TrimSpace(ls[i-1])
		switch {
		case inBlock:
			if strings.HasPrefix(t, "/*") {
				inBlock = false
			}
		case strings.HasSuffix(t, "*/") && !strings.HasPrefix(t, "/*"):
			inBlock = true
		case strings.Contains(t, "-----") || strings.Contains(t, "====="):
			return i + 1 // a section divider, not this definition's doc
		case strings.HasPrefix(t, "/*") && strings.HasSuffix(t, "*/"),
			strings.HasPrefix(t, "//"),
			strings.HasPrefix(t, "@"),
			strings.HasPrefix(t, "#[") || strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") && !indentLangs[ext],
			indentLangs[ext] && strings.HasPrefix(t, "#"):
		default:
			return i + 1
		}
		i--
	}
	return i + 1
}

// blockEnd returns the 1-based last line of the definition starting at line.
func blockEnd(ls []string, line int, ext string) int {
	if indentLangs[ext] {
		return indentEnd(ls, line)
	}
	return braceEnd(ls, line, ext)
}

// braceEnd scans from the definition line, tracking (), [] and {} together. The definition
// ends when a { opened at depth 0 closes, or at a ; at depth 0 before any body, or at the end
// of a line that leaves nothing open and doesn't continue on the next line.
func braceEnd(ls []string, line int, ext string) int {
	depth, body := 0, false
	inBlock, tpl := false, false // inside /* */, inside a `template` (spans lines)
	var interp []int             // depth at each open ${ in a template
	hashComment := ext == "php" || ext == "rb" || ext == "sh"
	rawTpl := ext == "go"                            // Go raw strings: no escapes inside
	jsLike := langByExt[ext] == "js" || htmlExt[ext] // has /regex/ literals
	last := min(len(ls), line-1+maxSpan)
	for i := line - 1; i < last; i++ {
		s := ls[i]
		for j := 0; j < len(s); j++ {
			c := s[j]
			if inBlock {
				if c == '*' && j+1 < len(s) && s[j+1] == '/' {
					inBlock = false
					j++
				}
				continue
			}
			if tpl {
				if c == '\\' && !rawTpl {
					j++
				} else if c == '`' {
					tpl = false
				} else if c == '$' && jsLike && j+1 < len(s) && s[j+1] == '{' {
					// ${…} inside a template is code again, and may hold templates of its own.
					interp = append(interp, depth)
					depth++
					tpl = false
					j++
				}
				continue
			}
			if c == '}' && len(interp) > 0 && depth-1 == interp[len(interp)-1] {
				interp = interp[:len(interp)-1]
				depth--
				tpl = true
				continue
			}
			switch c {
			case '/':
				if j+1 < len(s) && s[j+1] == '/' {
					j = len(s)
				} else if j+1 < len(s) && s[j+1] == '*' {
					inBlock = true
					j++
				} else if jsLike && regexStarts(s[:j]) {
					j = skipRegex(s, j)
				}
			case '#':
				if hashComment && !(j+1 < len(s) && s[j+1] == '[') {
					j = len(s)
				}
			case '"', '\'':
				// Strings end at the closing quote or the end of the line, so a Rust lifetime
				// or an apostrophe in odd syntax can't swallow the rest of the file.
				for j++; j < len(s) && s[j] != c; j++ {
					if s[j] == '\\' {
						j++
					}
				}
			case '`':
				tpl = true
			case '(', '[':
				depth++
			case '{':
				if depth == 0 {
					body = true
				}
				depth++
			case ')', ']', '}':
				depth--
				if depth < 0 { // started mid-block: stop at the enclosing block's end
					return max(line, i)
				}
				if depth == 0 && body && c == '}' {
					return i + 1
				}
			case ';':
				if depth == 0 && !body {
					return i + 1
				}
			}
		}
		if depth == 0 && !body && !inBlock && !tpl && !continues(ls, i) {
			return i + 1
		}
	}
	return last
}

var htmlExt = set("html", "htm")

// regexStarts says whether a / after this text opens a regex literal rather than dividing.
func regexStarts(before string) bool {
	t := strings.TrimRight(before, " \t")
	if t == "" {
		return true
	}
	if strings.ContainsRune("(,=:[!&|?{};+-*%>~^", rune(t[len(t)-1])) {
		return true
	}
	for _, kw := range []string{"return", "typeof", "case", "in", "of", "void", "yield", "await"} {
		if strings.HasSuffix(t, kw) && (len(t) == len(kw) || !isIdent(t[len(t)-len(kw)-1])) {
			return true
		}
	}
	return false
}

// skipRegex returns the index of the closing / of a regex literal starting at j (or the line end).
func skipRegex(s string, j int) int {
	class := false
	for j++; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '[':
			class = true
		case ']':
			class = false
		case '/':
			if !class {
				return j
			}
		}
	}
	return j
}

// continues says whether the statement on line i (with nothing left open) goes on: the line
// ends in an operator, or the next line opens the body (Allman braces) or chains a call.
func continues(ls []string, i int) bool {
	t := strings.TrimSpace(stripLineComment(ls[i]))
	if t == "" || strings.HasPrefix(t, "@") || strings.HasPrefix(t, "#[") {
		return true
	}
	for _, op := range []string{"=", ",", "=>", "->", "+", "-", "*", "&&", "||", "?", ":", ".", "|", "&", "extends", "implements"} {
		if strings.HasSuffix(t, op) {
			return true
		}
	}
	for k := i + 1; k < len(ls); k++ {
		n := strings.TrimSpace(ls[k])
		if n == "" {
			continue
		}
		return strings.HasPrefix(n, "{") || strings.HasPrefix(n, ".") || strings.HasPrefix(n, "?") ||
			strings.HasPrefix(n, ":") || strings.HasPrefix(n, "=>") || strings.HasPrefix(n, "extends") || strings.HasPrefix(n, "implements")
	}
	return false
}

func stripLineComment(s string) string {
	if i := strings.Index(s, "//"); i >= 0 && !strings.Contains(s[:i], "\"") && !strings.Contains(s[:i], "'") {
		return s[:i]
	}
	return s
}

// indentEnd: the header (which may span lines while brackets are open), then every line
// indented deeper than the definition. Blank and comment-only lines don't end the block but
// aren't counted as its last line.
func indentEnd(ls []string, line int) int {
	base := indentOf(ls[line-1])
	i, depth := line-1, 0
	for ; i < len(ls); i++ {
		for _, c := range stripHash(ls[i]) {
			switch c {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
		}
		if depth <= 0 {
			break
		}
	}
	end := min(i, len(ls)-1)
	for k := end + 1; k < len(ls) && k < line-1+maxSpan; k++ {
		t := strings.TrimSpace(ls[k])
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if indentOf(ls[k]) <= base {
			break
		}
		end = k
	}
	return end + 1
}

func stripHash(s string) string {
	if i := strings.IndexByte(s, '#'); i >= 0 {
		return s[:i]
	}
	return s
}

func indentOf(s string) int {
	n := 0
	for _, c := range s {
		switch c {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}
