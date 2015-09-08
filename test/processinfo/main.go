package main
import (
	"os"
	"log"
)

func main() {
	proc, err := os.FindProcess(0)
	log.Println("Proc-usage", proc.UserTime())
}