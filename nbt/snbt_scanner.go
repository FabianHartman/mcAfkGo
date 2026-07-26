package nbt

import "strconv"

const (
	scanContinue = iota
	scanBeginLiteral
	scanBeginCompound
	scanBeginList
	scanListValue
	scanListType
	scanCompoundTagName
	scanCompoundValue
	scanSkipSpace
	scanEndValue

	scanEnd
	scanError
)

const (
	parseCompoundName = iota
	parseCompoundValue
	parseListValue
)

const maxNestingDepth = 10000

type scanner struct {
	step       func(s *scanner, c byte) int
	parseState []int
	errContext string
	endTop     bool
}

func (s *scanner) reset() {
	s.step = stateBeginValue
	s.parseState = s.parseState[0:0]
	s.errContext = ""
	s.endTop = false
}

func (s *scanner) pushParseState(newParseState int, successState int) int {
	s.parseState = append(s.parseState, newParseState)
	if len(s.parseState) <= maxNestingDepth {
		return successState
	}

	return scanError
}

func (s *scanner) popParseState() {
	n := len(s.parseState) - 1
	s.parseState = s.parseState[:n]
	if n == 0 {
		s.step = stateEndTop
		s.endTop = true
	} else {
		s.step = stateEndValue
	}
}

func (s *scanner) eof() int {
	if s.errContext != "" {
		return scanError
	}

	if s.endTop {
		return scanEnd
	}

	s.step(s, ' ')
	if s.endTop {
		return scanEnd
	}

	if s.errContext == "" {
		s.errContext = "unexpected end of SNBT input"
	}

	return scanError
}

func stateEndTop(s *scanner, c byte) int {
	if !isSpace(c) {
		s.error(c, "after top-level value")
	}

	return scanEnd
}

func stateBeginValue(s *scanner, c byte) int {
	if isSpace(c) {
		s.step = stateBeginValue

		return scanSkipSpace
	}
	switch c {
	case '{':
		s.step = stateCompoundOrEmpty
		return s.pushParseState(parseCompoundName, scanBeginCompound)
	case '[':
		s.step = stateListOrArray
		return s.pushParseState(parseListValue, scanBeginList)
	case '"', '\'':
		return stateBeginString(s, c)
	default:
		if isNumber(c) || c == '-' || c == '+' {
			stateNum0(s, c)

			return scanBeginLiteral
		}

		if isAllowedInUnquotedString(c) {
			return stateBeginString(s, c)
		}
	}

	return s.error(c, "looking for beginning of value")
}

func stateCompoundOrEmpty(s *scanner, c byte) int {
	if isSpace(c) {
		return scanSkipSpace
	}
	if c == '}' {
		n := len(s.parseState)
		s.parseState[n-1] = parseCompoundValue

		return stateEndValue(s, c)
	}

	return stateBeginString(s, c)
}

func stateBeginString(s *scanner, c byte) int {
	if isSpace(c) {
		return scanSkipSpace
	}

	switch c {
	case '\'':
		s.step = stateInSingleQuotedString

		return scanBeginLiteral
	case '"':
		s.step = stateInDoubleQuotedString

		return scanBeginLiteral
	default:
		if isAllowedInUnquotedString(c) {
			s.step = stateInUnquotedString

			return scanBeginLiteral
		}
	}

	return s.error(c, "looking for beginning of string")
}

func stateInSingleQuotedString(s *scanner, c byte) int {
	if c == '\\' {
		s.step = stateInSingleQuotedStringEsc

		return scanContinue
	}

	if c == '\'' {
		s.step = stateEndValue

		return scanContinue
	}

	return scanContinue
}

func stateInSingleQuotedStringEsc(s *scanner, c byte) int {
	switch c {
	case '\\', '\'':
		s.step = stateInSingleQuotedString

		return scanContinue
	}

	return s.error(c, "in string escape code")
}

func stateInDoubleQuotedString(s *scanner, c byte) int {
	if c == '\\' {
		s.step = stateInDqStringEsc

		return scanContinue
	}

	if c == '"' {
		s.step = stateEndValue

		return scanContinue
	}

	return scanContinue
}

func stateInDqStringEsc(s *scanner, c byte) int {
	switch c {
	case 'b', 'f', 'n', 'r', 't', '\\', '/', '"':
		s.step = stateInDoubleQuotedString

		return scanContinue
	}

	return s.error(c, "in string escape code")
}

func stateInUnquotedString(s *scanner, c byte) int {
	if isAllowedInUnquotedString(c) {
		return scanContinue
	}

	return stateEndValue(s, c)
}

func stateListOrArray(s *scanner, c byte) int {
	if isSpace(c) {
		return scanSkipSpace
	}

	switch c {
	case 'B', 'I', 'L':
		s.step = stateListOrArrayT

		return scanBeginLiteral
	case ']':
		return stateEndValue(s, c)
	default:
		return stateBeginValue(s, c)
	}
}

func stateListOrArrayT(s *scanner, c byte) int {
	if c == ';' {
		s.step = stateArrayT

		return scanListType
	}

	return stateInUnquotedString(s, c)
}

func stateArrayT(s *scanner, c byte) int {
	if isSpace(c) {
		return scanSkipSpace
	}

	if c == ']' { // empty array
		return stateEndValue(s, c)
	}

	return stateBeginValue(s, c)
}

func stateNum0(s *scanner, c byte) int {
	if isNumber(c) || c == '-' || c == '+' {
		s.step = stateNum1

		return scanContinue
	}

	return stateEndNumValue(s, c)
}

func stateNum1(s *scanner, c byte) int {
	if isNumber(c) {
		s.step = stateNum1

		return scanContinue
	}

	if c == '.' {
		s.step = stateNumDot

		return scanContinue
	}

	return stateEndNumValue(s, c)
}

func stateNumDot(s *scanner, c byte) int {
	if isNumber(c) {
		s.step = stateNumDot0

		return scanContinue
	}

	switch c {
	case 'e', 'E':
		s.step = stateNumExp

		return scanContinue
	}

	if isAllowedInUnquotedString(c) {
		s.step = stateInUnquotedString

		return scanContinue
	}

	return s.error(c, "after decimal point in numeric literal")
}

func stateNumDot0(s *scanner, c byte) int {
	if isNumber(c) {
		s.step = stateNumDot0

		return scanContinue
	}

	switch c {
	case 'e', 'E':
		s.step = stateNumExp

		return scanContinue
	}

	return stateEndNumDotValue(s, c)
}

func stateNumExp(s *scanner, c byte) int {
	if isNumber(c) || c == '-' || c == '+' {
		s.step = stateNumExp0

		return scanContinue
	}

	return stateEndNumDotValue(s, c)
}

func stateNumExp0(s *scanner, c byte) int {
	if isNumber(c) {
		s.step = stateNumExp0

		return scanContinue
	}

	return stateEndNumDotValue(s, c)
}

func stateEndNumValue(s *scanner, c byte) int {
	switch c {
	case 'b', 'B':
		s.step = stateEndValue

		return scanContinue
	case 's', 'S':
		s.step = stateEndValue

		return scanContinue
	case 'l', 'L':
		s.step = stateEndValue

		return scanContinue
	case 'f', 'F', 'd', 'D':
		return stateEndNumDotValue(s, c)
	}

	if isAllowedInUnquotedString(c) {
		s.step = stateInUnquotedString

		return scanContinue
	}

	return stateEndValue(s, c)
}

func stateEndNumDotValue(s *scanner, c byte) int {
	switch c {
	case 'f', 'F':
		s.step = stateEndValue

		return scanContinue
	case 'd', 'D':
		s.step = stateEndValue

		return scanContinue
	}

	return stateEndValue(s, c)
}

func stateEndValue(s *scanner, c byte) int {
	n := len(s.parseState)
	if n == 0 {
		s.step = stateEndTop
		s.endTop = true

		return stateEndTop(s, c)
	}

	if isSpace(c) {
		return scanSkipSpace
	}

	ps := s.parseState[n-1]

	switch ps {
	case parseCompoundName:
		if c == ':' {
			s.parseState[n-1] = parseCompoundValue
			s.step = stateBeginValue

			return scanCompoundTagName
		}

		return s.error(c, "after compound tag name")
	case parseCompoundValue:
		switch c {
		case ',':
			s.parseState[n-1] = parseCompoundName
			s.step = stateBeginString

			return scanCompoundValue
		case '}':
			s.popParseState()

			return scanEndValue
		}

		return s.error(c, "after compound value")
	case parseListValue:
		switch c {
		case ',':
			s.step = stateBeginValue

			return scanListValue
		case ']':
			s.popParseState()

			return scanEndValue
		}

		return s.error(c, "after list element")
	}

	return s.error(c, "")
}

func (s *scanner) error(c byte, context string) int {
	s.step = stateError
	s.errContext = "invalid character " + quoteChar(c) + " " + context

	return scanError
}

func stateError(*scanner, byte) int {
	return scanError
}

func isSpace(c byte) bool {
	return c <= ' ' && (c == ' ' || c == '\t' || c == '\r' || c == '\n')
}

func isNumber(c byte) bool {
	return c >= '0' && c <= '9'
}

func isAllowedInUnquotedString(c byte) bool {
	return c == '_' || c == '-' ||
		c == '.' || c == '+' ||
		c >= '0' && c <= '9' ||
		c >= 'A' && c <= 'Z' ||
		c >= 'a' && c <= 'z'
}

func quoteChar(c byte) string {
	if c == '\'' {
		return `'\''`
	}

	if c == '"' {
		return `'"'`
	}

	s := strconv.Quote(string(c))

	return "'" + s[1:len(s)-1] + "'"
}
