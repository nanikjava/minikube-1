package cmd

import (
	"bytes"
	"fmt"
	"github.com/docker/machine/libmachine/state"
	"github.com/jroimartin/gocui"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/kubeconfig"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/out"
	"log"
	"os"
	"sync"
	"text/template"
	"time"
)


var g *gocui.Gui

func init() {
	g, _ = gocui.NewGui(gocui.OutputNormal)
}

var mwindows = []Window{
	Window{x: 0, y: 0, w:40, h:6, header: "Status", body: ``, refresh: 20 ,
		refreshFN: nil },
	//Window{x: 41, y: 0, w: 120, h:6 ,header: "Profile", body:`
	//		minikube directory   : %s
	//		current profile name : %s
	//		current machine size : %s
	//		`, refresh: 20},
	//Window{x: 0, y: 8, w: 120, h: 22 ,header: "Pods", body:`
	//		Pods
   //----
	//		%s
	//		`, refresh: 20},
}

func refreshstatus() {

	defer wg.Done()

	for {
		select {
		case <-done:
			return
		case <-time.After(20 * time.Second):
			g.Update(func(g *gocui.Gui) error {
				v, err := g.View("Status")

				if (err!=nil) {
					os.Exit(-2)
				}

				s:=status()

				v.BgColor = gocui.ColorYellow
				v.Clear()

				fmt.Fprintln(v,printKubeletStatus(s)	 )
				return nil
			})
		}
	}
}


func printKubeletStatus( status Status) (string) {
	tmpl, err := template.New("status").Parse(statusFormat)
	if err != nil {
		exit.WithError("Error creating status template", err)
	}

	var buff bytes.Buffer
	err = tmpl.Execute(&buff, status)
	val := buff.String()
	if err != nil {
		exit.WithError("Error executing status template", err)
	}
	if status.Kubeconfig == KubeconfigStatus.Misconfigured {
		val = "Misconfigured"
	}
	return val
}


var (
	done = make(chan struct{})
	wg   sync.WaitGroup

	mu  sync.Mutex // protects ctr
	ctr = 0
)


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

func UIMain() {
	defer g.Close()

	g.Highlight = true
	g.SelFgColor = gocui.ColorRed

	window  := Window{x: 0, y: 0, w:20, h:20, header: "Status", refresh: 20}
	g.SetManagerFunc(window.Layout)

	for i := 0; i < 1; i++ {
		wg.Add(1)
		go refreshstatus()
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func status() (Status) {
	// get status
	api, err := machine.NewAPIClient()
	if err != nil {
		exit.WithCodeT(exit.Unavailable, "Error getting client: {{.error}}", out.V{"error": err})
	}
	defer api.Close()

	hostSt, err := cluster.GetHostStatus(api, "minikube")
	if err != nil {
		exit.WithError("Error getting host status", err)
	}

	kubeletSt := state.None.String()
	kubeconfigSt := state.None.String()
	apiserverSt := state.None.String()

	var returnCode = 0


	if hostSt == state.Running.String() {
		clusterBootstrapper, err := getClusterBootstrapper(api, "kubeadm")
		if err != nil {
			exit.WithError("Error getting bootstrapper", err)
		}
		kubeletSt, err = clusterBootstrapper.GetKubeletStatus()
		if err != nil {
			//fmt.Println("kubelet err: %v", err)
			returnCode |= clusterNotRunningStatusFlag
		} else if kubeletSt != state.Running.String() {
			returnCode |= clusterNotRunningStatusFlag
		}

		ip, err := cluster.GetHostDriverIP(api, "minikube")
		if err != nil {
			//fmt.Println("Error host driver ip status:", err)
		}

		apiserverPort, err := kubeconfig.Port("minikube")
		if err != nil {
			// Fallback to presuming default apiserver port
			apiserverPort = constants.APIServerPort
		}

		apiserverSt, err = clusterBootstrapper.GetAPIServerStatus(ip, apiserverPort)
		if err != nil {
			//fmt.Println("Error apiserver status:", err)
		} else if apiserverSt != state.Running.String() {
			returnCode |= clusterNotRunningStatusFlag
		}

		ks, err := kubeconfig.IsClusterInConfig(ip, "minikube")
		if err != nil {
			//fmt.Println("Error kubeconfig status:", err)
		}
		if ks {
			kubeconfigSt = KubeconfigStatus.Configured
		} else {
			kubeconfigSt = KubeconfigStatus.Misconfigured
			returnCode |= k8sNotRunningStatusFlag
		}
	} else {
		returnCode |= minikubeNotRunningStatusFlag
	}

	status := Status{
		Host:       hostSt,
		Kubelet:    kubeletSt,
		APIServer:  apiserverSt,
		Kubeconfig: kubeconfigSt,
	}
	return status
}


