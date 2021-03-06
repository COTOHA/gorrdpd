package writers

import (
    "container/vector"
    "exec"
    "fmt"
    "os"
    "sort"
    "strings"
    "./config"
)


type Writer interface {
    Rollup(time int64, key string, samples *vector.IntVector)
}

func getRrdFile(t string, key string) string {
    return fmt.Sprintf("%s/%s-%s.rrd", config.GlobalConfig.DataDir, t, key)
}

func runRrd(argv []string) {
    config.GlobalConfig.Logger.Debug(strings.Join(argv, " "))
    p, error := exec.Run(config.GlobalConfig.RrdToolPath, argv, nil, config.GlobalConfig.DataDir, exec.PassThrough, exec.PassThrough, exec.PassThrough)
    if error != nil {

    } else {
        if error = p.Close(); error != nil {

        }
    }
}

/******************************************************************************/

type Quartiles struct {
}

type QuartilesItem struct {
    time int64
    lo, q1, q2, q3, hi, total int
}

func (quartiles *Quartiles) Rollup(time int64, key string, samples *vector.IntVector) {
    if samples.Len() < 2 { return }
    sort.Sort(samples)
    lo := samples.At(0)
    hi := samples.At(samples.Len() - 1)
    number := samples.Len()
    lo_c := number / 2
    hi_c := number - lo_c
    data := &QuartilesItem {}
    if lo_c > 0 && hi_c > 0 {
        lo_samples := samples.Slice(0, lo_c)
        hi_samples := samples.Slice(lo_c, hi_c)
        lo_sum := 0
        hi_sum := 0
        lo_samples.Do(func(elem interface {}) { lo_sum += elem.(int) })
        hi_samples.Do(func(elem interface {}) { hi_sum += elem.(int) })
        q1 := lo_sum / lo_c
        q2 := (lo_sum + hi_sum) / (lo_c + hi_c)
        q3 := hi_sum / hi_c

        data.time = time
        data.lo = lo
        data.q1 = q1
        data.q2 = q2
        data.q3 = q3
        data.hi = hi
        data.total = number
    }
    quartiles.save(time, key, data)
}

func (self *Quartiles) save(t int64, key string, data *QuartilesItem) {
    file := getRrdFile("quartiles", key)
    if _, err := os.Stat(file); err != nil {
        argv := []string {
            config.GlobalConfig.RrdToolPath,
            "create", file,
            "--step", "10",
            "--start", fmt.Sprintf("%d", data.time - 1),
            "DS:q1:GAUGE:600:0:U",
            "DS:q2:GAUGE:600:0:U",
            "DS:q3:GAUGE:600:0:U",
            "DS:hi:GAUGE:600:0:U",
            "DS:lo:GAUGE:600:0:U",
            "DS:total:GAUGE:600:0:U",
            "RRA:AVERAGE:0.5:1:25920",      // 72 hours at 1 sample per 10 secs
            "RRA:AVERAGE:0.5:60:4320",      // 1 month at 1 sample per 10 mins
            "RRA:AVERAGE:0.5:2880:5475",    // 5 years at 1 sample per 8 hours
            "RRA:MIN:0.5:1:25920",          // 72 hours at 1 sample per 10 secs
            "RRA:MIN:0.5:60:4320",          // 1 month at 1 sample per 10 mins
            "RRA:MIN:0.5:2880:5475",        // 5 years at 1 sample per 8 hours
            "RRA:MAX:0.5:1:25920",          // 72 hours at 1 sample per 10 secs
            "RRA:MAX:0.5:60:4320",          // 1 month at 1 sample per 10 mins
            "RRA:MAX:0.5:2880:5475",        // 5 years at 1 sample per 8 hours
        }
        runRrd(argv)
    }
    argv := []string {
        config.GlobalConfig.RrdToolPath,
        "update", file,
        fmt.Sprintf("%d:%d:%d:%d:%d:%d:%d", data.time, data.q1, data.q2, data.q3, data.lo, data.hi, data.total),
    }
    runRrd(argv)
}

/******************************************************************************/

type YesOrNo struct {
}

type YesOrNoItem struct {
    ok   uint64
    fail uint64
}

func (self *YesOrNo) Rollup(time int64, key string, samples *vector.IntVector) {
	data := &YesOrNoItem {}
	samples.Do(func(elem interface{}) {
	    value := elem.(int)
	    if value > 0 {
	        data.ok++
        } else {
            data.fail++
        }
	})
	self.save(time, key, data)
}

func (self *YesOrNo) save(t int64, key string, data *YesOrNoItem) {
    file := getRrdFile("yesno", key)
    if _, err := os.Stat(file); err != nil {
        argv := []string {
            config.GlobalConfig.RrdToolPath,
            "create", file,
            "--step", "10",
            "--start", fmt.Sprintf("%d", t - 1),
            "DS:ok:GAUGE:600:0:U",
            "DS:fail:GAUGE:600:0:U",
            "RRA:AVERAGE:0.5:1:25920",      // 72 hours at 1 sample per 10 secs
            "RRA:AVERAGE:0.5:60:4320",      // 1 month at 1 sample per 10 mins
            "RRA:AVERAGE:0.5:2880:5475",    // 5 years at 1 sample per 8 hours
        }
        runRrd(argv)
    }
    argv := []string {
        config.GlobalConfig.RrdToolPath,
        "update", file,
        fmt.Sprintf("%d:%d:%d", t, data.ok, data.fail),
    }
    runRrd(argv)
}
