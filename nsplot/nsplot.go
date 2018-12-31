package nsplot

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/gkawamoto/k8s-designer/dependency"
	"github.com/gkawamoto/k8s-designer/ui"
)

// PlotHandler ?
type PlotHandler struct {
	title  string
	window *ui.Window
	target string
	graph  *dependency.Graph
}

// NewPlotHandler ?
func NewPlotHandler(w *ui.Window, target string) (*PlotHandler, error) {
	var result = &PlotHandler{
		window: w,
		target: target,
	}
	var absTarget string
	var err error
	absTarget, err = filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	result.title = filepath.Base(absTarget)
	result.graph, err = dependency.BuildGraph(target)
	if err != nil {
		return nil, err
	}
	w.Register("ready", result.readyHandler)
	return result, nil
}

func (p *PlotHandler) readyHandler(data []byte) {
	var err error
	var graph *dependency.Graph
	graph, err = dependency.BuildGraph(p.target)
	if err != nil {
		log.Fatal(err)
	}
	var e *dependency.Entity
	for _, e = range graph.Entities() {
		var name = fmt.Sprintf("%s (%s)", e.Metadata.Name, e.Kind)
		p.window.AddNode(e.ID, name, ui.KubernetesKindToNodeKind(e.Kind))
	}
	var from, to int
	for from, to = range graph.References() {
		p.window.AddEdge(from, to)
	}
	p.window.SetTitle(p.title)
	p.window.Refresh()
}

// Run ?
func (p *PlotHandler) Run() {
	p.window.Run()
}
