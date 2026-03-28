package value

type VsiFunctionMetedata struct {
	Name       string
	Parameters []string
	Run        func(args []interface{}) (interface{}, error)
	Stack      []string
}
type VsiFunction struct {
	__metadata VsiFunctionMetedata
}

func (v VsiFunction) toString() VsiString {
	return VsiString{
		Value: "[Function]",
	}
}

// CreateFunction constructs a new VsiFunction
func CreateFunction(name string, params []string, run func(args []interface{}) (interface{}, error)) *VsiFunction {
	return &VsiFunction{
		__metadata: VsiFunctionMetedata{
			Name:       name,
			Parameters: params,
			Run:        run,
			Stack:      []string{},
		},
	}
}

// Call invokes the function with provided args
func (v *VsiFunction) Call(args []interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	if v.__metadata.Run == nil {
		return nil, nil
	}
	return v.__metadata.Run(args)
}
