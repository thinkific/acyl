// Code generated by gocc; DO NOT EDIT.

// This file is dual licensed under CC0 and The gonum license.
//
// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Copyright ©2017 Robin Eklind.
// This file is made available under a Creative Commons CC0 1.0
// Universal Public Domain Dedication.

package lexer

import (
	"io/ioutil"
	"unicode/utf8"

	"gonum.org/v1/gonum/graph/formats/dot/internal/token"
)

const (
	NoState    = -1
	NumStates  = 141
	NumSymbols = 184
)

type Lexer struct {
	src    []byte
	pos    int
	line   int
	column int
}

func NewLexer(src []byte) *Lexer {
	lexer := &Lexer{
		src:    src,
		pos:    0,
		line:   1,
		column: 1,
	}
	return lexer
}

func NewLexerFile(fpath string) (*Lexer, error) {
	src, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	return NewLexer(src), nil
}

func (l *Lexer) Scan() (tok *token.Token) {
	tok = new(token.Token)
	if l.pos >= len(l.src) {
		tok.Type = token.EOF
		tok.Pos.Offset, tok.Pos.Line, tok.Pos.Column = l.pos, l.line, l.column
		return
	}
	start, startLine, startColumn, end := l.pos, l.line, l.column, 0
	tok.Type = token.INVALID
	state, rune1, size := 0, rune(-1), 0
	for state != -1 {
		if l.pos >= len(l.src) {
			rune1 = -1
		} else {
			rune1, size = utf8.DecodeRune(l.src[l.pos:])
			l.pos += size
		}

		nextState := -1
		if rune1 != -1 {
			nextState = TransTab[state](rune1)
		}
		state = nextState

		if state != -1 {

			switch rune1 {
			case '\n':
				l.line++
				l.column = 1
			case '\r':
				l.column = 1
			case '\t':
				l.column += 4
			default:
				l.column++
			}

			switch {
			case ActTab[state].Accept != -1:
				tok.Type = ActTab[state].Accept
				end = l.pos
			case ActTab[state].Ignore != "":
				start, startLine, startColumn = l.pos, l.line, l.column
				state = 0
				if start >= len(l.src) {
					tok.Type = token.EOF
				}

			}
		} else {
			if tok.Type == token.INVALID {
				end = l.pos
			}
		}
	}
	if end > start {
		l.pos = end
		tok.Lit = l.src[start:end]
	} else {
		tok.Lit = []byte{}
	}
	tok.Pos.Offset, tok.Pos.Line, tok.Pos.Column = start, startLine, startColumn

	return
}

func (l *Lexer) Reset() {
	l.pos = 0
}

/*
Lexer symbols:
0: 'n'
1: 'o'
2: 'd'
3: 'e'
4: 'N'
5: 'o'
6: 'd'
7: 'e'
8: 'N'
9: 'O'
10: 'D'
11: 'E'
12: 'e'
13: 'd'
14: 'g'
15: 'e'
16: 'E'
17: 'd'
18: 'g'
19: 'e'
20: 'E'
21: 'D'
22: 'G'
23: 'E'
24: 'g'
25: 'r'
26: 'a'
27: 'p'
28: 'h'
29: 'G'
30: 'r'
31: 'a'
32: 'p'
33: 'h'
34: 'G'
35: 'R'
36: 'A'
37: 'P'
38: 'H'
39: 'd'
40: 'i'
41: 'g'
42: 'r'
43: 'a'
44: 'p'
45: 'h'
46: 'D'
47: 'i'
48: 'g'
49: 'r'
50: 'a'
51: 'p'
52: 'h'
53: 'd'
54: 'i'
55: 'G'
56: 'r'
57: 'a'
58: 'p'
59: 'h'
60: 'D'
61: 'i'
62: 'G'
63: 'r'
64: 'a'
65: 'p'
66: 'h'
67: 'D'
68: 'I'
69: 'G'
70: 'R'
71: 'A'
72: 'P'
73: 'H'
74: 's'
75: 'u'
76: 'b'
77: 'g'
78: 'r'
79: 'a'
80: 'p'
81: 'h'
82: 'S'
83: 'u'
84: 'b'
85: 'g'
86: 'r'
87: 'a'
88: 'p'
89: 'h'
90: 's'
91: 'u'
92: 'b'
93: 'G'
94: 'r'
95: 'a'
96: 'p'
97: 'h'
98: 'S'
99: 'u'
100: 'b'
101: 'G'
102: 'r'
103: 'a'
104: 'p'
105: 'h'
106: 'S'
107: 'U'
108: 'B'
109: 'G'
110: 'R'
111: 'A'
112: 'P'
113: 'H'
114: 's'
115: 't'
116: 'r'
117: 'i'
118: 'c'
119: 't'
120: 'S'
121: 't'
122: 'r'
123: 'i'
124: 'c'
125: 't'
126: 'S'
127: 'T'
128: 'R'
129: 'I'
130: 'C'
131: 'T'
132: '{'
133: '}'
134: ';'
135: '-'
136: '-'
137: '-'
138: '>'
139: '['
140: ']'
141: ','
142: '='
143: ':'
144: '_'
145: '-'
146: '.'
147: '-'
148: '.'
149: '\'
150: '"'
151: '\'
152: '"'
153: '"'
154: '='
155: '<'
156: '>'
157: '<'
158: '>'
159: '/'
160: '/'
161: '\n'
162: '#'
163: '\n'
164: '/'
165: '*'
166: '*'
167: '*'
168: '/'
169: ' '
170: '\t'
171: '\r'
172: '\n'
173: \u0001-'!'
174: '#'-'['
175: ']'-\u007f
176: 'a'-'z'
177: 'A'-'Z'
178: '0'-'9'
179: \u0080-\ufffc
180: \ufffe-\U0010ffff
181: \u0001-';'
182: '?'-\u00ff
183: .
*/
