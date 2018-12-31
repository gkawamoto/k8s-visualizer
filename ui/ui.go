package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/rakyll/statik/fs"
	"github.com/zserge/webview"
)

// Window ?
type Window struct {
	view   webview.WebView
	server struct {
		listener net.Listener
		http     *http.Server
	}
	nodes        []node
	edges        []edge
	OnReady      func()
	errorHandler func(error)
	listeners    listenerMap
}

type listenerMap map[string]func([]byte)

type node struct {
	Label string   `json:"label"`
	ID    int      `json:"id"`
	Shape NodeKind `json:"shape"`
}

type edge struct {
	From int `json:"from"`
	To   int `json:"to"`
}
type payload struct {
	Method  string `json:"method"`
	Payload string `json:"payload"`
}

// NodeKind ?
type NodeKind string

const (
	// NodeKindIngress ?
	NodeKindIngress NodeKind = "dot"
	// NodeKindService ?
	NodeKindService NodeKind = "diamond"
	// NodeKindDeployment ?
	NodeKindDeployment NodeKind = "square"
	// NodeKindDaemonSet ?
	NodeKindDaemonSet NodeKind = "triangle"

	// NodeKindUnknown ?
	NodeKindUnknown NodeKind = "triangleDown"
)

// KubernetesKindToNodeKind ?
func KubernetesKindToNodeKind(kind string) NodeKind {
	switch kind {
	case "Ingress":
		return NodeKindIngress
	case "Service":
		return NodeKindService
	case "Deployment":
		return NodeKindDeployment
	case "DaemonSet":
		return NodeKindDaemonSet
	}
	return NodeKindUnknown
}

// New ?
func New(errorHandler func(error)) (*Window, error) {
	var result = Window{
		nodes:        []node{},
		edges:        []edge{},
		errorHandler: errorHandler,
		listeners:    listenerMap{},
	}
	result.nodes = []node{}
	var err = result.createHTTPServer()
	if err != nil {
		return nil, err
	}
	result.createWebView()
	return &result, nil
}

func (w *Window) createWebView() {
	var view = webview.New(webview.Settings{
		URL:       fmt.Sprintf("http://%s/public/index.html", w.server.listener.Addr().String()),
		Resizable: true,
		Debug:     true,
		Width:     800,
		Height:    600,
		ExternalInvokeCallback: func(view webview.WebView, data string) {
			var obj payload
			var err = json.Unmarshal([]byte(data), &obj)
			if err != nil {
				w.invokeError(fmt.Errorf("externalInvokeCallback: json: unmarshal: %s", err))
				return
			}
			w.invoke(obj.Method, obj.Payload)
		},
	})
	w.view = view
}

func (w *Window) invoke(method string, payload string) {
	var handler func([]byte)
	var ok bool
	handler, ok = w.listeners[method]
	if !ok {
		return
	}
	handler([]byte(payload))
}

// Register ?
func (w *Window) Register(event string, handler func([]byte)) {
	w.listeners[event] = handler
}

func (w *Window) invokeError(err error) {
	if w.errorHandler == nil {
		log.Println(fmt.Errorf("uncaught error: %s", err))
		return
	}
	w.errorHandler(err)
}

func (w *Window) createHTTPServer() error {
	var err error
	var listener net.Listener
	listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	var sfs http.FileSystem
	sfs, err = fs.New()
	if err != nil {
		return err
	}
	w.server.http = &http.Server{
		Handler: http.StripPrefix("/public/", http.FileServer(sfs)),
	}
	w.server.listener = listener
	return nil
}

// Eval ?
func (w *Window) Eval(content string) error {
	log.Println(content)
	return w.view.Eval(content)
}

// Terminate ?
func (w *Window) Terminate() {
	if w.view != nil {
		w.view.Terminate()
		w.view = nil
	}
	if w.server.http != nil {
		w.server.http.Close()
		w.server.http = nil
	}
}

// SetTitle ?
func (w *Window) SetTitle(title string) {
	w.view.SetTitle(title)
}

// Run ?
func (w *Window) Run() error {
	var errchan = make(chan error)
	go func() {
		errchan <- w.server.http.Serve(w.server.listener)
	}()
	w.view.Run()
	w.Terminate()
	return nil
}

// AddNode ?
func (w *Window) AddNode(id int, label string, kind NodeKind) {
	var obj = node{label, id, kind}
	w.nodes = append(w.nodes, obj)
}

// AddEdge ?
func (w *Window) AddEdge(from, to int) {
	var obj = edge{from, to}
	w.edges = append(w.edges, obj)
}

// Refresh ?
func (w *Window) Refresh() error {
	var err error
	var nodeString []byte
	nodeString, err = json.Marshal(&w.nodes)
	if err != nil {
		return fmt.Errorf("window: refresh: json: marshal: nodes: %s", err)
	}
	var edgeString []byte
	edgeString, err = json.Marshal(&w.edges)
	if err != nil {
		return fmt.Errorf("window: refresh: json: marshal: edges: %s", err)
	}
	err = w.Eval(fmt.Sprintf(`network.setData({nodes:new vis.DataSet(%s), edges:new vis.DataSet(%s)})`, string(nodeString), string(edgeString)))
	if err != nil {
		return fmt.Errorf("window: refresh: eval: %s", err)
	}
	return nil
}
