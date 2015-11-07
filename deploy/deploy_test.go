package main_test
import (
	"testing"
	"os/exec"
	"os"
	"regexp"
)

/*
 * This runs deploy for all the simulation/test*.toml configs and outputs
 * the logs. It can only detect compilation failure, not runtime
 * failure.
 */

func TestCompileDeploy(t *testing.T) {
	// First compile deploy
	t.Log("Compiling deploy")
	err := exec.Command("go", "build").Run()
	checkFatal(t, "Couldn't compile deploy", err)

	// Read all simulation-names from the simulation/ - directory
	simulation_dir, err := os.Open("simulation")
	checkFatal(t, "Couldn't open simulation-directory", err)
	simulations, err := simulation_dir.Readdirnames(0)
	checkFatal(t, "Couldn't list directory-names", err)

	t.Log("Starting to run tests")

	// Check if it's a "test_.*\.toml"-file and run it with deploy
	for _, sim := range (simulations) {
		testfile, err := regexp.MatchString("test_na.*\\.toml", sim)
		checkFatal(t, "Error in matching", err)
		if testfile {
			sim_out, err := exec.Command("./deploy", "simulation/" + sim).Output()
			checkFatal(t, "Simulating " + sim + "failed\n" + string(sim_out), err)
			t.Log("Result of simulation", sim, "\n" + string(sim_out))
		}
	}
}

func checkFatal(t *testing.T, fail string, err error) {
	if err != nil {
		t.Fatal(fail + ":", err)
	}
}