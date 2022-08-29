package remoteconfig

import "testing"

func TestConfigurator(t *testing.T) {
	c := new(configurator)
	notify := func(key string, oldData, newData interface{}) {}
	c.RegisterChangeNotify(notify)
	c.OnDataChange("test", nil, nil)
	_ = c.Close()
}
