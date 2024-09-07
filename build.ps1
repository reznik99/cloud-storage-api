$VERSION = (git describe --tags)
$Env:GOOS = "linux"; 
$Env:GOARCH = "amd64"; 
$Env:CGO_ENABLED = 0; 
go build -v -ldflags="-X 'main.Version=${VERSION}'" -o dist/storage-api .