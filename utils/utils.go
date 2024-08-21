package utils

func HasItem(arr []int, item int) bool {
	for _, i := range arr {
		if i == item {
			return true
		}
	}
	return false
}
