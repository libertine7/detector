package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
)

var lock sync.RWMutex

// SimpleSensorStruct Main sensor sturcture
type SimpleSensorStruct struct {
	SensorID string `json:"sid"`
	State    string `json:"state"`
}

// SensorStruct Main sensor sturcture
type SensorStruct struct {
	XMLName       xml.Name  `xml:"Sensor"`
	SensorID      string    `json:"sid" xml:"SensorID,attr"`
	Channel       int       `json:"channel" xml:"-"`
	AlarmGap      float64   `json:"alarmgap" xml:"-"`
	SensorType    string    `json:"sensortype" xml:"SensorType"`
	StateTs       time.Time `json:"statets" xml:"StateTs"`
	State         string    `json:"state" xml:"State"`
	IntrusionType string    `json:"intrusiontype" xml:"-"`
	Position      string    `json:"position" xml:"-"`
	Comment       string    `json:"comment" xml:"Comment"`
	LatLon        string    `json:"latlon" xml:"-"`
	GeoJSON       string    `json:"geojson" xml:"-"`
	Data          string    `json:"data" xml:"-"`
	Tags          []string  `json:"tags" xml:"-"`
	ShowOnMap     bool      `json:"showonmap" xml:"-"`
}

// DdStateStruct main DD state structure
type DdStateStruct struct {
	Started time.Time               `json:"Started"`
	Sensors map[string]SensorStruct `json:"Sensors"`
	Params  map[string]string       `json:"Params"`
}

var DdState DdStateStruct = DdStateStruct{
	Started: time.Now(),
	Sensors: make(map[string]SensorStruct),
	Params:  make(map[string]string),
}

func (c *SensorsService) UpdateSensorsState(r *http.Request, req *[]SimpleSensorStruct, res *bool) error {
	for _, v := range *req {
		if _, ok := DdState.Sensors[v.SensorID]; ok {
			lock.Lock()
			ss := DdState.Sensors[v.SensorID]
			if ss.State != v.State {
				ss.StateTs = time.Now()
				ss.State = v.State
				DdState.Sensors[v.SensorID] = ss
			}
			lock.Unlock()
		} else {
			NewSensor := SensorStruct{}
			NewSensor.SensorID = v.SensorID
			NewSensor.State = v.State
			NewSensor.StateTs = time.Now()

			lock.Lock()
			DdState.Sensors[v.SensorID] = NewSensor
			lock.Unlock()
		}
	}

	*res = true
	return nil
}

// SaveData DD state to disk
func (d *DdStateStruct) SaveData() error {
	lock.Lock()
	ddStateJson, err := json.Marshal(DdState)
	if err != nil {
		fmt.Println("error:", err)
	}
	lock.Unlock()

	ioutil.WriteFile("ddata.json", ddStateJson, 0644)

	return nil
}

func (d *DdStateStruct) LoadData() error {

	if DdState.Params != nil {
		DdState.Params["xmlserver"] = "server url"
		DdState.Params["server"] = "server url"
		DdState.Params["project"] = "project name"
		DdState.Params["serial"] = ""
	}

	dd := DdStateStruct{}

	ddStateJson, err := ioutil.ReadFile("ddata.json")
	if err != nil {
		fmt.Println("error:", err)
		return err
	}

	err = json.Unmarshal(ddStateJson, &dd)
	if err != nil {
		fmt.Println("error:", err)
		return err
	}

	if dd.Params == nil {
		dd.Params = make(map[string]string)
		dd.Params["xmlserver"] = "server url"
		dd.Params["server"] = "server url"
		dd.Params["project"] = "project name"
		dd.Params["serial"] = ""
	}

	if dd.Sensors == nil {
		dd.Sensors = make(map[string]SensorStruct)
	}

	lock.Lock()
	DdState = dd
	lock.Unlock()
	fmt.Println("Data loaded")

	return nil
}

func HelloServer(w http.ResponseWriter, req *http.Request) {
	lock.Lock()
	ddStateJSON, err := json.Marshal(DdState)
	if err != nil {
		fmt.Println("error:", err)
	}
	lock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(ddStateJSON)
}

type SensorReqStruct struct {
	Server        string    `json:"server"`
	SensorID      string    `json:"sid"`
	Channel       int       `json:"channel"`
	AlarmGap      float64   `json:"alarmgap"`
	SensorType    string    `json:"sensortype"`
	StateTs       time.Time `json:"statets"`
	State         string    `json:"state"`
	IntrusionType string    `json:"intrusiontype"`
	Position      string    `json:"position"`
	Comment       string    `json:"comment"`
	LatLon        string    `json:"latlon"`
	GeoJSON       string    `json:"geojson"`
	Tags          []string  `json:"tags"`
	ShowOnMap     bool      `json:"showonmap"`
	ParamName     string    `json:"ParamName"`
	ParamValue    string    `json:"ParamValue"`
	Data          string    `json:"data"`
}

type SensorsService struct {
}

func (c *SensorsService) UpdateParam(r *http.Request, req *SensorReqStruct, res *bool) error {
	lock.Lock()
	if DdState.Params == nil {
		DdState.Params = make(map[string]string)
	}
	DdState.Params[req.ParamName] = req.ParamValue
	lock.Unlock()

	*res = true
	return nil
}

func (c *SensorsService) DeleteParam(r *http.Request, req *SensorReqStruct, res *bool) error {
	lock.Lock()
	delete(DdState.Params, req.ParamName)
	lock.Unlock()

	*res = true
	return nil
}

func (c *SensorsService) CreateSensor(r *http.Request, req *SensorReqStruct, res *bool) error {
	ss := SensorStruct{
		SensorID:      req.SensorID,
		SensorType:    req.SensorType,
		StateTs:       time.Now(),
		State:         req.State,
		Comment:       req.Comment,
		LatLon:        req.LatLon,
		Position:      req.Position,
		IntrusionType: req.IntrusionType,
	}

	lock.Lock()
	DdState.Sensors[ss.SensorID] = ss
	lock.Unlock()

	*res = true
	return nil
}

func (c *SensorsService) DeleteSensor(r *http.Request, req *SensorReqStruct, res *bool) error {
	lock.Lock()
	delete(DdState.Sensors, req.SensorID)
	lock.Unlock()

	*res = true
	return nil
}

func (c *SensorsService) SetSensorState(r *http.Request, req *SensorReqStruct, res *bool) error {
	if _, ok := DdState.Sensors[req.SensorID]; ok {
		lock.Lock()
		ss := DdState.Sensors[req.SensorID]
		ss.State = req.State
		DdState.Sensors[req.SensorID] = ss // check it
		lock.Unlock()
	}

	*res = true
	return nil
}

func (c *SensorsService) UpdateSensorData(r *http.Request, req *SensorReqStruct, res *bool) error {
	if _, ok := DdState.Sensors[req.SensorID]; ok {
		lock.RLock()
		ss := DdState.Sensors[req.SensorID]
		lock.RUnlock()

		ss.StateTs = time.Now()
		ss.State = req.State
		ss.Comment = req.Comment
		ss.LatLon = req.LatLon
		ss.Position = req.Position
		ss.IntrusionType = req.IntrusionType
		ss.Tags = req.Tags
		ss.Channel = req.Channel
		ss.AlarmGap = req.AlarmGap
		ss.ShowOnMap = req.ShowOnMap
		ss.SensorType = req.SensorType
		ss.Data = req.Data

		lock.Lock()
		DdState.Sensors[req.SensorID] = ss // check it
		lock.Unlock()
	}

	*res = true
	return nil
}

func (c *SensorsService) UpdateSensorGeoJSON(r *http.Request, req *SensorReqStruct, res *bool) error {
	if _, ok := DdState.Sensors[req.SensorID]; ok {
		lock.RLock()
		ss := DdState.Sensors[req.SensorID]
		lock.RUnlock()

		ss.StateTs = time.Now()
		ss.GeoJSON = req.GeoJSON

		lock.Lock()
		DdState.Sensors[req.SensorID] = ss // check it
		lock.Unlock()
	}

	*res = true
	return nil
}

func GetChannels() []int {
	var channels []int
	lock.RLock()
	for _, v := range DdState.Sensors {
		if v.Channel != 0 {
			channels = append(channels, v.Channel)
		}
	}
	lock.RUnlock()

	return channels
}

func SetChannelState(chn int, st string) bool {
	lock.RLock()
	for _, v := range DdState.Sensors {
		// fmt.Println("tt", time.Now().Sub(v.StateTs) > time.Second*30, v.StateTs)
		if v.Channel == chn && v.State != st && (st != "OK" || time.Now().Sub(v.StateTs) > time.Second*30) {

			go func(senid, stt string) {
				lock.Lock()

				ss := DdState.Sensors[senid]
				ss.StateTs = time.Now()
				ss.State = stt

				DdState.Sensors[senid] = ss
				lock.Unlock()
				fmt.Println("set state", chn, st)
			}(v.SensorID, st)
		}
	}
	lock.RUnlock()

	return true
}

func (c *SensorsService) GetSensors(r *http.Request, req *SensorReqStruct, res *DdStateStruct) error {
	dd := DdStateStruct{}
	lock.Lock()
	ddStateJson, err := json.Marshal(DdState)
	if err != nil {
		fmt.Println("error:", err)
	}
	lock.Unlock()

	err = json.Unmarshal(ddStateJson, &dd)
	if err != nil {
		fmt.Println("error:", err)
	}

	*res = dd
	return nil
}

func (c *SensorsService) GetSensorsArr(r *http.Request, req *SensorReqStruct, res *[]SensorStruct) error {
	dd := DdStateStruct{}
	var Sensors []SensorStruct
	lock.Lock()
	ddStateJson, err := json.Marshal(DdState)
	if err != nil {
		fmt.Println("error:", err)
	}
	lock.Unlock()

	err = json.Unmarshal(ddStateJson, &dd)
	if err != nil {
		fmt.Println("error:", err)
	}

	for _, element := range dd.Sensors {
		Sensors = append(Sensors, element)
	}

	*res = Sensors
	return nil
}

func (c *SensorsService) SaveData(r *http.Request, req *SensorReqStruct, res *bool) error {
	DdState.SaveData()

	*res = true
	return nil
}

type RPCClientRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      string      `json:"id"`
}

func SendDataToServer() {
	dd := DdStateStruct{}

	for {
		lock.Lock()
		ddStateJSON, err := json.Marshal(DdState)
		lock.Unlock()
		if err != nil {
			fmt.Println("error:", err)
		}

		err = json.Unmarshal(ddStateJSON, &dd)
		if err != nil {
			fmt.Println("error:", err)
		}

		SensorsJSON, err := json.Marshal(RPCClientRequest{JSONRPC: "2.0", Method: "xx.xx", Params: dd, ID: "1"})
		if err == nil {
			if _, ok := dd.Params["server"]; ok {
				_, err = url.Parse(dd.Params["server"])
				if err == nil {
					fmt.Println("send data to ", dd.Params["server"])
					c := &http.Client{
						Timeout: 1 * time.Second,
					}
					_, err = c.Post(dd.Params["server"], "application/json", bytes.NewReader(SensorsJSON))
					if err != nil {
						fmt.Println("error:", err)
					}
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
}

type DdStateXmlStruct struct {
	XMLName xml.Name       `json:"-" xml:"Sensors"`
	Started time.Time      `json:"Started" xml:"Started"`
	Sensors []SensorStruct `json:"Sensors" xml:"Sensors"`
}

func SendSSTMKToServer() {
	dd := DdStateStruct{}

	for {
		lock.Lock()
		ddStateJSON, err := json.Marshal(DdState)
		lock.Unlock()
		if err != nil {
			fmt.Println("error:", err)
		}

		err = json.Unmarshal(ddStateJSON, &dd)
		if err != nil {
			fmt.Println("error:", err)
		}

		var Dxml DdStateXmlStruct = DdStateXmlStruct{
			Started: time.Now(),
			Sensors: make([]SensorStruct, 0),
		}

		for _, v := range dd.Sensors {
			Dxml.Sensors = append(Dxml.Sensors, v)
		}

		output, err := xml.MarshalIndent(Dxml, "  ", "    ")
		if err != nil {
			fmt.Printf("error: %v\n", err)
		}

		// output = append([]byte(xml.Header), output[:]...)
		// os.Stdout.Write(output)

		if err == nil {
			if _, ok := dd.Params["xmlserver"]; ok {
				_, err = url.Parse(dd.Params["xmlserver"])
				if err == nil {
					fmt.Println("send xml to ", dd.Params["xmlserver"])
					c := &http.Client{
						Timeout: 1 * time.Second,
					}
					_, err = c.Post(dd.Params["xmlserver"], "application/xml", bytes.NewReader(output))
					if err != nil {
						fmt.Println("error:", err)
					}
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func AutoSaveData() {
	for {
		lock.Lock()
		ddStateJSON, err := json.Marshal(DdState)
		lock.Unlock()

		if err == nil {
			ioutil.WriteFile("ddata.json", ddStateJSON, 0644)
		}

		time.Sleep(time.Second * 30)
	}
}

func main() {
	DdState.LoadData()

	fmt.Println(GetChannels())

	datachanel := make(chan []byte, 100)
	metricchannel := make(chan [8000]float64, 100)
	go worker(datachanel, metricchannel)
	// go serveDriver(datachanel)
	go echo(metricchannel)

	go SendDataToServer()
	go SendSSTMKToServer()
	go AutoSaveData()

	fs := http.FileServer(http.Dir("static"))

	s := rpc.NewServer()
	s.RegisterCodec(json2.NewCustomCodec(&rpc.CompressionSelector{}), "application/json")
	s.RegisterService(new(SensorsService), "")

	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/stat", HelloServer)
	http.Handle("/rpc", s)
	http.ListenAndServe(":8118", nil)
}
