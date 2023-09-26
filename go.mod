module github.com/note-cli

go 1.15

replace github.com/blues/note-cli/lib => ./lib

require (
	github.com/blues/note-cli/lib v0.0.0-20230907231149-742f8c61893d
	github.com/blues/note-go v1.7.0
	github.com/fatih/color v1.15.0
	github.com/peterh/liner v1.2.2
	golang.org/x/term v0.12.0
)

require (
	github.com/creack/goselect v0.1.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/golang/snappy v0.0.4
	github.com/lufia/plan9stats v0.0.0-20230326075908-cb1d2100619a // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/shirou/gopsutil/v3 v3.23.8 // indirect
	golang.org/x/sys v0.12.0 // indirect
	periph.io/x/host/v3 v3.8.2 // indirect
	periph.io/x/periph v3.6.8+incompatible // indirect
)
