package main

import (
    "bytes"
    "flag"
    "net"
    "os"
    "path"
    "strconv"
    "strings"
    "time"
    "./config"
    "./logger"
    "./types"
    "./writers"
)

var (
    log logger.Logger
    /* DNS names cache */
    hostLookupCache map[string] string
    /* Slices */
    slices *types.Slices
)

func lookupHost(addr *net.UDPAddr) string {
    ip := addr.IP.String()
    if _, found := hostLookupCache[ip]; found { return hostLookupCache[ip] }

    cname, _, error := net.LookupHost(ip)
    if error != nil {
        // if debug > 1 { log.Stderrf("Host lookup failed for IP %s: %s", ip, error) }
        return ip
    }
    hostLookupCache[ip] = cname
    return cname
}

func process(addr *net.UDPAddr, buf string, msgchan chan<- *types.Message) {
    log.Debug("Processing message from %s: %s", addr, buf)

    fields := strings.Split(buf, ":", 2)

    if value, error := strconv.Atoi(fields[1]); error != nil {
        log.Debug("Number %s is not valid: %s", fields[1], error)
    } else {
        msgchan <- types.NewMessage(lookupHost(addr), fields[0], value)
    }
}

func listen(msgchan chan<- *types.Message) {
    log.Debug("Starting listener on %s", config.GlobalConfig.UDPAddress)

    // Listen for requests
    listener, error := net.ListenUDP("udp", config.GlobalConfig.UDPAddress)
    if error != nil {
        log.Fatal("Cannot listen: %s", error)
        os.Exit(1)
    }
    // Ensure listener will be closed on return
    defer listener.Close()

    message := make([]byte, 256)
    for {
        n, addr, error := listener.ReadFromUDP(message)
        if error != nil {
            log.Debug("Cannot read UDP from %s: %s\n", addr, error)
            continue
        }
        buf := bytes.NewBuffer(message[0:n])
        process(addr, buf.String(), msgchan)
    }
}

func msgSlicer(msgchan <-chan *types.Message) {
    for {
        message := <-msgchan
        slices.Add(message)
    }
}

func initialize() {
    var slice, write, debug int
    var listen, data, rrdtool string
    flag.StringVar(&listen,  "listen",  config.DEFAULT_LISTEN,         "Set the port (+optional address) to listen at")
    flag.StringVar(&data,    "data",    config.DEFAULT_DATA_DIR,       "Set the data directory")
    flag.StringVar(&rrdtool, "rrdtool", config.DEFAULT_RRD_TOOL_PATH,  "Set the rrdtool absolute path")
    flag.IntVar   (&debug,   "debug",   int(config.DEFAULT_SEVERITY),  "Set the debug level, the lower - the more verbose (0-5)")
    flag.IntVar   (&slice,   "slice",   config.DEFAULT_SLICE_INTERVAL, "Set the slice interval in seconds")
    flag.IntVar   (&write,   "write",   config.DEFAULT_WRITE_INTERVAL, "Set the write interval in seconds")
    flag.Parse()

    if len(data) == 0 || data[0] != '/' {
        wd, _ := os.Getwd()
        data = path.Join(wd, data)
    }

    config.GlobalConfig.Listen        = listen
    config.GlobalConfig.DataDir       = data
    config.GlobalConfig.RrdToolPath   = path.Clean(rrdtool)
    config.GlobalConfig.Logger        = logger.NewConsoleLogger(logger.Severity(debug))
    config.GlobalConfig.SliceInterval = slice
    config.GlobalConfig.WriteInterval = write

    log = config.GlobalConfig.Logger

    if _, err := os.Stat(data); err != nil {
        os.MkdirAll(data, 0755)
    }

    hostLookupCache = make(map[string] string)

    address, error := net.ResolveUDPAddr(listen)
    if error != nil {
        log.Fatal("Cannot parse \"%s\": %s", listen, error)
        os.Exit(1)
    }

    config.GlobalConfig.UDPAddress = address

    slices = types.NewSlices(config.GlobalConfig.SliceInterval)
}

func rollupSlices(active_writers []writers.Writer) {
    log.Debug("Rolling up slices")

    closedSlices := slices.ExtractClosedSlices(false)
    closedSlices.Do(func(elem interface {}) {
        slice := elem.(*types.Slice)
        for _, set := range slice.Sets {
            for _, writer := range active_writers {
                writer.Rollup(set.Time, set.Key, set.Values)
            }
        }
    })
}

func main() {
    initialize()

    // Messages channel
    msgchan := make(chan *types.Message)
    go msgSlicer(msgchan)

    active_writers := []writers.Writer {
        &writers.Quartiles { },
        &writers.YesOrNo   { },
    }

    ticker := time.NewTicker(int64(config.GlobalConfig.WriteInterval) * 1000000000) // 10^9
    defer ticker.Stop()
    go func() {
        for {
            <-ticker.C;
            rollupSlices(active_writers)
        }
    }()

    listen(msgchan)
}
