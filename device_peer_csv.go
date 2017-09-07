package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/tonyHuinker/ehop"
)

type peerDetails struct {
	Metrics [4]int64
}

func newPeerDetails() peerDetails {
	var p peerDetails
	p.Metrics = [4]int64{0, 0, 0, 0}
	return p
}

func askForInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(prompt)
	response, _ := reader.ReadString('\n')
	fmt.Println("")
	return strings.TrimSpace(response)
}

type metricRsp struct {
	Stats  []stat `json:"stats"`
	Cycle  string `json:"cycle"`
	NodeID int    `json:"node_id"`
	From   int64  `json:"from"`
	Until  int64  `json:"until"`
	Clock  int64  `json:"clock"`
}

type stat struct {
	OID      int           `json:"oid"`
	Time     int64         `json:"time"`
	Duration int64         `json:"duration"`
	Values   [][]tSetValue `json:"values"` //Outer array represents multi-metric requests
}

type tSetValue struct {
	Key   protocolKey   `json:"key"`
	Vtype string        `json:"vtype"`
	Value []deviceValue `json:"value"`
}

type deviceValue struct {
	Key   deviceKey `json:"key"`
	Vtype string    `json:"vtype"`
	Value int64     `json:"value"`
}

type protocolKey struct {
	KeyType  string `json:"key_type"`
	Protocol string `json:"str"`
}

type deviceKey struct {
	KeyType   string `json:"key_type"`
	Addr      string `json:"addr"`
	Host      string `json:"host"`
	DeviceOID int    `json:"device_oid"`
}

func main() {
	//Get number of days (to * by ms) to add to
	days := askForInput("How many days of lookback?")
	daysINT, _ := strconv.Atoi(days)
	lookback := daysINT * -86400000

	//Specify Key File
	keyFile := askForInput("What is the name of your keyFile?")
	myhop := ehop.NewEDAfromKey(keyFile)

	deviceID := askForInput("What is the device id?")
	body := `{"cycle": "auto","from": ` + strconv.Itoa(lookback) + `, "metric_category": "app_detail", "metric_specs": [{"name": "pkts_in"},{"name": "pkts_out"},{"name": "bytes_in"},{"name": "bytes_out"}],"object_ids": [` + deviceID + `],"object_type": "device","until": 0}`

	//Get all peers from the EDA
	resp, error := ehop.CreateEhopRequest("POST", "metrics/total", body, myhop)
	defer resp.Body.Close()

	if error != nil {
		fmt.Println("Error requesting peer metrics: " + error.Error())
		os.Exit(-1)
	} else if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-200 status code requesting peer metrics: " + resp.Status)
		os.Exit(-1)
	}

	//Store into Structs
	var metricRsp metricRsp

	error = json.NewDecoder(resp.Body).Decode(&metricRsp)
	if error != nil {
		fmt.Println("Error parsing response JSON: " + error.Error())
		os.Exit(-1)
	}
	fmt.Println("Metrics successfully queried")

	peerList := map[string]peerDetails{}

	for _, stat := range metricRsp.Stats {
		for _, values := range stat.Values {
			for _, protocol := range values {
				for _, peer := range protocol.Value {
					peerList[peer.Key.Addr+","+protocol.Key.Protocol] = newPeerDetails()
				}
			}
		}
	}
	for _, stat := range metricRsp.Stats {
		for i, values := range stat.Values {
			for _, protocol := range values {
				for _, peer := range protocol.Value {
					p := peerList[peer.Key.Addr+","+protocol.Key.Protocol]
					p.Metrics[i] = peer.Value
					peerList[peer.Key.Addr+","+protocol.Key.Protocol] = p
				}
			}
		}
	}

	f, _ := os.Create("device_" + deviceID + "_peer_details.csv")

	io.WriteString(f, "PeerIP,Protocol,Packets In,Packets Out,Bytes In,Bytes Out\n")
	for ip, peerDetails := range peerList {
		m := peerDetails.Metrics
		io.WriteString(f, ip+","+strconv.FormatInt(m[0], 10)+","+strconv.FormatInt(m[1], 10)+","+strconv.FormatInt(m[2], 10)+","+strconv.FormatInt(m[3], 10)+"\n")
	}
	f.Close()

	fmt.Println("File device_" + deviceID + "_peer_details.csv successfully written")

}
