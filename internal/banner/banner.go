package banner

import "fmt"

const Version = "1.0.0"

func Print() {
	banner := `
    ___                             ______
   /   |  _________ ___  _______   / ____/___
  / /| | / ___/ __  / / / / ___/  / / __ / __ \
 / ___ |/ /  / /_/ / /_/ (__  )  / /_/ / /_/ /
/_/  |_/_/   \__, /\__,_/____/   \____/\____/
            /____/  v%s - Metrics Sentinel
    `
	fmt.Printf(banner, Version)
	fmt.Println("\n------------------------------------------------")
}
