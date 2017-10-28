package httpdoc

type HTMLElements []*HTMLElement

func (ee HTMLElements) String() (s string) {
	for i, e := range ee {
		if i > 0 {
			s += "\n"
		}
		s += e.String()
	}
	return
}

func (ee HTMLElements) Eq(num int) *HTMLElement {
	if num < len(ee) {
		return ee[num]
	}
	return nil
}

func (ee HTMLElements) First() *HTMLElement {
	return ee.Eq(0)
}

func (ee HTMLElements) Last() *HTMLElement {
	if len(ee) > 0 {
		return ee[len(ee)-1]
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

func (ee HTMLElements) FilterByAttr(attrName string) (res HTMLElements) {
	for _, e := range ee {
		if _, ok := e.Attributes[attrName]; ok {
			res = append(res, e)
		}
	}
	return
}

func (ee HTMLElements) FilterByAttrValue(attrName, attrValue string) (res HTMLElements) {
	for _, e := range ee {
		if e.Attributes[attrName] == attrValue {
			res = append(res, e)
		}
	}
	return
}
