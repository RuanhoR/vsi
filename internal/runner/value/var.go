package value

type VsiVariable struct {
	Const bool
	Value interface{}
}

func createVariable(value interface{}) *VsiVariable {
	return &VsiVariable{
		Const: false,
		Value: value,
	}
}

// CreateVariable constructs a new VsiVariable (exported)
func CreateVariable(value interface{}) *VsiVariable {
	return &VsiVariable{
		Const: false,
		Value: value,
	}
}
