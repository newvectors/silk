package gui

// AccessibleChildren adapts []IWidget to the dependency-free a11y walker. Go
// slice return types are invariant: Children() []IWidget does not implement
// Children() []interface{}, so without this bridge only the root was dumped.
func (w *Widget) AccessibleChildren() []interface{} {
	children := w.Children()
	out := make([]interface{}, 0, len(children))
	for _, child := range children {
		out = append(out, child)
	}
	return out
}
