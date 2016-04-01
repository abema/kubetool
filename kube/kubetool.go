package kube

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api/v1"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
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
	kubectl Kubectl
	force   bool
}

func init() {
	out = os.Stdout
}

// SetNamespace to define target namespace.
func (t *Tool) SetNamespace(namespace string) {
	t.kubectl.Namespace = namespace
}

// SetForce to skip terminal confirmation.
func (t *Tool) SetForce(force bool) {
	t.force = force
}

func newTableWriter() *tablewriter.Table {
	w := tablewriter.NewWriter(out)
	w.SetAlignment(tablewriter.ALIGN_LEFT)
	w.SetBorder(false)
	w.SetHeaderLine(false)
	w.SetRowLine(false)
	w.SetColumnSeparator("")
	w.SetCenterSeparator("")
	return w
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
	pods, err := t.kubectl.PodList(nil)
	if err != nil {
		return
	}
	w := newTableWriter()
	w.SetHeader([]string{"name", "status", "R", "pod ip", "node ip", "image", "version"})
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
			w.Append([]string{
				pod.Name,
				status,
				strconv.Itoa(pod.Status.ContainerStatuses[i].RestartCount),
				pod.Status.PodIP,
				pod.Status.HostIP,
				img,
				ver,
			})
		}
	}
	w.Render()
	return
}

// PrintRCList print images of running RCs.
func (t *Tool) PrintRCList() (err error) {
	rcs, err := t.kubectl.RCList()
	if err != nil {
		return
	}
	w := newTableWriter()
	w.SetHeader([]string{"name", "replicas", "image", "version"})
	for _, rc := range rcs {
		rcSpec := rc.Spec
		if rcSpec.Template == nil {
			continue
		}
		podSpec := rcSpec.Template.Spec
		for _, container := range podSpec.Containers {
			img, ver := parseImage(container.Image)
			w.Append([]string{
				rc.Name,
				fmt.Sprintf("%d/%d", rc.Status.Replicas, *rcSpec.Replicas),
				img,
				ver,
			})
		}
	}
	w.Render()
	return
}

// Reload all or one pod(s) in single rc.
func (t *Tool) Reload(name string, interval int, one bool) (err error) {
	if one {
		log("Reloading " + magenta("1") + " pod in replication controller.")
	} else {
		log("Reloading " + red("all") + " pods in replication controller.")
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
		logf("pod[%03d]: %s", i, green(pods[i].Name))
	}
	t.confirm("continue?")
	// do reload
	err = t.reloadPods(rc, pods, interval)
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
func (t *Tool) FixVersion(name string, interval int) (err error) {
	rc, err := t.kubectl.RC(name)
	if err != nil {
		return
	}
	allPods, err := t.kubectl.PodList(rc.Spec.Selector)
	if err != nil {
		return
	}
	pods := []v1.Pod{}
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
		logf("pod[%03d]: %s", i, green(pods[i].Name))
	}
	t.confirm("continue?")
	// do reload
	err = t.reloadPods(rc, pods, interval)
	return
}

// reloadPods deletes pods one by one with waiting created pod become available.
func (t *Tool) reloadPods(rc v1.ReplicationController, pods []v1.Pod, interval int) (err error) {

	// check all pods available
	if !t.rcAvailable(rc) {
		if err = t.waitRCAvailable(rc.Name); err != nil {
			return
		}
	}

	// delete pods one by one.
	for i := range pods {
		logf("Deleting %s...", green(pods[i].Name))
		if err = t.kubectl.DeletePod(pods[i].Name); err != nil {
			return
		}
		// wait for specified interval seconds
		if err = t.waitRCAvailable(rc.Name); err != nil {
			return
		}
		if interval > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}

	log(green("Done reloading pods"))
	return
}

// waitRCAvailable for 1 minutes, exit to abort when not available.
func (t *Tool) waitRCAvailable(name string) (err error) {
	log("Wait until all pods available...")
	avail := false
	// wait for about a minute to available all pods includes recreated.
	for i := 0; i < 10; i++ {
		// get RC again and check pod statuses
		rc, err := t.kubectl.RC(name)
		if err != nil {
			return err
		}
		avail = t.rcAvailable(rc)
		if avail {
			break
		}
		// wait for 5 seconds
		time.Sleep(time.Second * 5)
	}
	// exit when pod is unavailable
	if !avail {
		return errors.New("RC is not stable stauts")
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
func pickContainer(rc v1.ReplicationController, container string) (c v1.Container, err error) {
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
func (t *Tool) rcAvailable(rc v1.ReplicationController) bool {
	// check pods count reaches rc desied.
	curr := rc.Status.Replicas
	total := *rc.Spec.Replicas
	if curr != total {
		return false
	}
	// check all pod status.
	pods, err := t.kubectl.PodList(rc.Spec.Selector)
	if err != nil {
		log(red(err.Error()))
		return false
	}
	// check actual pod count equals to spec.
	if len(pods) != total {
		return false
	}
	for i := range pods {
		if !t.podAvailable(pods[i]) {
			return false
		}
	}
	return true
}

// check pod status.
func (t *Tool) podAvailable(pod v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return false
	}
	for _, cs := range pod.Status.ContainerStatuses {
		// must be running
		if cs.State.Running == nil {
			return false
		}
		// must be ready
		if !cs.Ready {
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
	if t.force {
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
