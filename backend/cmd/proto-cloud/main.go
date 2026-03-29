package main

import (
	"fmt"
	"os"

"github.com/Vinicius163-dot/proto-cloud/backend/internal/cli"
)

func main() {
	fmt.Println("proto-cloud — CLI (esqueleto)")

	if len(os.Args) < 2 {
		fmt.Println("Uso: proto-cloud <comando> [args]")
		fmt.Println("Comandos: regions, instances, version")
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "version":
		fmt.Println("proto-cloud v0.1.0")

	case "regions":
		// Subcomando: regions list
		if len(os.Args) >= 3 && os.Args[2] == "list" {
			regions := cli.RegionsList()
			for _, r := range regions {
				fmt.Println(r)
			}
			return
		}
		fmt.Println("Uso: proto-cloud regions list")

	case "instances":
		fmt.Println("TODO: instances list --region <region>")
	default:
		fmt.Println("Comando não reconhecido:", cmd)
	}
}
