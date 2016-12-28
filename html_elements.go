package httpdoc

type HTMLElements []*HTMLElement

func (ee HTMLElements) Eq(num int) *HTMLElement {
	if num < len(ee) {
		return ee[num]
	}
	return nil
}
