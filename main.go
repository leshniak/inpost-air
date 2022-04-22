package main

import (
	"flag"
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

func printUsage(appName string) {
	output := flag.CommandLine.Output()
	fmt.Fprintf(output, "Usage: %s [...OPTIONS] POINT_ID\n\n", appName)
	fmt.Fprintln(output, "Options:")
	flag.PrintDefaults()
}

func main() {
	exec, err := os.Executable()
	if err != nil {
		panic(err)
	}

	appName := filepath.Base(exec)
	appNameWithoutExt := strings.TrimSuffix(appName, filepath.Ext(appName))
	configPath := path.Join(filepath.Dir(exec), fmt.Sprintf("%s.config.json", appNameWithoutExt))

	flag.Usage = func() {
		output := flag.CommandLine.Output()
		fmt.Fprintf(output, "Usage: %s [...OPTIONS] POINT_ID\n\n", appName)
		fmt.Fprintln(output, "Options:")
		flag.PrintDefaults()
	}
	shouldLogin := flag.Bool("login", false, "login to InPost Mobile")
	flag.Parse()
	pointId := strings.ToUpper(flag.Arg(0))

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

	if pointId == "" {
		flag.Usage()
		os.Exit(0)
	}

	if *shouldLogin {
		var number int
		fmt.Print("Phone number: ")
		fmt.Scanf("%d", &number)
		inpost.SendSMSCode(fmt.Sprintf("%d", number))

		var smsCode int
		fmt.Print("SMS code: ")
		fmt.Scanf("%d", &smsCode)
		inpost.ConfirmSMSCode(fmt.Sprintf("%d", number), fmt.Sprintf("%d", smsCode))

		fmt.Println("Logged in.")
	}

	point, err := inpost.GetPoint(pointId)
	if err != nil {
		log.Fatalf("Couldn't get air sensor data for %s: %+v", pointId, err)
	}

	if !point.AirSensor {
		fmt.Printf("%s has no air sensor.\n", pointId)
		os.Exit(0)
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
