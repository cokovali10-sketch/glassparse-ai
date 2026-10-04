@echo off
echo Building glassparse-ai engine...
cd engine
go build -o ..\glassparse.exe -ldflags="-s -w" ./cmd/glassparse
cd ..
echo Done: glassparse.exe
