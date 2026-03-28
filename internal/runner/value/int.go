package value

import "strconv"

type VsiNumber struct {
	Value int
}

func (v VsiNumber) toString() VsiString {
	return VsiString{
		Value: strconv.Itoa(v.Value),
	}
}
