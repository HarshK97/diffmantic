package tui

import "testing"

func testModel(t *testing.T, srcBytes, dstBytes []byte) model {
	t.Helper()
	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24
	m.ready = true
	m.openAllFolds()
	return m
}
