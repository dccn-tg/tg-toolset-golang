package strings

import (
    log "github.com/sirupsen/logrus"
    "strings"
)

var logger *log.Entry

func init() {
    // set logging
    logger = log.WithFields(log.Fields{"source":"utility.strings"})
}

// StringWrap wraps text at the specified column lineWidth on word breaks
func StringWrap(text string, lineWidth int) string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return text
	}
	wrapped := words[0]
	spaceLeft := lineWidth - len(wrapped)
	for _, word := range words[1:] {
		if len(word)+1 > spaceLeft {
			wrapped += "\n" + word
			spaceLeft = lineWidth - len(word)
		} else {
			wrapped += " " + word
			spaceLeft -= 1 + len(word)
		}
	}
	return wrapped
}

// StringXOR finds the XOR between two strings and return the XOR part in byte array.
func StringXOR(str1, str2 string) []byte {

    var out []byte

    set1 := make(map[byte]bool)
    set2 := make(map[byte]bool)

    for _,c := range []byte(str1) {
        set1[c] = true
    }

    for _,c := range []byte(str2) {
        set2[c] = true
    }

    for _,c := range []byte(str1) {
        if !set2[c] {
            out = append(out, c)
        }
    }

    for _,c := range []byte(str2) {
        if !set1[c] {
            out = append(out, c)
        }
    }

    return out
}

// StringAND finds the AND between two strings and return the AND part in byte array.
func StringAND(str1, str2 string) []byte {

    var out []byte

    set1 := make(map[byte]bool)

    for _,c := range []byte(str1) {
        set1[c] = true
    }

    for _,c := range []byte(str2) {
        if set1[c] {
            out = append(out, c)
        }
    }

    return out
}