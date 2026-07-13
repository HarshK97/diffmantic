package actions

import (
	"fmt"
	"io"
	"os"
)

type EditScript struct {
	actions []Action
}

func NewEditScript() *EditScript {
	return &EditScript{}
}

func (es *EditScript) Add(a Action) {
	es.actions = append(es.actions, a)
}

func (es *EditScript) Size() int {
	return len(es.actions)
}

func (es *EditScript) Actions() []Action {
	return es.actions
}


func PrintActions(es *EditScript) {
	FprintActions(os.Stdout, es)
}

func FprintActions(w io.Writer, es *EditScript) {
	if es == nil || es.Size() == 0 {
		fmt.Fprintln(w, "(no edit actions)")
		return
	}
	fmt.Fprintf(w, "\n%-4s  %-14s  %-25s %-20s  %-25s  %s\n",
		"#", "Op", "Node Type", "Node Label", "Parent Type", "Details")
	fmt.Fprintln(w, "──────────────────────────────────────────────────────────────────────────────────────────────────────────")
	for i, a := range es.Actions() {
		nodeType := ""
		nodeLabel := "-"
		if a.Node != nil {
			nodeType = a.Node.Type
			if a.Node.Label != "" {
				nodeLabel = a.Node.Label
			}
		}

		parentType := "-"
		detail := ""

		switch a.Type {
		case Insert:
			if a.Parent != nil {
				parentType = a.Parent.Type
			}
			detail = fmt.Sprintf("pos=%d", a.Position)
		case Delete:
		case Update:
			detail = fmt.Sprintf("val=%q", a.Value)
		case Move:
			if a.Parent != nil {
				parentType = a.Parent.Type
			}
			detail = fmt.Sprintf("pos=%d", a.Position)
		}

		if a.Subtree {
			detail += " [subtree]"
		}

		op := a.Type.String()
		fmt.Fprintf(w, "%-4d  %-14s  %-25s %-20s  %-25s  %s\n",
			i+1, op, nodeType, nodeLabel, parentType, detail)
	}
	fmt.Fprintf(w, "\nTotal actions: %d\n", es.Size())
}
