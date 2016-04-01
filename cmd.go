package main

import (
	"fmt"
	"os"

	"github.com/abema/kubetool/kube"
	"github.com/alecthomas/kingpin"
	"github.com/fatih/color"
)

var (
	red = color.New(color.FgRed).SprintfFunc()

	app       = kingpin.New("kubetool", "kubernetes bulk task executor.")
	verbose   = app.Flag("verbose", "Enable verbose log.").Short('v').Bool()
	namespace = app.Flag("namespace", "Target namespace. default is all namespaces").String()
	force     = app.Flag("yes", "Skip confirmation.").Short('y').Bool()

	// command info
	info = app.Command("info", "Print cluster & version info about cluster.").Alias("i")

	// command rc
	rc = app.Command("rc", "Print all rc.")

	// command pod
	pod   = app.Command("pod", "Print all pods").Alias("pods").Alias("po")
	podRC = pod.Flag("rc", "rc name for pod target").String()

	// command reload
	reload         = app.Command("reload", "Reload all pods in rc.")
	reloadName     = reload.Arg("name", "Name of target RC.").Required().String()
	reloadInterval = reload.Flag("interval", "Update interval seconds between pods.").Default("0").Int()
	reloadOne      = reload.Flag("1", "Reload only 1 pod").Bool()

	// command set version
	update          = app.Command("update", "Update image version of rc")
	updateName      = update.Arg("name", "Name of target RC.").Required().String()
	updateVersion   = update.Arg("version", "Version strings of image.").Required().String()
	updateReload    = update.Flag("reload", "Reload pods after update.").Bool()
	updateReloadOne = update.Flag("1", "Reload only 1 pod after update.").Short('1').Bool()
	updateContainer = update.Flag("container", "Target container name. Default is first container in defs.").Short('c').String()
	updateInterval  = update.Flag("interval", "Reloading interval after update.").Default("0").Int()

	fixVersion         = app.Command("fix-version", "Fix all pods to destroy all that has different version of RC ones.")
	fixVersionName     = fixVersion.Arg("name", "Name of target RC.").Required().String()
	fixVersionInterval = fixVersion.Flag("interval", "Reloading interval after update.").Default("0").Int()
)

func main() {

	ktool := kube.Tool{}
	if force != nil {
		ktool.SetForce(*force)
	}

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))
	if namespace != nil {
		ktool.SetNamespace(*namespace)
	}

	var err error

	switch cmd {
	case info.FullCommand():
		err = ktool.PrintInfo()
	case rc.FullCommand():
		err = ktool.PrintRCList()
	case pod.FullCommand():
		rcname := ""
		if podRC != nil {
			rcname = *podRC
		}
		err = ktool.PrintPodList(rcname)
	case reload.FullCommand():
		err = ktool.Reload(*reloadName, *reloadInterval, *reloadOne)
	case update.FullCommand():
		container := ""
		if updateContainer != nil {
			container = *updateContainer
		}
		err = ktool.Update(*updateName, container, *updateVersion)
		if *updateReload && err != nil {
			err = ktool.Reload(*updateName, *updateInterval, *updateReloadOne)
		}
	case fixVersion.FullCommand():
		err = ktool.FixVersion(*fixVersionName, *fixVersionInterval)
	}
	if err != nil {
		fmt.Println(red(err.Error()))
		os.Exit(1)
	}

}
