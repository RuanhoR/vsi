package value

import (
	"errors"
	"strings"
)

type VsiString struct {
	Value string
}

func (v VsiString) Slice(start *VsiNumber, end *VsiNumber) (VsiString, error) {
	runes := []rune(v.Value)
	startIdx := 0
	endIdx := len(runes)
	if start != nil {
		startIdx = start.Value
	}
	if end != nil {
		endIdx = end.Value
	}
	if startIdx > len(runes) || endIdx > len(runes) {
		return VsiString{
			Value: "",
		}, errors.New("[vsi]: slice start or end can't large of string")
	}
	return VsiString{
		Value: string(runes[startIdx:endIdx]),
	}, nil
}
func (v VsiString) Length() VsiNumber {
	return VsiNumber{
		Value: len([]rune(v.Value)),
	}
}
func (v VsiString) Split(str VsiString) []VsiString {
	result := strings.Split(v.Value, str.Value)
	re := []VsiString{}
	for index := 0; index <= len(result); index++ {
		re = append(re, VsiString{
			Value: result[index],
		})
	}
	return re
}
