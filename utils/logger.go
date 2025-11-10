package utils

import (
	"fmt"
	"time"
)

func Info(msg string) {
	fmt.Printf("[%s] INFO: %s\n", time.Now().Format("15:04:05"), msg)
}
