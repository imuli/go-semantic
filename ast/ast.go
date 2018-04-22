package ast

type LocationSpan struct {
	start []int
	end   []int
}

type ParsingError struct {
	location []int
	message  string
}

type File struct {
	kind                  string `json:"type"`
	name                  string
	locationSpan          LocationSpan
	footerSpan            []int
	parsingErrorsDetected bool
	children              []Node
	parsingError          *ParsingError `json:"parsingError,omitempty"`
}

type Node struct {
	kind         string `json:"type"`
	name         string
	locationSpan LocationSpan
	span         []int  `json:",omitempty"` // only in leaf nodes
	headerSpan   []int  `json:",omitempty"` // \
	footerSpan   []int  `json:",omitempty"` //  } only in container nodes
	children     []Node `json:",omitempty"` // /
}
