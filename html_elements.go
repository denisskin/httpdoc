package httpdoc

type HTMLElements []*HTMLElement

func (ee HTMLElements) Eq(num int) *HTMLElement {
	if num < len(ee) {
		return ee[num]
	}
	return nil
}

func (ee HTMLElements) GetByID(id string) *HTMLElement {
	for _, e := range ee {
		if e.Attributes["id"] == id {
			return e
		}
	}
	return nil
}

func (ee HTMLElements) GetByName(id string) *HTMLElement {
	for _, e := range ee {
		if e.Attributes["name"] == id || e.Attributes["id"] == id {
			return e
		}
	}
	return nil
}

func (ee HTMLElements) GetByAttr(name, val string) (res HTMLElements) {
	for _, e := range ee {
		if e.Attributes[name] == val {
			res = append(res, e)
		}
	}
	return
}
