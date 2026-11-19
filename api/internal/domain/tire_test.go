package domain

import "testing"

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		mm     float64
		damage bool
		status string
		level  int
	}{{5, false, "ok", 0}, {4.9, false, "alerta", 1}, {3, false, "alerta", 1}, {2.9, false, "critico", 2}, {1.6, false, "critico", 2}, {9, true, "critico", 3}} {
		got, e := Classify(c.mm, c.damage)
		if e != nil || got.Status != c.status || got.Level != c.level {
			t.Fatalf("%+v => %+v %v", c, got, e)
		}
	}
	if _, e := Classify(-1, false); e == nil {
		t.Fatal("accepted negative tread")
	}
}

func TestConfiguredThresholds(t *testing.T) {
	c, e := ClassifyWithThresholds(4.5, false, 6, 4)
	if e != nil || c.Status != "alerta" {
		t.Fatal(c, e)
	}
	c, e = ClassifyWithThresholds(3.5, false, 6, 4)
	if e != nil || c.Status != "critico" {
		t.Fatal(c, e)
	}
	if _, e = ClassifyWithThresholds(8, false, 3, 5); e == nil {
		t.Fatal("accepted inverted thresholds")
	}
}
