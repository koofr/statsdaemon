package main

import (
	"bytes"
	"github.com/bmizerany/assert"
	"github.com/koofr/statsdaemon/common"
	"github.com/koofr/statsdaemon/counter"
	"github.com/koofr/statsdaemon/timer"
	"github.com/koofr/statsdaemon/udp"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var commonPercentiles = Percentiles{
	&Percentile{
		99,
		"99",
	},
}
var output = common.NullOutput()

func TestPacketParse(t *testing.T) {
	d := []byte("gaugor:333|g")
	packets := udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 1)
	packet := packets[0]
	assert.Equal(t, "gaugor", packet.Bucket)
	assert.Equal(t, float64(333), packet.Value)
	assert.Equal(t, "g", packet.Modifier)
	assert.Equal(t, float32(1), packet.Sampling)

	d = []byte("gorets:2|c|@0.1")
	packets = udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 1)
	packet = packets[0]
	assert.Equal(t, "gorets", packet.Bucket)
	assert.Equal(t, float64(2), packet.Value)
	assert.Equal(t, "c", packet.Modifier)
	assert.Equal(t, float32(0.1), packet.Sampling)

	d = []byte("gorets:4|c")
	packets = udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 1)
	packet = packets[0]
	assert.Equal(t, "gorets", packet.Bucket)
	assert.Equal(t, float64(4), packet.Value)
	assert.Equal(t, "c", packet.Modifier)
	assert.Equal(t, float32(1), packet.Sampling)

	d = []byte("gorets:-4|c")
	packets = udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 1)
	packet = packets[0]
	assert.Equal(t, "gorets", packet.Bucket)
	assert.Equal(t, float64(-4), packet.Value)
	assert.Equal(t, "c", packet.Modifier)
	assert.Equal(t, float32(1), packet.Sampling)

	d = []byte("glork:320|ms")
	packets = udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 1)
	packet = packets[0]
	assert.Equal(t, "glork", packet.Bucket)
	assert.Equal(t, float64(320), packet.Value)
	assert.Equal(t, "ms", packet.Modifier)
	assert.Equal(t, float32(1), packet.Sampling)

	d = []byte("a.key.with-0.dash:4|c")
	packets = udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 1)
	packet = packets[0]
	assert.Equal(t, "a.key.with-0.dash", packet.Bucket)
	assert.Equal(t, float64(4), packet.Value)
	assert.Equal(t, "c", packet.Modifier)
	assert.Equal(t, float32(1), packet.Sampling)

	d = []byte("a.key.with-0.dash:4|c\ngauge:3|g")
	packets = udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 2)
	packet = packets[0]
	assert.Equal(t, "a.key.with-0.dash", packet.Bucket)
	assert.Equal(t, float64(4), packet.Value)
	assert.Equal(t, "c", packet.Modifier)
	assert.Equal(t, float32(1), packet.Sampling)

	packet = packets[1]
	assert.Equal(t, "gauge", packet.Bucket)
	assert.Equal(t, float64(3), packet.Value)
	assert.Equal(t, "g", packet.Modifier)
	assert.Equal(t, float32(1), packet.Sampling)

	errors_key := "target_type=count.type=invalid_line.unit=Err"
	d = []byte("a.key.with-0.dash:4\ngauge3|g")
	packets = udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 2)
	assert.Equal(t, packets[0].Bucket, errors_key)
	assert.Equal(t, packets[1].Bucket, errors_key)

	d = []byte("a.key.with-0.dash:4")
	packets = udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)
	assert.Equal(t, len(packets), 1)
	assert.Equal(t, packets[0].Bucket, errors_key)
}

func TestMean(t *testing.T) {
	// Some data with expected mean of 20
	d := []byte("response_time:0|ms\nresponse_time:30|ms\nresponse_time:30|ms")
	packets := udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)

	for _, p := range packets {
		timer.Add(timers, p)
	}
	var buff bytes.Buffer
	var num int64
	num += processTimers(&buff, time.Now().Unix(), Percentiles{})
	assert.Equal(t, num, int64(1))
	dataForGraphite := buff.String()
	pattern := `response_time\.mean 20\.[0-9]+ `
	meanRegexp := regexp.MustCompile(pattern)

	matched := meanRegexp.MatchString(dataForGraphite)
	assert.Equal(t, matched, true)
}

func TestUpperPercentile(t *testing.T) {
	// Some data with expected mean of 20
	d := []byte("time:0|ms\ntime:1|ms\ntime:2|ms\ntime:3|ms")
	packets := udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)

	for _, p := range packets {
		timer.Add(timers, p)
	}

	var buff bytes.Buffer
	var num int64
	num += processTimers(&buff, time.Now().Unix(), Percentiles{
		&Percentile{
			75,
			"75",
		},
	})
	assert.Equal(t, num, int64(1))
	dataForGraphite := buff.String()

	meanRegexp := regexp.MustCompile(`time\.upper_75 2\.`)
	matched := meanRegexp.MatchString(dataForGraphite)
	assert.Equal(t, matched, true)
}

func TestMetrics20Timer(t *testing.T) {
	d := []byte("foo=bar.target_type=gauge.unit=ms:5|ms\nfoo=bar.target_type=gauge.unit=ms:10|ms")
	packets := udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)

	for _, p := range packets {
		timer.Add(timers, p)
	}

	var buff bytes.Buffer
	var num int64
	num += processTimers(&buff, time.Now().Unix(), Percentiles{
		&Percentile{
			75,
			"75",
		},
	})
	dataForGraphite := buff.String()
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=upper_75 10.000000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=mean_75 7.500000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=sum_75 15.000000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=mean 7.500000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=median 7.500000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=std 2.500000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=sum 15.000000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=upper 10.000000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=gauge.unit=ms.stat=lower 5.000000"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=count.unit=Pckt.orig_unit=ms.pckt_type=sent.direction=in 2"))
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=rate.unit=Pcktps.orig_unit=ms.pckt_type=sent.direction=in 0.200000"))
}
func TestMetrics20Count(t *testing.T) {
	d := []byte("foo=bar.target_type=count.unit=B:5|c\nfoo=bar.target_type=count.unit=B:10|c")
	packets := udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)

	for _, p := range packets {
		counter.Add(counters, p)
	}

	var buff bytes.Buffer
	var num int64
	num += processCounters(&buff, time.Now().Unix(), Percentiles{
		&Percentile{
			75,
			"75",
		},
	})
	dataForGraphite := buff.String()
	assert.T(t, strings.Contains(dataForGraphite, "foo=bar.target_type=rate.unit=Bps 1.5"))
}

func TestLowerPercentile(t *testing.T) {
	// Some data with expected mean of 20
	d := []byte("time:0|ms\ntime:1|ms\ntime:2|ms\ntime:3|ms")
	packets := udp.ParseMessage(d, prefix_internal, output, udp.ParseLine)

	for _, p := range packets {
		timer.Add(timers, p)
	}

	var buff bytes.Buffer
	var num int64
	num += processTimers(&buff, time.Now().Unix(), Percentiles{
		&Percentile{
			-75,
			"-75",
		},
	})
	assert.Equal(t, num, int64(1))
	dataForGraphite := buff.String()

	meanRegexp := regexp.MustCompile(`time\.upper_75 1\.`)
	matched := meanRegexp.MatchString(dataForGraphite)
	assert.Equal(t, matched, false)

	meanRegexp = regexp.MustCompile(`time\.lower_75 1\.`)
	matched = meanRegexp.MatchString(dataForGraphite)
	assert.Equal(t, matched, true)
}

func BenchmarkManyDifferentSensors(t *testing.B) {
	r := rand.New(rand.NewSource(438))
	for i := 0; i < 1000; i++ {
		bucket := "response_time" + strconv.Itoa(i)
		for i := 0; i < 10000; i++ {
			m := &common.Metric{bucket, r.Float64(), "ms", r.Float32(), 0}
			timer.Add(timers, m)
		}
	}

	for i := 0; i < 1000; i++ {
		bucket := "count" + strconv.Itoa(i)
		for i := 0; i < 10000; i++ {
			a := r.Float64()
			counters[bucket] = a
		}
	}

	for i := 0; i < 1000; i++ {
		bucket := "gauge" + strconv.Itoa(i)
		for i := 0; i < 10000; i++ {
			a := r.Float64()
			gauges[bucket] = a
		}
	}

	var buff bytes.Buffer
	now := time.Now().Unix()
	t.ResetTimer()
	processTimers(&buff, now, commonPercentiles)
	processCounters(&buff, now, commonPercentiles)
	processGauges(&buff, now, commonPercentiles)
}

func BenchmarkOneBigTimer(t *testing.B) {
	r := rand.New(rand.NewSource(438))
	bucket := "response_time"
	for i := 0; i < 10000000; i++ {
		m := &common.Metric{bucket, r.Float64(), "ms", r.Float32(), 0}
		timer.Add(timers, m)
	}

	var buff bytes.Buffer
	t.ResetTimer()
	processTimers(&buff, time.Now().Unix(), commonPercentiles)
}

func BenchmarkLotsOfTimers(t *testing.B) {
	r := rand.New(rand.NewSource(438))
	for i := 0; i < 1000; i++ {
		bucket := "response_time" + strconv.Itoa(i)
		for i := 0; i < 10000; i++ {
			m := &common.Metric{bucket, r.Float64(), "ms", r.Float32(), 0}
			timer.Add(timers, m)
		}
	}

	var buff bytes.Buffer
	t.ResetTimer()
	processTimers(&buff, time.Now().Unix(), commonPercentiles)
}
