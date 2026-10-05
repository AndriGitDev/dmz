package detect

import "strings"

// Only these explicitly labelled programming-language blocks establish that an
// unquoted assignment is an expression. Env/shell/YAML and unknown fences keep
// their existing literal handling. This is not a complete Markdown/AST parser.
func credentialProgramLanguage(info string) bool {
	fields := strings.Fields(info)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "python", "py", "javascript", "js", "jsx", "typescript", "ts", "tsx",
		"go", "golang", "java", "c", "cpp", "c++", "csharp", "cs", "c#",
		"rust", "rs", "ruby", "rb", "php", "kotlin", "swift":
		return true
	}
	return false
}

func credentialCodeRanges(input string) [][2]int {
	var ranges [][2]int
	var fence byte
	width, begin, offset := 0, 0, 0
	program := false
	for _, line := range strings.SplitAfter(input, "\n") {
		text := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		indent := 0
		for indent < len(text) && text[indent] == ' ' {
			indent++
		}
		if indent <= 3 {
			text = text[indent:]
			if len(text) >= 3 && (text[0] == '`' || text[0] == '~') {
				count := 1
				for count < len(text) && text[count] == text[0] {
					count++
				}
				if count >= 3 {
					rest := strings.Trim(text[count:], " \t")
					if fence == 0 && (text[0] != '`' || !strings.Contains(rest, "`")) {
						fence, width, begin = text[0], count, offset+len(line)
						program = credentialProgramLanguage(rest)
					} else if fence == text[0] && count >= width && rest == "" {
						if program {
							ranges = append(ranges, [2]int{begin, offset})
						}
						fence, program = 0, false
					}
				}
			}
		}
		offset += len(line)
	}
	if fence != 0 && program {
		ranges = append(ranges, [2]int{begin, len(input)})
	}
	return ranges
}
