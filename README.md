# netatmo-api-go
Simple API to access Netatmo weather station data written in Go. Will auto update token if needed.

## Quickstart

- [Create a new netatmo app](https://dev.netatmo.com/apps/createanapp#form)
- Generate a new token using the token generator. Scope needed is `read_station`: ![token_generator_netatmo.png](token_generator_netatmo.png)
- Edit ```test/netatmo.toml```with your credentials 
- run ```go run test/netatmo-api-test.go -f test/netatmo.toml```
- Output shall look like :
```
Station: Mein Zuhause (Wetterstation Innensensor)
        City: Bern
        Country: CH
        Timezone: Europe/Zurich
        Longitude: 0x14000098c38
        Latitude: 0x14000098c3c
        Altitude: 0x14000098c0c

        Module: Wetterstation Aussensensor
                BatteryPercent: 63 (age 399s)
                RFStatus: 71 (age 399s)
                MinTemp: 9.4 (age 399s)
                MaxTemp: 13.5 (age 399s)
                TempTrend: down (age 399s)
                Humidity: 68 (age 399s)
                Temperature: 13.1 (age 399s)
        Module: Schlafzimmer Sensor
                BatteryPercent: 61 (age 437s)
                RFStatus: 63 (age 437s)
                Humidity: 50 (age 437s)
                CO2: 1600 (age 437s)
                Temperature: 24.2 (age 437s)
                MinTemp: 23.2 (age 437s)
                MaxTemp: 24.2 (age 437s)
                TempTrend: stable (age 437s)
        Module: Wetterstation Innensensor
                WifiStatus: 60 (age 395s)
                Noise: 35 (age 395s)
                PressureTrend: down (age 395s)
                MinTemp: 24.1 (age 395s)
                TempTrend: stable (age 395s)
                Pressure: 1011.5 (age 395s)
                AbsolutePressure: 963.2 (age 395s)
                Temperature: 24.4 (age 395s)
                MaxTemp: 24.5 (age 395s)
                Humidity: 50 (age 395s)
                CO2: 1376 (age 395s)
```

## Tips
- Only Read() method actually do an API call and refresh all data at once
- Main station is handle as a module, it means that Modules() method returns list of additional modules and station itself.
- Data() returns sensors values (such as temperature) whereas Info() returns module status (such as battery level)
