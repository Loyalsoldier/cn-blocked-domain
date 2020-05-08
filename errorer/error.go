package errorer

// CheckError panics runtime error
func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}
