package monitor

import (
	"fmt"
	"testing"
)

func TestUnmarshalHeartbeatMsg(t *testing.T) {
	// actual, err := unmarshalHeartbeatMsg(heartbeatMsg)
	// require.NoError(t, err)
	// require.Equal(t, expectedMetricsItem, actual)
}

func TestWatchLocalNode(t *testing.T) {
	// m := NewMonitor("ke-worker2", "Nanjing")
	// go m.watchLocalNode()
	// time.Sleep(2 * time.Second)
	// metrics := m.store.Get("ke-worker2")
	// data, err := marshalHeartbeatMsg(metrics)
	// require.NoError(t, err)
	// fmt.Println(data)
	// m.Stop()
}

type Test struct {
	first  string
	second int
}

func TestSomething(t *testing.T) {
	m1 := make(map[string]*Test)
	m1["test"] = &Test{first: "test", second: 1}

	m2 := make(map[string]*Test)
	for name, item := range m1 {
		m2[name] = item
	}
	m2["test"].first = "hello world"
	fmt.Println(m1["test"])
}
