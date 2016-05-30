package runner

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/lhchavez/quark/common"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// isSpace returns true if the rune is either an unicode space or a Java
// whitespace character. The only characters that seem to be Java whitespace
// but not unicode whitespace are:
// U+001C FILE SEPARATOR
// U+001D GROUP SEPARATOR
// U+001E RECORD SEPARATOR
// U+001F UNIT SEPARATOR
func isSpace(r rune) bool {
	return unicode.IsSpace(r) || ('\u001c' <= r && r <= '\u001f')
}

// scanTokens is a split function for a Scanner similar to bufio.ScanWords,
// except that it also treats some runes that Java treats as spaces as spaces.
func scanTokens(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading spaces.
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !isSpace(r) {
			break
		}
	}
	// Scan until space, marking end of word.
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if isSpace(r) {
			return i + width, data[start:i], nil
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}
	// Request more data.
	return start, nil, nil
}

func isNumericRune(r rune) bool {
	return r == '.' || r == '-' || ('0' <= r && r <= '9')
}

func scanNumericTokens(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip non-numeric characters.
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if isNumericRune(r) {
			break
		}
	}
	// Scan until non-numeric.
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if !isNumericRune(r) {
			return i + width, data[start:i], nil
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated token. Return it.
	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}
	// Request more data
	return start, nil, nil
}

func CalculateScore(
	settings *common.ValidatorSettings,
	contestantOutput, expectedOutput io.Reader,
) (float64, error) {
	contestantScanner := bufio.NewScanner(contestantOutput)
	scanFunc := scanTokens
	if settings.Name == "token-numeric" {
		scanFunc = scanNumericTokens
	}

	contestantScanner.Split(scanFunc)
	if settings.Name == "literal" || settings.Name == "custom" {
		if !contestantScanner.Scan() {
			return 0, io.ErrUnexpectedEOF
		}
		value, err := strconv.ParseFloat(contestantScanner.Text(), 64)
		return math.Max(0, math.Min(1, value)), err
	}

	expectedScanner := bufio.NewScanner(expectedOutput)
	expectedScanner.Split(scanFunc)

	correct := true
	for correct {
		expectedNext := expectedScanner.Scan()
		contestantNext := contestantScanner.Scan()
		if expectedNext != contestantNext {
			correct = false
		}
		if !expectedNext {
			break
		}
		switch settings.Name {
		case "token":
			correct = token(expectedScanner.Text(), contestantScanner.Text())
		case "token-caseless":
			correct = tokenCaseless(expectedScanner.Text(), contestantScanner.Text())
		case "token-numeric":
			correct = tokenNumeric(
				expectedScanner.Text(),
				contestantScanner.Text(),
				*settings.Tolerance,
			)
		default:
			return 0, errors.New(fmt.Sprintf("Unknown validator: %q", settings.Name))
		}
	}
	if !correct {
		return 0.0, nil
	}
	return 1.0, nil
}

func token(a, b string) bool {
	return a == b
}

func tokenCaseless(a, b string) bool {
	return strings.EqualFold(a, b)
}

func tokenNumeric(a, b string, tolerance float64) bool {
	af, erra := strconv.ParseFloat(a, 64)
	bf, errb := strconv.ParseFloat(b, 64)
	if erra == nil && errb == nil {
		return math.Abs(af-bf) <= math.Abs(af)*tolerance
	}
	return erra != nil && errb != nil
}
