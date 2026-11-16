package main

import "runtime"

func desktopDefault() bool { return runtime.GOOS == "windows" || runtime.GOOS == "darwin" }
