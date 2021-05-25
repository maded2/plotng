
export GOOS=linux; go build plotng/cmd/plotng
tar -czvf plotng_linux_amd64.tar.gz plotng README.md config.json

export GOOS=darwin; go build plotng/cmd/plotng
tar -czvf plotng_macos_amd64.tar.gz plotng README.md config.json

export GOOS=windows; go build plotng/cmd/plotng
zip plotng_windows_amd64.zip plotng.exe README.md config.json
