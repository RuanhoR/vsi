package value

type VsiObject struct {
	Proto map[string]interface{}
}

func (v VsiObject) toString() VsiString {
	return VsiString{
		Value: "[Object]",
	}
}

// CreateObject constructs a new VsiObject with an exported Proto map
func CreateObject() *VsiObject {
	return &VsiObject{
		Proto: make(map[string]interface{}),
	}
}
