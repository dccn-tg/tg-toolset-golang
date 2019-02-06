package main

import (
    "fmt"
//    "bytes"
//    "net/http"
//    "os"
    "strings"
    "gopkg.in/mcuadros/go-syslog.v2"
)

func main() {

    channel := make(syslog.LogPartsChannel)
    handler := syslog.NewChannelHandler(channel)

    server := syslog.NewServer()
    server.SetFormat(syslog.RFC3164)
    server.SetHandler(handler)
    server.ListenUDP("0.0.0.0:5514")
    server.ListenTCP("0.0.0.0:5514")

    server.Boot()

    go func(channel syslog.LogPartsChannel) {
        for logParts := range channel {
		fmt.Printf("%s\n", logParts["content"])
        }
    } (channel)

    server.Wait()
}

// Converts nginx $msec (Float) to milliseconds (Int).
func fixTimestamp(content string) string {
    timestampIndex := strings.LastIndex(content, " ")
    timestampParts := strings.Split(content[timestampIndex + 1:], ".")
    timestamp := timestampParts[0]
    if len(timestampParts) > 1 {
        timestamp += (timestampParts[1] + "000")[0:3]
    } else {
        timestamp += "000"
    }

    return content[:timestampIndex] + " " + timestamp
}
