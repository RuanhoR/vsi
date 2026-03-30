package value

// VsiObject 表示 VSI 中的对象类型
type VsiObject struct {
	Proto map[string]interface{} // 对象属性
	Const bool                   // 是否不可变
}

func (v VsiObject) toString() VsiString {
	return VsiString{
		Value: "[Object]",
	}
}

// CreateObject 创建一个新的可变 VsiObject
func CreateObject() *VsiObject {
	return &VsiObject{
		Proto: make(map[string]interface{}),
		Const: false,
	}
}

// CreateConstObject 创建一个不可变的 VsiObject
func CreateConstObject() *VsiObject {
	return &VsiObject{
		Proto: make(map[string]interface{}),
		Const: true,
	}
}

// Freeze 将对象设为不可变
func (v *VsiObject) Freeze() {
	v.Const = true
}

// IsFrozen 检查对象是否不可变
func (v *VsiObject) IsFrozen() bool {
	return v.Const
}

// SetProperty 设置属性，如果对象不可变则返回错误
func (v *VsiObject) SetProperty(key string, value interface{}) error {
	if v.Const {
		return &ImmutableError{Type: "Object"}
	}
	v.Proto[key] = value
	return nil
}

// ImmutableError 不可变错误
type ImmutableError struct {
	Type string
}

func (e *ImmutableError) Error() string {
	return "Cannot modify immutable " + e.Type
}
