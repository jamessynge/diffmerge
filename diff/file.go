package diff

import (

)

type File interface {
	Path() string
	Body() []byte	
	GetLine(n LineNo) Line
}





