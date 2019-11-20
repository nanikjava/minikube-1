package cmd

import (
	"bytes"
	"fmt"
	"github.com/docker/machine/libmachine/state"
	"github.com/jroimartin/gocui"
	"github.com/spf13/viper"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/kubeconfig"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/out"
	"log"
	"text/template"
	"time"
)

type wmatrix struct {
	x,y int
}

type coordinate struct {
	p1, p2 int
}


var (
	done = make(chan struct{})
	finish = make(chan struct{})
	g *gocui.Gui
	listprofile  []string
	wmatrixarray map[string]wmatrix
	X []coordinate
	Y []coordinate
)


func init() {
	// X
	// -----
	// 0,254
	// 256,510
	// 512,766
	// 768,1024
	X =  []coordinate{
		{ p1: 0, p2: 254, },
		{ p1: 265, p2: 510, },
		{ p1: 512, p2: 766, },
		{ p1: 768, p2: 1024, },
	}

	// Y
	// ----
	// 0,190
	// 192,382
	// 384,574
	// 576,768
	Y =  []coordinate{
		{ p1: 0, p2: 190, },
		{ p1: 192, p2: 382, },
		{ p1: 384, p2: 574, },
		{ p1: 576, p2: 768, },
	}
}

func refreshprofilewindow(profilename string) {
	for {
		select {
		case <-time.After(10 * time.Second):
			getAllProfiles()

			//check to make sure that the view still exist
			if err,_ := g.View(profilename); err == nil {
				return
			}

			g.Update(func(g *gocui.Gui) error {
				v, err := g.View(profilename)

				if (err!=nil) {
					return nil
				}

				v.Clear()
				fmt.Fprintln(v,printKubeletStatus(checkStatus(profilename))	 )
				return nil
			})
		}
	}
}

func checkStatus(profilename string) (Status) {
	// get status
	api, err := machine.NewAPIClient()
	if err != nil {
		exit.WithCodeT(exit.Unavailable, "Error getting client: {{.error}}", out.V{"error": err})
	}
	defer api.Close()

	var status Status

	hostSt, err := cluster.GetHostStatus(api, profilename)
	if err != nil {
		//exit.WithError("Error getting host status", err)
	}

	kubeletSt := state.None.String()
	kubeconfigSt := state.None.String()
	apiserverSt := state.None.String()

	var returnCode = 0

	if hostSt == state.Running.String() {
		viper.Set(config.MachineProfile,profilename)
		clusterBootstrapper, err := getClusterBootstrapper(api, "kubeadm")
		if err != nil {
			//exit.WithError("Error getting bootstrapper", err)
		}

		if (clusterBootstrapper != nil) {
			kubeletSt, err = clusterBootstrapper.GetKubeletStatus()
			if err != nil {
				//fmt.Println("kubelet err: %v", err)
				returnCode |= clusterNotRunningStatusFlag
			} else if kubeletSt != state.Running.String() {
				returnCode |= clusterNotRunningStatusFlag
			}
		} else {
			returnCode |= clusterNotRunningStatusFlag
		}

		ip, err := cluster.GetHostDriverIP(api, profilename)
		if err != nil {
			//fmt.Println("Error host driver ip status:", err)
		}

		apiserverPort, err := kubeconfig.Port(profilename)
		if err != nil {
			// Fallback to presuming default apiserver port
			apiserverPort = constants.APIServerPort
		}

		if (clusterBootstrapper != nil) {
			apiserverSt, err = clusterBootstrapper.GetAPIServerStatus(ip, apiserverPort)
			if err != nil {
				//fmt.Println("Error apiserver status:", err)
			} else if apiserverSt != state.Running.String() {
				returnCode |= clusterNotRunningStatusFlag
			}
		} else {
			returnCode |= clusterNotRunningStatusFlag
		}

		ks, err := kubeconfig.IsClusterInConfig(ip, profilename)
		if ks {
			kubeconfigSt = KubeconfigStatus.Configured
		} else {
			kubeconfigSt = KubeconfigStatus.Misconfigured
			returnCode |= k8sNotRunningStatusFlag
		}
	} else {
		returnCode |= minikubeNotRunningStatusFlag
	}

	status = Status{
		Host:       hostSt,
		Kubelet:    kubeletSt,
		APIServer:  apiserverSt,
		Kubeconfig: kubeconfigSt,
	}

	return status
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

func UIMain() {
	g, _ = gocui.NewGui(gocui.OutputNormal)

	defer g.Close()

	g.Highlight = true
	g.SelFgColor = gocui.ColorDefault

	getAllProfiles()
	g.SetManagerFunc(Layout)


	if err := keybindings(); err != nil {
		log.Panicln(err)
	}

	go checkViews()
	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}


}

func checkViews() {
	for {
		select {
		case <-finish:
			return
		case <-time.After(10 * time.Second):
			getAllProfiles()
			// delete view if it does not exist anymore
			vfound := false
			recalc := false

			if len(g.Views())>0 {
				for _, v := range g.Views() {
					vfound = false
					for _, p:= range listprofile {
						if v.Name() == p {
							vfound = true
						}
					}

					if (! vfound) {
						g.DeleteView(v.Name())
						g.Update(func(gui *gocui.Gui) error {
							return nil
						})
						recalc = true
						break;
					}
				}
			}

			// create view if there is a new profile
			var viewName string

			getAllProfiles()
			for _, p:= range listprofile{
				createNewView := true
				viewName = p
				for _, v := range g.Views()  {
					if p == v.Name() {
						createNewView = false
						break
					}
				}

				if createNewView{
					v, _ := g.SetView(viewName, 0,0,1,1)
					v.Wrap = true
					v.Title = viewName
					go refreshprofilewindow(viewName)
					recalc=true
				}
			}




			var x int = -1
			var y int
			var xwidth = 50
			var ywidth = 25

			if  (recalc) {
				if len(g.Views()) > 0 {
					for _, v := range g.Views() {
						if (x > 4) {
							x = 0
							y++
						} else {
							x++
						}

						if (y > 4) {
							y = 0
							x = 0
						}

						startX := x * xwidth
						startY := y * ywidth

						endX := startX + (xwidth - 2)
						endY := startY + (ywidth - 2)

						name := v.Name()
						g.SetView(name, startX, startY, endX, endY)

					}
				}
			}
		}
	}
}

func Layout(gui *gocui.Gui) error {
	//x := 0
	//y := 0
	//wsize := 30
	//hsize := 30

	// cross check the views with the profile list
	// any view that is not available in the profile list
	// remove that view
	getAllProfiles()
	//if len(listprofile) > 0 {
	//	for _, profile  := range listprofile {
	//		// check if the view exist....
	//		v,err :=  g.View(profile)
	//		// ... since err is nil that means view does not exist
	//		if (v==nil) {
	//			// ... create the view
	//			v, err = g.SetView(profile, x, y, x+wsize, y+hsize)
	//
	//			if err != nil {
	//				if err != gocui.ErrUnknownView {
	//					log.Panicln(err)
	//				}
	//				v.Wrap = true
	//				v.Title = profile
	//
	//				go refreshprofilewindow(profile)
	//				x += wsize+1
	//			}
	//		}
	//	}
	//}
	return nil
}

func keybindings() error {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	//<- finish
	return gocui.ErrQuit
}

// get all local profiles
// only interested in valid profiles
func getAllProfiles() {
	listprofile= listprofile[:0]
	api, err := machine.NewAPIClient()
	if err != nil {
		exit.WithCodeT(exit.Unavailable, "Error getting client: {{.error}}", out.V{"error": err})
	}
	defer api.Close()

	validProfiles, _, err := config.ListProfiles()

	if len(validProfiles) != 0 || err != nil {
		for _, p := range validProfiles {
			listprofile = append(listprofile, p.Name)
		}
	}
}


