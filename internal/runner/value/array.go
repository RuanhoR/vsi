package value

// VsiArray 表示 VSI 中的数组类型
type VsiArray struct {
	Items []interface{} // 数组元素
	Const bool          // 是否不可变
}

func (a VsiArray) toString() VsiString {
	return VsiString{Value: "[Array]"}
}

func (a VsiArray) Length() VsiNumber {
	return VsiNumber{Value: len(a.Items)}
}

// CreateArray 创建一个新的可变 VsiArray
func CreateArray(items []interface{}) *VsiArray {
	if items == nil {
		items = []interface{}{}
	}
	return &VsiArray{
		Items: items,
		Const: false,
	}
}

// CreateConstArray 创建一个不可变的 VsiArray
func CreateConstArray(items []interface{}) *VsiArray {
	if items == nil {
		items = []interface{}{}
	}
	return &VsiArray{
		Items: items,
		Const: true,
	}
}

// Freeze 将数组设为不可变
func (a *VsiArray) Freeze() {
	a.Const = true
}

// IsFrozen 检查数组是否不可变
func (a *VsiArray) IsFrozen() bool {
	return a.Const
}

// SetItem 设置指定索引的元素，如果数组不可变则返回错误
func (a *VsiArray) SetItem(index int, value interface{}) error {
	if a.Const {
		return &ImmutableError{Type: "Array"}
	}
	if index >= 0 && index < len(a.Items) {
		a.Items[index] = value
	}
	return nil
}

// Push 添加元素到数组末尾，如果数组不可变则返回错误
func (a *VsiArray) Push(value interface{}) error {
	if a.Const {
		return &ImmutableError{Type: "Array"}
	}
	a.Items = append(a.Items, value)
	return nil
}
