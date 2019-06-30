// Package reggen generates text based on regex definitions
// This lib is from https://github.com/lucasjones/reggen/blob/master/reggen.go
package utilities

import (
	"fmt"
	"hfrd/modules/gosdk/common"
	"math"
	"math/rand"
	"os"
	"regexp/syntax"
	"time"
)

const runeRangeEnd = 0x10ffff
const printableChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~ \t\n\r"

var printableCharsNoNL = printableChars[:len(printableChars)-2]

type state struct {
	limit int
}

type Generator struct {
	re    *syntax.Regexp
	rand  *rand.Rand
	debug bool
}

func (g *Generator) generate(s *state, re *syntax.Regexp) string {
	op := re.Op
	switch op {
	case syntax.OpNoMatch:
	case syntax.OpEmptyMatch:
		return ""
	case syntax.OpLiteral:
		res := ""
		for _, r := range re.Rune {
			res += string(r)
		}
		return res
	case syntax.OpCharClass:
		// number of possible chars
		sum := 0
		for i := 0; i < len(re.Rune); i += 2 {
			if g.debug {
				common.Logger.Info(fmt.Sprintf("Range: %#U-%#U\n", re.Rune[i], re.Rune[i+1]))
			}
			sum += int(re.Rune[i+1]-re.Rune[i]) + 1
			if re.Rune[i+1] == runeRangeEnd {
				sum = -1
				break
			}
		}
		// pick random char in range (inverse match group)
		if sum == -1 {
			possibleChars := []uint8{}
			for j := 0; j < len(printableChars); j++ {
				c := printableChars[j]
				// Check c in range
				for i := 0; i < len(re.Rune); i += 2 {
					if rune(c) >= re.Rune[i] && rune(c) <= re.Rune[i+1] {
						possibleChars = append(possibleChars, c)
						break
					}
				}
			}
			if len(possibleChars) > 0 {
				c := possibleChars[g.rand.Intn(len(possibleChars))]
				if g.debug {
					common.Logger.Info(fmt.Sprintf("Generated rune %c for inverse range %v\n", c, re))
				}
				return string([]byte{c})
			}
		}
		if g.debug {
			fmt.Println("Char range: ", sum)
		}
		r := g.rand.Intn(int(sum))
		var ru rune
		sum = 0
		for i := 0; i < len(re.Rune); i += 2 {
			gap := int(re.Rune[i+1]-re.Rune[i]) + 1
			if sum+gap > r {
				ru = re.Rune[i] + rune(r-sum)
				break
			}
			sum += gap
		}
		if g.debug {
			fmt.Printf("Generated rune %c for range %v\n", ru, re)
		}
		return string(ru)
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		chars := printableChars
		if op == syntax.OpAnyCharNotNL {
			chars = printableCharsNoNL
		}
		c := chars[g.rand.Intn(len(chars))]
		return string([]byte{c})
	case syntax.OpBeginLine:
	case syntax.OpEndLine:
	case syntax.OpBeginText:
	case syntax.OpEndText:
	case syntax.OpWordBoundary:
	case syntax.OpNoWordBoundary:
	case syntax.OpCapture:
		if g.debug {
			fmt.Println("OpCapture", re.Sub, len(re.Sub))
		}
		return g.generate(s, re.Sub0[0])
	case syntax.OpStar:
		// Repeat zero or more times
		res := ""
		count := g.rand.Intn(s.limit + 1)
		for i := 0; i < count; i++ {
			for _, r := range re.Sub {
				res += g.generate(s, r)
			}
		}
		return res
	case syntax.OpPlus:
		// Repeat one or more times
		res := ""
		count := g.rand.Intn(s.limit) + 1
		for i := 0; i < count; i++ {
			for _, r := range re.Sub {
				res += g.generate(s, r)
			}
		}
		return res
	case syntax.OpQuest:
		// Zero or one instances
		res := ""
		count := g.rand.Intn(2)
		if g.debug {
			fmt.Println("Quest", count)
		}
		for i := 0; i < count; i++ {
			for _, r := range re.Sub {
				res += g.generate(s, r)
			}
		}
		return res
	case syntax.OpRepeat:
		// Repeat one or more times
		if g.debug {
			fmt.Println("OpRepeat", re.Min, re.Max)
		}
		res := ""
		count := 0
		re.Max = int(math.Min(float64(re.Max), float64(s.limit)))
		if re.Max > re.Min {
			count = g.rand.Intn(re.Max - re.Min + 1)
		}
		if g.debug {
			fmt.Println(re.Max, count)
		}
		for i := 0; i < re.Min || i < (re.Min+count); i++ {
			for _, r := range re.Sub {
				res += g.generate(s, r)
			}
		}
		return res
	case syntax.OpConcat:
		// Concatenate sub-regexes
		res := ""
		for _, r := range re.Sub {
			res += g.generate(s, r)
		}
		return res
	case syntax.OpAlternate:
		if g.debug {
			fmt.Println("OpAlternative", re.Sub, len(re.Sub))
		}
		i := g.rand.Intn(len(re.Sub))
		return g.generate(s, re.Sub[i])
	default:
		fmt.Fprintln(os.Stderr, "[reg-gen] Unhandled op: ", op)
	}
	return ""
}

// limit is the maximum number of times star, range or plus should repeat
// i.e. [0-9]+ will generate at most 10 characters if this is set to 10
func (g *Generator) Generate(limit int) string {
	return g.generate(&state{limit: limit}, g.re)
}

// create a new generator
func NewGenerator(regex string) (*Generator, error) {
	re, err := syntax.Parse(regex, syntax.Perl)
	if err != nil {
		return nil, err
	}
	//fmt.Println("Compiled re ", re)
	return &Generator{
		re:   re,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func GenerateRegexString(regex string, limit int) (string, error) {
	g, err := NewGenerator(regex)
	if err != nil {
		return "", err
	}
	return g.Generate(limit), nil
}
