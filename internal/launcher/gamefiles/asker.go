package gamefiles

import "errors"

var ErrDeclined = errors.New("quit")

type Asker interface {
	Say(a ...any)
	Sayf(format string, a ...any)
	Yes(question string, def bool) bool
}
