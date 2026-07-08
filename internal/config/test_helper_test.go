package config

import "testing"

func resetForTest() {
	v = nil
	globalCfg = nil
	rls = nil
	validators = nil
	changeHooks = nil
}

func TestMain(m *testing.M) {
	resetForTest()
	m.Run()
}
