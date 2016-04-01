package kube

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Exit(testMain(m))
}

func testMain(m *testing.M) int {
	// create test rc
	out, err := exec.Command("kubectl", "create", "-f", "test_rc.yml").CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		return 1
	}
	defer func() {
		// drop test rc
		out, err := exec.Command("kubectl", "delete", "-f", "test_rc.yml").CombinedOutput()
		if err != nil {
			fmt.Println(string(out))
		}
	}()
	// drop test rc
	return m.Run()
}

func TestVersion(t *testing.T) {
	kc := Kubectl{}
	s, c, err := kc.Version()
	require.NoError(t, err)
	require.NotEmpty(t, s)
	require.NotEmpty(t, c)
}

func TestCurrentContext(t *testing.T) {
	kc := Kubectl{}
	c, err := kc.CurrentContext()
	require.NoError(t, err)
	require.NotEmpty(t, c)
}

func TestPodList(t *testing.T) {
	kc := Kubectl{}
	c, err := kc.PodList(nil)
	require.NoError(t, err)
	require.NotEmpty(t, c)
}

func TestRCList(t *testing.T) {
	kc := Kubectl{}
	c, err := kc.RCList()
	require.NoError(t, err)
	require.NotEmpty(t, c)
}

func TestPod(t *testing.T) {
	kc := Kubectl{}
	pods, err := kc.PodList(nil)
	require.NoError(t, err)
	pod, err := kc.Pod(pods[0].Name)
	require.NoError(t, err)
	require.Equal(t, pod.Name, pods[0].Name)
}

func TestRC(t *testing.T) {
	kc := Kubectl{}
	rcs, err := kc.RCList()
	require.NoError(t, err)
	rc, err := kc.RC(rcs[0].Name)
	require.NoError(t, err)
	require.Equal(t, rc.Name, rcs[0].Name)
}

func TestDeletePod(t *testing.T) {
	kc := Kubectl{}
	// wait until pods ready
	time.Sleep(5 * time.Second)

	pods, err := kc.PodList(Selector{"name": "kubetool-test"})
	require.NoError(t, err)
	require.Equal(t, 2, len(pods))

	require.NoError(t, kc.DeletePod(pods[0].Name))

	/* TODO better deleting timing check
	// get status
	p, err := kc.Pod(pods[0].Name)
	require.NoError(t, err)
	// phase will be pending
	fmt.Println(p.Status.ContainerStatuses[0].State)
	assert.NotNil(t, p.Status.ContainerStatuses[0].State.Running)
	assert.NotNil(t, p.Status.ContainerStatuses[0].State.Terminated)
	assert.NotNil(t, p.Status.ContainerStatuses[0].State.Waiting)
	*/

}

func TestPatch(t *testing.T) {
	kc := Kubectl{}
	rc, err := kc.RC("kubetool-test")
	require.NoError(t, err)

	err = kc.PatchRC(rc.Name, `{"metadata":{"labels":{"test":"kubetool"}}}`)
	require.NoError(t, err)

	rc, err = kc.RC("kubetool-test")
	require.NoError(t, err)
	require.Equal(t, "kubetool", rc.ObjectMeta.Labels["test"])
}
