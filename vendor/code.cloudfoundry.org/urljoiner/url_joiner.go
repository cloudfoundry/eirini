package urljoiner // import "code.cloudfoundry.org/urljoiner"

func Join(base string, components ...string) string {
	final := base
	for _, component := range components {
		if len(component) == 0 {
			continue
		}

		lastFinalChar := ""
		if len(final) > 0 {
			lastFinalChar = string(final[len(final)-1])
		}

		if lastFinalChar == "/" && string(component[0]) == "/" {
			final = final[:len(final)-1] + component
		} else if lastFinalChar == "/" || string(component[0]) == "/" {
			final = final + component
		} else {
			final = final + "/" + component
		}
	}

	return final
}
