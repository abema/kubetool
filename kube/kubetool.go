package kube

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/buger/goterm"
	"github.com/fatih/color"
)

var (
	red     = color.New(color.FgRed).SprintfFunc()
	blue    = color.New(color.FgBlue).SprintfFunc()
	green   = color.New(color.FgGreen).SprintfFunc()
	yellow  = color.New(color.FgYellow).SprintfFunc()
	magenta = color.New(color.FgMagenta).SprintfFunc()
	cyan    = color.New(color.FgCyan).SprintfFunc()
	gray    = color.New(color.FgHiBlack).SprintfFunc()
	white   = color.New(color.FgWhite).SprintfFunc()
	bold    = color.New(color.Bold).SprintfFunc()

	out io.Writer
)

// Tool is to execute batch tasks using kubectl command.
type Tool struct {
	kubectl   Kubectl
	yes       bool
	force     bool
	interval  int
	minStable float64
}

func init() {
	out = os.Stdout
}

// SetNamespace to define target namespace.
func (t *Tool) SetNamespace(namespace string) {
	t.kubectl.Namespace = namespace
}

// SetYes to skip confirmation.
func (t *Tool) SetYes(yes bool) {
	t.yes = yes
}

// SetForce to reload without status checking.
func (t *Tool) SetForce(force bool) {
	t.force = force
}

// SetInterval seconds between each pod restarts.
func (t *Tool) SetInterval(interval int) {
	t.interval = interval
}

// SetMinimumStable available pods rate to detect RC availability.
func (t *Tool) SetMinimumStable(minStable float64) {
	t.minStable = minStable
}

// PrintInfo writes version of target cluster.
func (t *Tool) PrintInfo() (err error) {
	c, s, err := t.kubectl.Version()
	if err != nil {
		return
	}
	t.PrintContext()
	log("client version :", green(c))
	log("server version :", blue(s))
	return
}

// PrintContext writes current active context.
func (t *Tool) PrintContext() (err error) {
	cc, err := t.kubectl.CurrentContext()
	if err != nil {
		return
	}
	log("context ->", yellow(cc))
	return
}

// PrintPodList print images of running pods in specific RC.
func (t *Tool) PrintPodList(rcname string) (err error) {
	var selector Selector
	if rcname != "" {
		selector = Selector{"name": rcname}
	}

	pods, err := t.kubectl.PodList(selector)
	if err != nil {
		return
	}
	w := goterm.NewTable(0, 4, 1, ' ', 0)
	fmt.Fprintf(w, "NAME\tSTATUS\tR\tPOD IP\tNODE IP\tIMAGE\tVERSION\n")
	for _, pod := range pods {
		for i, container := range pod.Spec.Containers {
			cstate := pod.Status.ContainerStatuses[i].State
			status := "Running"
			if cstate.Terminated != nil {
				status = "Terminated"
			} else if cstate.Waiting != nil {
				status = "Waiting"
			}
			img, ver := parseImage(container.Image)
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
				pod.Name,
				status,
				pod.Status.ContainerStatuses[i].RestartCount,
				pod.Status.PodIP,
				pod.Status.HostIP,
				img, ver,
			)
		}
	}
	fmt.Println(w.String())
	return
}

// PrintRCList print images of running RCs.
func (t *Tool) PrintRCList() (err error) {
	rcs, err := t.kubectl.RCList()
	if err != nil {
		return
	}
	w := goterm.NewTable(0, 4, 1, ' ', 0)
	fmt.Fprintf(w, "NAME\tREPLICAS\tIMAGE\tVERSION\n")
	for _, rc := range rcs {
		rcSpec := rc.Spec
		if rcSpec.Template == nil {
			continue
		}
		podSpec := rcSpec.Template.Spec
		for _, container := range podSpec.Containers {
			img, ver := parseImage(container.Image)
			fmt.Fprintf(w, "%s\t%d/%d\t%s\t%s\n",
				rc.Name, rc.Status.Replicas, *rcSpec.Replicas, img, ver,
			)
		}
	}
	fmt.Println(w.String())
	//goterm.Println(w)
	return
}

// Reload all or one pod(s) in single rc.
func (t *Tool) Reload(name string, one bool) (err error) {
	if one {
		log("reloading " + magenta("1") + " pod in replication controller.")
	} else {
		log("reloading " + red("all") + " pods in replication controller.")
	}
	rc, err := t.kubectl.RC(name)
	if err != nil {
		return
	}
	pods, err := t.kubectl.PodList(rc.Spec.Selector)
	if err != nil {
		return
	}
	if len(pods) == 0 {
		err = errors.New("no pod found")
		return
	}
	t.PrintContext()
	log("rc      :", blue(name))

	// first pod only
	if one {
		pods = pods[:1]
	}

	for i := range pods {
		if t.podAvailable(pods[i]) {
			logf("pod[%03d]: %s", i, green(pods[i].Name))
		} else {
			logf("pod[%03d]: %s", i, red(pods[i].Name))
		}
	}
	t.confirm("continue?")
	// do reload
	err = t.reloadPods(rc, pods)
	return
}

// Update RC image version to specific value.
func (t *Tool) Update(name string, container string, version string) (err error) {
	rc, err := t.kubectl.RC(name)
	if err != nil {
		return
	}
	c, err := pickContainer(rc, container)
	if err != nil {
		return
	}
	img, ver := parseImage(c.Image)
	t.PrintContext()
	log("RC       :", green(name))
	log("Container:", green(name))
	log("Image    :", magenta(img)+":"+yellow(ver))
	log("       ->:", magenta(img)+":"+bold(yellow(version)))
	t.confirm("continue?")

	newImage := img + ":" + version

	//t.kubectl.Patch
	patch := fmt.Sprintf(`{"spec":{"template":{"spec":{"containers":[{"name":"%s","image":"%s"}]}}}}`, c.Name, newImage)
	if err = t.kubectl.PatchRC(rc.Name, patch); err != nil {
		return
	}
	log(green("Successfully patched"))
	return
}

// FixVersion of pods running on RC with destroying all pods that has
// different version of RC ones.
func (t *Tool) FixVersion(name string) (err error) {
	rc, err := t.kubectl.RC(name)
	if err != nil {
		return
	}
	allPods, err := t.kubectl.PodList(rc.Spec.Selector)
	if err != nil {
		return
	}
	pods := []Pod{}
	rspec := rc.Spec.Template.Spec
	for _, pod := range allPods {
		// when containers has different size, move on
		if len(pod.Spec.Containers) != len(rspec.Containers) {
			pods = append(pods, pod)
			continue
		}
		for i, cs := range pod.Spec.Containers {
			if cs.Image != rspec.Containers[i].Image {
				pods = append(pods, pod)
				break
			}
		}
	}

	if len(pods) == 0 {
		log(green("all pods are up to date."))
		return
	}

	t.PrintContext()
	log("rc      :", blue(name))

	for i := range pods {
		if t.podAvailable(pods[i]) {
			logf("pod[%03d]: %s", i, green(pods[i].Name))
		} else {
			logf("pod[%03d]: %s %s", i, red(pods[i].Name), gray("(unavailable)"))
		}
	}
	t.confirm("continue?")
	// do reload
	err = t.reloadPods(rc, pods)
	return
}

// reloadPods deletes pods one by one with waiting created pod become available.
func (t *Tool) reloadPods(rc ReplicationController, pods []Pod) (err error) {

	livePods := make([]Pod, 0, len(pods))
	deadPods := make([]Pod, 0, len(pods))

	deletedPods := make([]string, 0, len(pods))

	// separate dead/live pods
	for i := range pods {
		if t.podAvailable(pods[i]) {
			livePods = append(livePods, pods[i])
		} else {
			deadPods = append(deadPods, pods[i])
		}
	}
	// delete dead pods first without waiting availability.
	for i := range deadPods {
		logf("deleting pod %s...", red(deadPods[i].Name))
		if err = t.kubectl.DeletePod(deadPods[i].Name); err != nil {
			return
		}
		deletedPods = append(deletedPods, deadPods[i].Name)
	}

	// wait for availability
	t.waitRCAvailable(rc.Name, deletedPods)

	// delete pods one by one.
	for i := range livePods {
		logf("deleting pod %s...", green(livePods[i].Name))
		if err = t.kubectl.DeletePod(livePods[i].Name); err != nil {
			return
		}
		deletedPods = append(deletedPods, livePods[i].Name)
		// wait for specified interval seconds.
		if err = t.waitRCAvailable(rc.Name, deletedPods); err != nil {
			return
		}
		if t.interval > 0 {
			time.Sleep(time.Duration(t.interval) * time.Second)
		}
	}

	log(green("done reloading pods"))
	return
}

// waitRCAvailable for 1 minutes, exit to abort when not available.
// Providing ignoreNames will mark as failed even if pod has same name is available.
func (t *Tool) waitRCAvailable(name string, ignoreNames []string) (err error) {
	if t.force {
		return
	}
	first := true
	avail := false
	// wait for about a minute to available all pods includes recreated.
	for i := 0; i < 10; i++ {
		// get RC again and check pod statuses
		rc, err := t.kubectl.RC(name)
		if err != nil {
			return err
		}
		avail = t.rcAvailable(rc, ignoreNames)
		if avail {
			break
		}
		if first {
			first = false
		}
		// wait for 5 seconds
		time.Sleep(time.Second * 5)
	}
	// exit when pod is unavailable
	if !avail {
		return errors.New("RC does not have enough stable pods. Use -f to force reloading pods")
	}
	return
}

func parseImage(img string) (name string, version string) {
	name = img
	version = "latest"
	if lastColon := strings.LastIndexByte(img, ':'); lastColon > 0 {
		name = img[:lastColon]
		version = img[lastColon+1:]
	}
	return
}

// pickContainer from pods.
func pickContainer(rc ReplicationController, container string) (c Container, err error) {
	cs := rc.Spec.Template.Spec.Containers
	if container == "" {
		return cs[0], nil
	}
	for i := range cs {
		if cs[i].Name == container {
			return cs[i], nil
		}
	}
	return c, fmt.Errorf("container not found: %s", container)
}

// check rc status.
func (t *Tool) rcAvailable(rc ReplicationController, ignorePods []string) bool {
	// check pods count reaches rc desied.
	total := int(*rc.Spec.Replicas)
	// check all pod status.
	pods, err := t.kubectl.PodList(rc.Spec.Selector)
	if err != nil {
		log(red(err.Error()))
		return false
	}
	// minimum available requirement pods
	reqNum := int(float64(total) * t.minStable)
	if reqNum < 1 {
		reqNum = 1
	}
	if reqNum > total {
		reqNum = total
	}
	// count available pods
	availCount := 0
	for i := range pods {
		// check ignore pods
		if contains(pods[i].Name, ignorePods) {
			continue
		}
		if t.podAvailable(pods[i]) {
			availCount++
		}
	}
	// RC is available when available pods > required pods
	if availCount > reqNum {
		return true
	}
	log("waiting", blue(strconv.Itoa(int(reqNum-availCount+1))), "more pod(s) become available. ("+blue(strconv.Itoa(int(availCount)))+"/"+blue(strconv.Itoa(int(*rc.Spec.Replicas)))+")")
	return false
}

func contains(item string, list []string) bool {
	for i := range list {
		if list[i] == item {
			return true
		}
	}
	return false
}

// check pod status.
func (t *Tool) podAvailable(pod Pod) bool {
	if pod.Status.Phase != PodRunning {
		return false
	}
	for _, cs := range pod.Status.ContainerStatuses {
		// must be ready
		if !cs.Ready {
			return false
		}
		// must be running
		if cs.State.Running == nil {
			return false
		}
	}
	return true
}

// fail exit process if err != nil
func fail(err error) {
	log(red(err.Error()))
	os.Exit(1)
}

// confirm user input via terminal.
func (t *Tool) confirm(msg string) {
	if t.yes {
		return
	}
	fmt.Printf(msg + " (y/N) ")
	res := ""
	fmt.Scanf("%s", &res)
	if !strings.Contains(strings.ToLower(res), "y") {
		fail(errors.New("aborted"))
	}
}

func logf(format string, vars ...interface{}) {
	fmt.Fprintf(out, format, vars...)
	fmt.Fprintln(out)
}

func log(msg ...interface{}) {
	fmt.Fprintln(out, msg...)
}
