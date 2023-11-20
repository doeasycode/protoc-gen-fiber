# build
build:
	go build -ldflags="-s -w" . && cp protoc-gen-fiber /Applications/www/go/bin && rm protoc-gen-fiber

