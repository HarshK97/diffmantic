package actions

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/HarshK97/diffmantic/internal/engine"
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


type jsonOutput struct {
	Matches []jsonMatch  `json:"matches"`
	Actions []jsonAction `json:"actions"`
}

type jsonMatch struct {
	Src  string `json:"src"`
	Dest string `json:"dest"`
}

type jsonAction struct {
	Action  string `json:"action"`
	Tree    string `json:"tree"`
	Parent  string `json:"parent,omitempty"`
	At      *int   `json:"at,omitempty"`
	Label   string `json:"label,omitempty"`
	Subtree *bool  `json:"subtree,omitempty"`
}

func ToJSON(es *EditScript, ms *engine.Mapping) ([]byte, error) {
	out := buildJSONOutput(es, ms)
	return json.MarshalIndent(out, "", "  ")
}

func WriteJSON(w io.Writer, es *EditScript, ms *engine.Mapping) error {
	out := buildJSONOutput(es, ms)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func PrintJSON(es *EditScript, ms *engine.Mapping) error {
	return WriteJSON(os.Stdout, es, ms)
}

func buildJSONOutput(es *EditScript, ms *engine.Mapping) jsonOutput {
	out := jsonOutput{
		Matches: make([]jsonMatch, 0, len(ms.Pairs)),
		Actions: make([]jsonAction, 0, es.Size()),
	}

	for _, p := range ms.Pairs {
		out.Matches = append(out.Matches, jsonMatch{
			Src:  NodeToString(p.Src),
			Dest: NodeToString(p.Dst),
		})
	}

	for _, a := range es.Actions() {
		ja := jsonAction{
			Action: a.Type.String(),
			Tree:   NodeToString(a.Node),
		}

		switch a.Type {
		case Insert, Move:
			if a.Parent != nil {
				ja.Parent = NodeToString(a.Parent)
			}
			pos := a.Position
			ja.At = &pos
		case Update:
			ja.Label = a.Value
		case Delete:
		}

		if a.Subtree {
			st := true
			ja.Subtree = &st
		}

		out.Actions = append(out.Actions, ja)
	}

	return out
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
