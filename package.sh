export GOOS=linux
go build plotng/cmd/plotng-client
go build plotng/cmd/plotng-server
tar -czvf plotng_linux_amd64.tar.gz plotng-client plotng-server README.md config.json

export GOOS=darwin
go build plotng/cmd/plotng-client
go build plotng/cmd/plotng-server
tar -czvf plotng_macos_amd64.tar.gz plotng-client plotng-server README.md config.json

export GOOS=windows
go build plotng/cmd/plotng-client
go build plotng/cmd/plotng-server
zip plotng_windows_amd64.zip plotng-client.exe plotng-server.exe README.md config.json
