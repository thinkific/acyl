package metrics

type FakeCollector struct{}

func (fc *FakeCollector) Timing(name string, tags ...string) (end func(moretags ...string)) {
	return func(moretags ...string) {}
}
func (fc *FakeCollector) Increment(name string, tags ...string)            {}
func (fc *FakeCollector) Gauge(name string, value float64, tags ...string) {}

var _ Collector = &FakeCollector{}
