package launcher

import (
	"fmt"
	"strings"
)

type fakeUI struct {
	yes     bool
	answers []string
	asked   []Prompt
	said    []string
	notes   []string
}

func (u *fakeUI) Say(a ...any) { u.said = append(u.said, strings.TrimSuffix(fmt.Sprintln(a...), "\n")) }

func (u *fakeUI) Sayf(format string, a ...any) { u.said = append(u.said, fmt.Sprintf(format, a...)) }

func (u *fakeUI) Yes(string, bool) bool { return u.yes }

func (u *fakeUI) Notify(text string) { u.notes = append(u.notes, text) }

func (u *fakeUI) Ask(p Prompt) (string, bool) {
	u.asked = append(u.asked, p)
	if len(u.answers) == 0 {
		return "", false
	}
	answer := u.answers[0]
	u.answers = u.answers[1:]
	return answer, true
}

func (u *fakeUI) VKSignIn(VKSignIn) func() { return func() {} }

func (u *fakeUI) Foreground() {}
