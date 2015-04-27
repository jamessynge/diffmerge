package diff

import (

)

type File interface {
	Path() string
	Body() []byte	
	Line(n LineNo) Line

	// Range covering the entire file.
	FullRange() FileRange

}





