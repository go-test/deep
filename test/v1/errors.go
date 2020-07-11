package deeptest

type Error struct{}

func (e Error) Error() string {
	return ""
}
