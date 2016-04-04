package kube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Kubectl executes kubectl as command.
type Kubectl struct {
	Debug     bool
	Namespace string
}

// Exec kubectl commands with arguments.
func (kc *Kubectl) Exec(args ...string) (b []byte, err error) {

	if kc.Namespace == "all" {
		args = append(args, "--all-namespace")
	} else if kc.Namespace != "" {
		args = append(args, "--namespace="+kc.Namespace)
	}

	stdout := bytes.Buffer{}
	cmd := exec.Command("kubectl", args...)
	if kc.Debug {
		log("exec kubectl", args)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return
	}
	return stdout.Bytes(), nil

}

func trim(text string) string {
	return strings.Trim(text, " \n")
}

// CurrentContext returns current context.
func (kc *Kubectl) CurrentContext() (ctx string, err error) {
	b, err := kc.Exec("config", "current-context")
	if err != nil {
		return
	}
	return trim(string(b)), nil
}

// Version returns kubernetes client/server version.
func (kc *Kubectl) Version() (client string, server string, err error) {
	b, err := kc.Exec("version")
	if err != nil {
		return
	}
	verstr := string(b)
	cliptrn := regexp.MustCompile(`(?m)Client Version:.*GitVersion:"(.+?)"`)
	srvptrn := regexp.MustCompile(`(?m)Server Version:.*GitVersion:"(.+?)"`)

	cliv := cliptrn.FindStringSubmatch(verstr)
	srvv := srvptrn.FindStringSubmatch(verstr)

	if len(cliv) < 2 {
		err = fmt.Errorf("client version not found")
		return
	}
	if len(srvv) < 2 {
		err = fmt.Errorf("server version not found")
		return
	}
	return cliv[1], srvv[1], nil
}

type rcList struct {
	Items []ReplicationController
}

// RCList return replication controllers.
func (kc *Kubectl) RCList() (rcs []ReplicationController, err error) {
	b, err := kc.Exec("get", "rc", "--output=json")
	if err != nil {
		return
	}
	list := rcList{}
	if err = json.Unmarshal(b, &list); err != nil {
		err = errors.New(trim(string(b)))
	}
	return list.Items, err
}

// RC return single replication controller.
func (kc Kubectl) RC(name string) (rc ReplicationController, err error) {
	b, err := kc.Exec("get", "rc", name, "--output=json")
	if err != nil {
		return
	}
	if err = json.Unmarshal(b, &rc); err != nil {
		err = errors.New(trim(string(b)))
	}
	return
}

// PatchRC updates RC fields.
func (kc Kubectl) PatchRC(rc string, patch string) (err error) {
	_, err = kc.Exec("patch", "rc", rc, "-p", patch)
	return
}

type podList struct {
	Items []Pod
}

// PodList return pods.
func (kc *Kubectl) PodList(selector Selector) (pods []Pod, err error) {
	args := []string{"get", "pod", "--output=json"}
	if selector != nil && len(selector) > 0 {
		args = append(args, "--selector="+selector.Format())
	}
	b, err := kc.Exec(args...)
	if err != nil {
		return
	}
	list := podList{}
	json.Unmarshal(b, &list)
	return list.Items, nil
}

// Pod return single pod.
func (kc *Kubectl) Pod(name string) (pod Pod, err error) {
	b, err := kc.Exec("get", "pod", name, "--output=json")
	if err != nil {
		return
	}
	if err = json.Unmarshal(b, &pod); err != nil {
		err = errors.New(trim(string(b)))
	}
	return
}

// DeletePod in cluster
func (kc *Kubectl) DeletePod(name string) (err error) {
	_, err = kc.Exec("delete", "pod", name)
	return
}

// Selector map.
type Selector map[string]string

// Format selector as k=v,k=v style.
func (s Selector) Format() string {
	list := []string{}
	for k, v := range s {
		list = append(list, k+"="+v)
	}
	return strings.Join(list, ",")
}
