module github.com/note-cli

go 1.23.0

toolchain go1.23.3

replace github.com/blues/note-cli/lib => ./lib

// uncomment this for easier testing locally
// replace github.com/blues/note-go => ../hub/note-go

require (
	github.com/blues/note-cli/lib v0.0.0-20240515194341-6ba45582741d
	github.com/blues/note-go v1.7.4
	github.com/fatih/color v1.17.0
	github.com/peterh/liner v1.2.2
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/spf13/viper v1.21.0
)

require (
	github.com/blues/notehub-go v0.0.0-20260105133531-1e40c1ed371c // indirect
	github.com/creack/goselect v0.1.2 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/text v0.28.0 // indirect
	gopkg.in/validator.v2 v2.0.1 // indirect
	periph.io/x/conn/v3 v3.7.0 // indirect
)

require (
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/golang/snappy v0.0.4
	github.com/lufia/plan9stats v0.0.0-20240513124658-fba389f38bae // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/shirou/gopsutil/v3 v3.24.4 // indirect
	github.com/shoenig/go-m1cpu v0.1.7 // indirect
	github.com/spf13/cobra v1.10.1
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	go.bug.st/serial v1.6.2
	golang.org/x/sys v0.34.0 // indirect
	periph.io/x/host/v3 v3.8.2 // indirect
)
