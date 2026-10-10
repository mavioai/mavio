package library

// named is a provider of any capability.
type named interface{ Name() string }

// ordered returns the providers a library uses, given its order of them:
// all of them when order is nil, else those order names, in its order.
func ordered[P named](all []P, order []string) []P {
	if order == nil {
		return all
	}
	out := make([]P, 0, len(order))
	for _, name := range order {
		for _, p := range all {
			if p.Name() == name {
				out = append(out, p)
				break
			}
		}
	}
	return out
}
