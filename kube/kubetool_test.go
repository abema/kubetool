package kube

import (
	"bytes"
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetNamespace(t *testing.T) {
	kt := Tool{}
	require.Equal(t, "", kt.kubectl.Namespace)
	kt.SetNamespace("YO")
	require.Equal(t, "YO", kt.kubectl.Namespace)
}

func TestSetForce(t *testing.T) {
	kt := Tool{}
	assert.False(t, kt.force)
	kt.SetForce(true)
	assert.True(t, kt.force)
	kt.SetForce(false)
	assert.False(t, kt.force)
}

func TestSetYes(t *testing.T) {
	kt := Tool{}
	assert.False(t, kt.yes)
	kt.SetYes(true)
	assert.True(t, kt.yes)
	kt.SetYes(false)
	assert.False(t, kt.yes)
}

func TestSetInterval(t *testing.T) {
	kt := Tool{}
	assert.Equal(t, 0, kt.interval)
	kt.SetInterval(10)
	assert.Equal(t, 10, kt.interval)
}

func TestSetMinimumStable(t *testing.T) {
	kt := Tool{}
	assert.Equal(t, float64(0), kt.minStable)
	kt.SetMinimumStable(0.8)
	assert.Equal(t, float64(0.8), kt.minStable)
}

func TestPrintInfo(t *testing.T) {
	b := bytes.Buffer{}
	out = &b
	kt := Tool{}
	require.NoError(t, kt.PrintInfo())
	text := b.String()
	require.NotEmpty(t, text)
}

func TestPrintContext(t *testing.T) {
	b := bytes.Buffer{}
	out = &b
	kt := Tool{}
	require.NoError(t, kt.PrintContext())
	text := b.String()
	require.NotEmpty(t, text)
}

func TestPrintPodList(t *testing.T) {
	b := bytes.Buffer{}
	out = &b
	kt := Tool{}
	require.NoError(t, kt.PrintPodList("kubetool-test"))
	text := b.String()
	require.NotEmpty(t, text)
	require.Contains(t, text, "kubetool-test")
}

func TestPrintRCList(t *testing.T) {
	b := bytes.Buffer{}
	out = &b
	kt := Tool{}
	require.NoError(t, kt.PrintRCList())
	text := b.String()
	require.NotEmpty(t, text)
	require.Contains(t, text, "kubetool-test")
}

func TestReload(t *testing.T) {
	out = os.Stdout
	kt := Tool{}
	kt.SetForce(true)

	// wait rc available
	rc, err := kt.kubectl.RC("kubetool-test")
	require.NoError(t, err)
	require.NoError(t, kt.waitRCAvailable(rc.Name, []string{}))

	// get pod list of RC
	olds, err := kt.kubectl.PodList(Selector{"name": "kubetool-test"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(olds))

	// reload all
	kt.Reload("kubetool-test", false)

	// get new pod list of RC
	news, err := kt.kubectl.PodList(Selector{"name": "kubetool-test"})
	require.NoError(t, err)
	require.Equal(t, 2, len(news))
	for i := range news {
		assert.Equal(t, v1.PodRunning, news[i].Status.Phase)
		assert.True(t, news[i].Status.ContainerStatuses[0].Ready)
		// must be re-created
		assert.NotEqual(t, olds[i].Name, news[i].Name)
	}
}

func TestUpdate(t *testing.T) {

	kt := Tool{}
	kt.SetForce(true)
	require.NoError(t, kt.Update("kubetool-test", "", "1.9.12"))

	rc, err := kt.kubectl.RC("kubetool-test")
	require.NoError(t, err)
	assert.Equal(t, "nginx:1.9.12", rc.Spec.Template.Spec.Containers[0].Image)

}
