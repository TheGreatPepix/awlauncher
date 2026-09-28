package launcher

import (
	"strconv"
	"sync"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
)

type Operation struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Game    bool   `json:"game,omitempty"`
	Account string `json:"account,omitempty"`
}

func AccountOp(title string, acc config.Account) Operation {
	return Operation{Title: title, Account: strconv.FormatInt(acc.UserID, 10)}
}

type Ops struct {
	mu   sync.Mutex
	list []Operation
	next int
}

func (o *Ops) Begin(op *Operation) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, other := range o.list {
		if (op.Game && other.Game) || (op.Account != "" && op.Account == other.Account) {
			return "Wait until “" + other.Title + "” finishes"
		}
	}
	o.next++
	op.ID = o.next
	o.list = append(o.list, *op)
	return ""
}

func (o *Ops) Finish(id int) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, op := range o.list {
		if op.ID == id {
			o.list = append(o.list[:i], o.list[i+1:]...)
			break
		}
	}
	return len(o.list)
}

func (o *Ops) GameBusy() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, op := range o.list {
		if op.Game {
			return true
		}
	}
	return false
}

func (o *Ops) List() []Operation {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]Operation{}, o.list...)
}
