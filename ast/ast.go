package ast

type LocationSpan struct {
	Start [2]int `json:"start"`
	End   [2]int `json:"end"`
}

type ParsingError struct {
	Location []int  `json:"location"`
	Message  string `json:"message"`
}

type File struct {
	Kind                  string        `json:"type"`
	Name                  string        `json:"name"`
	LocationSpan          LocationSpan  `json:"locationSpan"`
	FooterSpan            [2]int        `json:"footerSpan"`
	ParsingErrorsDetected bool          `json:"parsingErrorsDetected"`
	Children              []Node        `json:"children"`
	ParsingError          *ParsingError `json:"parsingError,omitempty"`
}

type Node struct {
	Kind         string       `json:"type"`
	Name         string       `json:"name"`
	LocationSpan LocationSpan `json:"locationSpan"`
	Span         *[2]int       `json:"span,omitempty"`       // only in leaf nodes
	HeaderSpan   *[2]int       `json:"headerSpan,omitempty"` // \
	FooterSpan   *[2]int       `json:"footerSpan,omitempty"` //  } only in container nodes
	Children     []Node       `json:"children,omitempty"`   // /
}

func MakeLines(buf []byte) []int {
	lines := []int{}
	lines = append(lines, -1)
	lines = append(lines, 0)
	for i := 0; i < len(buf); i++ {
		if buf[i] == '\n' {
			lines = append(lines, i+1)
		}
	}
	lines = append(lines, len(buf))
	return lines
}

func GetLine(lines []int, offset int) int {
	left := 0
	right := len(lines)
	for right > left + 1 {
		mid := (left + right) / 2
		switch true {
		case offset < lines[mid]:
			right = mid
		case lines[mid] < offset:
			left = mid
		default:
			return mid
		}
	}
	return left
}

func LineChar(lines []int, offset int, shift int) [2]int {
	line := GetLine(lines, offset)
	return [2]int{line, offset - lines[line] + shift}
}

func MakeLoc(lines []int, start int, stop int) LocationSpan {
	return LocationSpan{
		Start: LineChar(lines, start, 0),
		End:   LineChar(lines, stop + 1, -1),
	}
}

// prepare ast for consumption by semantic merge
func CleanNode(node Node, lines []int, pEnd int, nStart int) *Node {
	// extend back to just after the previous Node
	node.Span[0] = pEnd + 1
	// extend up to the end of line, or the next Node
	nextLine := lines[GetLine(lines, node.Span[1]) + 1]
	if nextLine < nStart {
		node.Span[1] = nextLine - 1
	} else {
		node.Span[1] = nStart - 1
	}
	// make the location span to go along with this
	node.LocationSpan = MakeLoc(lines, node.Span[0], node.Span[1])

	if len(node.Children) == 0 {
		return &node
	}

	// FIXME this assumes HeaderSpan and FooterSpan are set
	node.HeaderSpan[0] = node.Span[0]

	// extend header to the end of line, or the first child Node
	nextLine = lines[GetLine(lines, node.HeaderSpan[1]) + 1]
	if nextLine < node.Children[0].Span[0] {
		node.HeaderSpan[1] = nextLine - 1
	} else {
		node.HeaderSpan[1] = node.Children[0].Span[0] - 1
	}

	node.FooterSpan[1] = node.Span[1]
	pcEnd := node.HeaderSpan[1]
	for i, child := range node.Children {
		var ncStart int
		if i == len(node.Children) - 1 {
			ncStart = node.FooterSpan[0]
		} else {
			ncStart = node.Children[i+1].Span[0]
		}
		clean := CleanNode(child, lines, pcEnd, ncStart)
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
func CleanFile(file *File, lines []int) *File {
	length := lines[len(lines)-1]
	file.LocationSpan = MakeLoc(lines, 0, length - 1)

	// can't have an empty File
	if len(file.Children) == 0 {
		file.Children = []Node{
			{
				Kind: "raw",
				Span: &[2]int{0, length},
			},
		};
	}

	// insert the header if necessary
	if file.Children[0].Span[0] != 0 {
		file.Children = append([]Node{{
			Kind: "header",
			Span: &[2]int{0, file.Children[0].Span[0] - 1},
		}}, file.Children...)
	}

	pEnd := -1
	for i, child := range file.Children {
		var nStart int
		if i == len(file.Children) - 1 {
			nStart = length
		} else {
			nStart = file.Children[i+1].Span[0]
		}
		clean := CleanNode(child, lines, pEnd, nStart)
		pEnd = child.Span[1]
		if clean == nil {
			return nil
		}
		file.Children[i] = *clean
	}

	file.FooterSpan = [2]int{file.Children[len(file.Children)-1].Span[1] + 1, length - 1};

	return file
}
