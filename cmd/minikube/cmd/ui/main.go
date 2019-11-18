package ui

import (
	"fmt"
	"github.com/jroimartin/gocui"
	"k8s.io/minikube/pkg/minikube/machine"
	"log"
)


var g *gocui.Gui

func init() {
	g, _ = gocui.NewGui(gocui.OutputNormal)
}

var mwindows = []Window{
	Window{x: 0, y: 0, w:40, h:6, header: "Status", body: `

			apiserver : %s
			kubelet   : %s
			`, refresh: 20},
	Window{x: 41, y: 0, w: 120, h:6 ,header: "Profile", body:`
			minikube directory   : %s
			current profile name : %s
			current machine size : %s
			`, refresh: 20},
	Window{x: 0, y: 8, w: 120, h: 22 ,header: "Pods", body:`
			Pods
   ----
			%s
			`, refresh: 20},
}

type Window struct {
	x,y         int				// x,y position
	w,h         int				// w,h width and height
	header      string			// text header
	body        string			// body
	refresh     int				// content refresh time (-1 no refresh)
	refreshFN   func()
}

func newView(x,y,w,h int, name string, body string) error {
	v, err := g.SetView(name, x, y, w, h)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Wrap = true
		fmt.Fprintln(v,  name)
		fmt.Fprint(v, body)

	}
	if _, err := g.SetCurrentView(name); err != nil {
		return err
	}
	return nil
}

func (w *Window) Layout(g *gocui.Gui) error {
	for _, w := range mwindows{
		newView(w.x,w.y,w.w,w.h,w.header,w.body)
	}

	return nil
}

func Main() {
	defer g.Close()

	g.Highlight = true
	g.SelFgColor = gocui.ColorRed

	window  := Window{x: 0, y: 0, w:20, h:20, header: "Status", refresh: 20}
	g.SetManagerFunc(window.Layout)


	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, toggleButton); err != nil {
		log.Panicln(err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}


	m, _ := machine.NewAPIClient()
	defer m.Close()

}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func toggleButton(g *gocui.Gui, v *gocui.View) error {
	nextview := "butdown"
	if v != nil && v.Name() == "butdown" {
		nextview = "butup"
	}
	_, err := g.SetCurrentView(nextview)
	return err
}


