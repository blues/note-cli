module github.com/note-cli

go 1.15

replace github.com/blues/note-cli/lib => ./lib
replace github.com/blues/note-go => ../hub/note-go

// uncomment this for easier testing locally
// replace github.com/blues/note-go => ../hub/note-go

require (
	github.com/blues/note-cli/lib v0.0.0-20240515194341-6ba45582741d
	github.com/blues/note-go v1.7.4
	github.com/fatih/color v1.17.0
	github.com/peterh/liner v1.2.2
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
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
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	go.bug.st/serial v1.6.2
	golang.org/x/sys v0.20.0 // indirect
	periph.io/x/host/v3 v3.8.2 // indirect
)
