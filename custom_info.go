package libphonelab

type CustomInfo struct {
	context string
	_type   string
}

func (c *CustomInfo) Context() string {
	return c.context
}

func (c *CustomInfo) Type() string {
	return c._type
}
