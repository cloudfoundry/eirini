package util

func TruncateString(str string, num int) string {
	if len(str) > num {
		return str[0:num]
	}
	return str
}
