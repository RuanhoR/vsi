package value

type VsiArray struct {
	Items []interface{}
}

func (a VsiArray) toString() VsiString {
	return VsiString{Value: "[Array]"}
}

func (a VsiArray) Length() VsiNumber {
	return VsiNumber{Value: len(a.Items)}
}

func createArray(items []interface{}) *VsiArray {
	if items == nil {
		items = []interface{}{}
	}
	return &VsiArray{Items: items}
}

// CreateArray constructs a new VsiArray (exported)
func CreateArray(items []interface{}) *VsiArray {
	if items == nil {
		items = []interface{}{}
	}
	return &VsiArray{Items: items}
}
