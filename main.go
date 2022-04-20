package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func containsString(list []string, str string) bool {
	for _, value := range list {
		if value == str {
			return true
		}
	}
	return false
}

func main() {
	exec, err := os.Executable()
	if err != nil {
		panic(err)
	}

	appName := filepath.Base(exec)
	appNameWithoutExt := strings.TrimSuffix(appName, filepath.Ext(appName))
	configPath := path.Join(filepath.Dir(exec), fmt.Sprintf("%s.config.json", appNameWithoutExt))
	readConfig := func() []byte {
		json, _ := ioutil.ReadFile(configPath)
		return json
	}
	saveConfig := func(json []byte) {
		err := ioutil.WriteFile(configPath, json, 0644)
		if err != nil {
			log.Fatalf("Couldn't save config file: %+v", err)
		}
	}
	inpost := NewInPostAPIClient(readConfig, saveConfig)
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Printf("Usage: %s [--login] POINT_ID\n", appName)
		return
	}

	if containsString(args, "--login") {
		number := 0
		fmt.Print("Phone number: ")
		fmt.Scanf("%d", &number)
		inpost.SendSMSCode(fmt.Sprintf("%d", number))

		smsCode := 0
		fmt.Print("SMS code: ")
		fmt.Scanf("%d", &smsCode)
		inpost.ConfirmSMSCode(fmt.Sprintf("%d", number), fmt.Sprintf("%d", smsCode))

		fmt.Println("Logged in.")
		return
	}

	pointId := strings.ToUpper(args[0])
	point, err := inpost.GetPoint(pointId)
	if err != nil {
		log.Fatalf("Couldn't get air sensor data for %s: %+v", pointId, err)
	}

	if !point.AirSensor {
		fmt.Printf("%s has no air sensor.\n", pointId)
		return
	}

	fmt.Printf("Point name........: %s\n", point.Name)
	fmt.Printf("Temperature.......: %.1f °C\n", point.AirSensorData.Weather.Temperature)
	fmt.Printf("Pressure..........: %d hPa\n", int(math.Round(float64(point.AirSensorData.Weather.Pressure))))
	fmt.Printf("Humidity..........: %d%%\n", int(math.Round(float64(point.AirSensorData.Weather.Humidity))))
	fmt.Printf("Dust PM 10........: %.1f μg/m³ (%d%%)\n",
		point.AirSensorData.Pollutants.PM10.Value,
		int(math.Round(float64(point.AirSensorData.Pollutants.PM10.Percent))))
	fmt.Printf("Dust PM 2.5.......: %.1f μg/m³ (%d%%)\n",
		point.AirSensorData.Pollutants.PM25.Value,
		int(math.Round(float64(point.AirSensorData.Pollutants.PM25.Percent))))
	fmt.Printf("Air quality.......: %s\n", strings.ToLower(strings.ReplaceAll(point.AirSensorData.AirQuality, "_", " ")))
	fmt.Printf("Last updated......: %s\n", point.AirSensorData.UpdatedUntil.Local().Format(time.Stamp))
}
