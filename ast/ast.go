package ast

type LocationSpan struct {
	Start [2]int `yaml:"start,flow"`
	End   [2]int `yaml:"end,flow"`
}

type ParsingError struct {
	Position int    `yaml:"-"`
	Location [2]int `yaml:"location,flow"`
	Message  string `yaml:"message"`
}

const (
	NumberingBytes = iota
	NumberingUTF16 = iota
)

type File struct {
	Kind                  string         `yaml:"type"`
	Name                  string         `yaml:"name"`
	Numbering             int            `yaml:"-"`
	LocationSpan          LocationSpan   `yaml:"locationSpan,flow"`
	FooterSpan            [2]int         `yaml:"footerSpan,flow"`
	ParsingErrorsDetected bool           `yaml:"parsingErrorsDetected"`
	Children              []Node         `yaml:"children"`
	ParsingError          []ParsingError `yaml:"parsingError,omitempty"`
}

type Node struct {
	Kind         string       `yaml:"type"`
	Name         string       `yaml:"name"`
	LocationSpan LocationSpan `yaml:"locationSpan,flow"`
	Span         *[2]int      `yaml:"span,omitempty,flow"`       // only in leaf nodes
	HeaderSpan   *[2]int      `yaml:"headerSpan,omitempty,flow"` // \
	FooterSpan   *[2]int      `yaml:"footerSpan,omitempty,flow"` //  } only in container nodes
	Children     []Node       `yaml:"children,omitempty"`        // /
}

type Vitals struct {
	length int
	lines  []int // offsets of start of lines
	line   []int // line number of each byte
	col    []int // column number of each byte
	char   []int // character number of each byte
}

func MakeVitals(source []byte) (v *Vitals) {
	v = &Vitals{
		length: len(source),
		lines:  []int{-1, 0}, // 0th line doesn't exist, 1st line starts at byte 0
		line:   make([]int, len(source)+1),
		col:    make([]int, len(source)+1),
		char:   make([]int, len(source)+1),
	}

	line := 1 // start counting lines at 1
	col := 1  // columns at 1
	char := 0 // characters at 0

	for i, c := range source {
		v.line[i] = line
		v.col[i] = col
		v.char[i] = char

		switch true {
		case c == '\n' || c == '\r' && (len(source) <= i+1 || source[i+1] != '\n'):
			// at \n or (\r not followed by \n), the next character will be the beginning of the next line
			v.lines = append(v.lines, i+1)
			line++
			col = 0 // columns start at 1, and we're adding one below
		case c&0xc0 == 0x80:
			// bytes with bits 10xxxxxx indicate a multibyte continuation sequence
			// they don't count toward character or column counts
			v.col[i] = v.col[i-1]
			v.char[i] = v.char[i-1]
			continue
		case c&0xf0 == 0xf0:
			// bytes with bits 1111xxxx indicate unicode >0x10000
			// they take up two characters
			col++
			char++
		}

		// each character increments the position once
		col++
		char++
	}
	v.line[v.length] = line
	v.col[v.length] = col
	v.char[v.length] = char
	v.lines = append(v.lines, 2<<30-1)
	return
}

func (v *Vitals) GetLine(offset int) int {
	switch true {
	case offset < 0:
		return 1
	case offset > v.length:
		return len(v.line) - 2
	default:
		return v.line[offset]
	}
}

func (v *Vitals) GetCol(offset int) int {
	switch true {
	case offset < 0:
		return 0
	case offset > v.length:
		return 0
	default:
		return v.col[offset]
	}
}

func (v *Vitals) GetChar(offset int) int {
	switch true {
	case offset < 0:
		return -1
	case offset > v.length:
		return v.char[v.length] + offset - v.length
	default:
		return v.char[offset]
	}
}

func (v *Vitals) LineChar(offset int) [2]int {
	return [2]int{v.GetLine(offset), v.GetCol(offset)}
}

func (v *Vitals) MakeLoc(start int, stop int) LocationSpan {
	return LocationSpan{
		Start: v.LineChar(start),
		End:   v.LineChar(stop),
	}
}

// prepare ast for consumption by semantic merge
func (v *Vitals) CleanNode(node Node, pEnd int, nStart int) *Node {
	// extend back to just after the previous Node
	node.Span[0] = pEnd + 1
	// extend up to the end of line, or the next Node
	nextLine := v.lines[v.GetLine(node.Span[1])+1]
	if nextLine < nStart {
		node.Span[1] = nextLine - 1
	} else {
		node.Span[1] = nStart - 1
	}
	// make the location span to go along with this
	node.LocationSpan = v.MakeLoc(node.Span[0], node.Span[1])

	if len(node.Children) == 0 {
		return &node
	}

	// FIXME this assumes HeaderSpan and FooterSpan are set
	node.HeaderSpan[0] = node.Span[0]

	// extend header to the end of line, or the first child Node
	nextLine = v.lines[v.GetLine(node.HeaderSpan[1])+1]
	if nextLine < node.Children[0].Span[0] {
		node.HeaderSpan[1] = nextLine - 1
	} else {
		node.HeaderSpan[1] = node.Children[0].Span[0] - 1
	}

	node.FooterSpan[1] = node.Span[1]
	pcEnd := node.HeaderSpan[1]
	for i, child := range node.Children {
		var ncStart int
		if i == len(node.Children)-1 {
			ncStart = node.FooterSpan[0]
		} else {
			ncStart = node.Children[i+1].Span[0]
		}
		clean := v.CleanNode(child, pcEnd, ncStart)
		pcEnd = child.Span[1]
		if clean == nil {
			return nil
		}
		node.Children[i] = *clean
	}
	// extend footer back to the end of the last child Node
	node.FooterSpan[0] = node.Children[len(node.Children)-1].Span[1] + 1

	return &node
}

// prepare ast for consumption by semantic merge
func (v *Vitals) CleanFile(file *File) *File {
	file.LocationSpan = v.MakeLoc(0, v.length-1)

	// can't have an empty File unless it's all empty
	if v.length != 0 && len(file.Children) == 0 {
		file.Children = []Node{
			{
				Kind: "raw",
				Span: &[2]int{0, v.length},
			},
		}
	}

	// insert the header if necessary
	if v.length != 0 && file.Children[0].Span[0] != 0 {
		file.Children = append([]Node{{
			Kind: "header",
			Span: &[2]int{0, file.Children[0].Span[0] - 1},
		}}, file.Children...)
	}

	pEnd := -1
	for i, child := range file.Children {
		var nStart int
		if i == len(file.Children)-1 {
			nStart = v.length
		} else {
			nStart = file.Children[i+1].Span[0]
		}
		clean := v.CleanNode(child, pEnd, nStart)
		pEnd = child.Span[1]
		if clean == nil {
			return nil
		}
		file.Children[i] = *clean
	}

	switch file.Numbering {
	case NumberingBytes:
		file.FooterSpan = [2]int{pEnd + 1, v.length - 1}
	case NumberingUTF16:
		file.FooterSpan = [2]int{pEnd + 1, v.GetChar(v.length) - 1}
	}
	return v.convertSpans(file)
}

func (v *Vitals) convertSpan(span *[2]int) *[2]int {
	if span != nil {
		span[0] = v.GetChar(span[0])
		span[1] = v.GetChar(span[1])
	}
	return span
}

func (v *Vitals) convertNodeSpans(node Node) Node {
	node.HeaderSpan = v.convertSpan(node.HeaderSpan)
	node.Span = v.convertSpan(node.Span)
	node.FooterSpan = v.convertSpan(node.FooterSpan)

	for i, child := range node.Children {
		node.Children[i] = v.convertNodeSpans(child)
	}
	return node
}

func (v *Vitals) convertSpans(node *File) *File {
	if node.Numbering == NumberingUTF16 {
		return node
	}
	node.FooterSpan = *v.convertSpan(&node.FooterSpan)
	for i, child := range node.Children {
		node.Children[i] = v.convertNodeSpans(child)
	}
	for i := range node.ParsingError {
		node.ParsingError[i].Location = v.LineChar(node.ParsingError[i].Position)
	}
	node.Numbering = NumberingUTF16
	return node
}
