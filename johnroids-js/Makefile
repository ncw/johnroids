build:
	gopherjs build -m
	GOARCH=wasm GOOS=js /opt/go/go-tip/bin/go build -o johnroids.wasm

serve:	build
	go run serve.go

upload:	build
	rclone -v copy --include="*.{wasm,js,css,html}" . box:public_html/johnroids
	echo See https://www.craig-wood.com/nick/johnroids/
